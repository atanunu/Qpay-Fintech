package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

// WorkPayment handles one leased operation. It never resubmits a previously marked submission.
func (s *Service) WorkPayment(ctx context.Context) (bool, error) {
	if !s.Config.ExternalEnabled || s.Config.Gateway == nil {
		return false, nil
	}
	job, payment, lease := "", "", security.Random("", 24)
	attempt := 0
	e := s.transact(ctx, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `UPDATE jobs SET status='leased',attempts=attempts+1,lease_until=$1,lease_token=$2 WHERE id=(SELECT id FROM jobs WHERE (status='queued' AND available_at<=$3) OR (status='leased' AND lease_until<$3) ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,object_id,attempts`, s.Now().Add(90*time.Second), lease, s.Now()).Scan(&job, &payment, &attempt)
	})
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	var request ProviderRequest
	submit := false
	skip := false
	e = s.transact(ctx, func(tx *sql.Tx) error {
		submit, skip = false, false
		var owner string
		if e := tx.QueryRowContext(ctx, `SELECT owner_id FROM payments WHERE id=$1`, payment).Scan(&owner); e != nil {
			return e
		}
		u, e := s.user(ctx, tx, owner, true)
		if e != nil {
			return e
		}
		p, e := scanPayment(tx.QueryRowContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE id=$1 FOR UPDATE`, payment))
		if e != nil {
			return e
		}
		var token string
		var until time.Time
		if e = tx.QueryRowContext(ctx, `SELECT lease_token,lease_until FROM jobs WHERE id=$1 FOR UPDATE`, job).Scan(&token, &until); e != nil {
			return e
		}
		if token != lease || !until.After(s.Now()) {
			return conflict("worker lease no longer owned")
		}
		if p.Status == "failed" || (p.Status == "succeeded" && p.FulfilmentStatus != "pending") {
			skip = true
			return exec(tx, ctx, `UPDATE jobs SET status='done',lease_until=NULL WHERE id=$1`, job)
		}
		if p.Status == "pending_review" {
			skip = true
			return exec(tx, ctx, `UPDATE jobs SET status='dead',lease_until=NULL,last_error='review_required' WHERE id=$1`, job)
		}
		var encrypted, narration string
		if e = tx.QueryRowContext(ctx, `SELECT destination_enc,narration FROM payments WHERE id=$1`, payment).Scan(&encrypted, &narration); e != nil {
			return e
		}
		raw, e := s.Config.Box.Open(encrypted, "payment:"+payment)
		if e != nil {
			return e
		}
		var d Destination
		if e = json.Unmarshal([]byte(raw), &d); e != nil {
			return e
		}
		request = ProviderRequest{Reference: payment, UpstreamID: p.ProviderReference, Kind: p.Kind, Amount: p.Amount, Currency: p.Currency, Destination: d, Narration: narration}
		if p.Status == "accepted" {
			controls, ce := s.controls(ctx, tx, u.ID)
			if ce != nil {
				return ce
			}
			if e = s.eligible(u); e != nil || controls.Frozen {
				skip = true
				if e = exec(tx, ctx, `UPDATE payments SET status='pending_review',updated_at=$2 WHERE id=$1`, payment, s.Now()); e != nil {
					return e
				}
				return exec(tx, ctx, `UPDATE jobs SET status='dead',lease_until=NULL,last_error='account_restricted_before_submission' WHERE id=$1`, job)
			}
			submit = true
			return exec(tx, ctx, `UPDATE payments SET status='submitted',updated_at=$2 WHERE id=$1`, payment, s.Now())
		}
		return nil
	})
	if e != nil {
		return true, e
	}
	if skip {
		return true, nil
	}
	network, cancel := context.WithTimeout(ctx, 65*time.Second)
	defer cancel()
	var result ProviderResult
	if submit {
		result, e = s.Config.Gateway.Submit(network, request)
	} else {
		result, e = s.Config.Gateway.Query(network, request)
	}
	if e != nil {
		return true, s.retryPayment(ctx, job, payment, lease, attempt, "upstream_unresolved")
	}
	return true, s.applyResult(ctx, job, payment, lease, attempt, result)
}
func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return time.Duration(1<<attempt) * time.Second
}
func (s *Service) retryPayment(ctx context.Context, job, payment, lease string, attempt int, reason string) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		// Follow the same payment-before-job locking order as result application.
		var current string
		if e := tx.QueryRowContext(ctx, `SELECT id FROM payments WHERE id=$1 FOR UPDATE`, payment).Scan(&current); e != nil {
			return e
		}
		if e := tx.QueryRowContext(ctx, `SELECT lease_token FROM jobs WHERE id=$1 FOR UPDATE`, job).Scan(&current); e != nil {
			return e
		}
		if current != lease {
			return conflict("worker lease superseded")
		}
		if e := exec(tx, ctx, `UPDATE payments SET status='pending',updated_at=$2 WHERE id=$1 AND status IN('accepted','submitted','pending')`, payment, s.Now()); e != nil {
			return e
		}
		state := "queued"
		if attempt >= 30 {
			state = "dead"
		}
		return exec(tx, ctx, `UPDATE jobs SET status=$3,available_at=$4,lease_until=NULL,last_error=$5 WHERE id=$1 AND lease_token=$2`, job, lease, state, s.Now().Add(retryDelay(attempt)), reason)
	})
}
func (s *Service) applyResult(ctx context.Context, job, payment, lease string, attempt int, r ProviderResult) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		var owner string
		if e := tx.QueryRowContext(ctx, `SELECT owner_id FROM payments WHERE id=$1`, payment).Scan(&owner); e != nil {
			return e
		}
		if _, e := s.user(ctx, tx, owner, true); e != nil {
			return e
		}
		p, e := scanPayment(tx.QueryRowContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE id=$1 FOR UPDATE`, payment))
		if e != nil {
			return e
		}
		var current string
		if e = tx.QueryRowContext(ctx, `SELECT lease_token FROM jobs WHERE id=$1 FOR UPDATE`, job).Scan(&current); e != nil {
			return e
		}
		if current != lease {
			return conflict("worker lease superseded")
		}
		obs := security.Digest(jsonText([]any{payment, r.Reference, r.Status, r.Amount, r.Currency, r.UpstreamID, security.Digest(r.Fulfilment)}))
		if e = exec(tx, ctx, `INSERT INTO observations(id,payment_id,status,reference,amount,currency) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(id) DO NOTHING`, obs, payment, r.Status, r.Reference, int64(r.Amount), r.Currency); e != nil {
			return e
		}
		mismatch := r.Reference != payment || r.Amount != p.Amount || r.Currency != p.Currency || len(r.Fulfilment) > 10000 || len(r.UpstreamID) > 150 || (p.ProviderReference != "" && r.UpstreamID != "" && p.ProviderReference != r.UpstreamID)
		terminalConflict := (p.Status == "succeeded" && r.Status == "failed") || (p.Status == "failed" && r.Status == "succeeded")
		if mismatch || terminalConflict || r.Status == "review" {
			if e = exec(tx, ctx, `UPDATE payments SET status='pending_review',updated_at=$2 WHERE id=$1 AND status NOT IN('succeeded','failed')`, payment, s.Now()); e != nil {
				return e
			}
			if e = exec(tx, ctx, `UPDATE jobs SET status='dead',lease_until=NULL,last_error='conflicting_provider_evidence' WHERE id=$1`, job); e != nil {
				return e
			}
			return s.audit(ctx, tx, "worker", "payment.evidence_conflict", payment, map[string]any{"observation_id": obs})
		}
		if r.UpstreamID != "" {
			if e = exec(tx, ctx, `UPDATE payments SET provider_reference=$2 WHERE id=$1`, payment, r.UpstreamID); e != nil {
				return e
			}
		}
		if r.Status != "succeeded" && r.Status != "failed" {
			state := "queued"
			if attempt >= 30 {
				state = "dead"
			}
			if e = exec(tx, ctx, `UPDATE payments SET status='pending',updated_at=$2 WHERE id=$1 AND status NOT IN('succeeded','failed')`, payment, s.Now()); e != nil {
				return e
			}
			return exec(tx, ctx, `UPDATE jobs SET status=$2,available_at=$3,lease_until=NULL,last_error='awaiting_original_outcome' WHERE id=$1`, job, state, s.Now().Add(retryDelay(attempt)))
		}
		if p.Status != "succeeded" && p.Status != "failed" {
			if e = lockAccounts(ctx, tx, "wallet:"+owner, "house:clearing"); e != nil {
				return e
			}
			holdState := "released"
			if r.Status == "succeeded" {
				holdState = "captured"
			}
			changed, e := tx.ExecContext(ctx, `UPDATE holds SET status=$2 WHERE payment_id=$1 AND status='active'`, payment, holdState)
			if e != nil {
				return e
			}
			n, _ := changed.RowsAffected()
			if n != 1 {
				return errors.New("financial operation has no active hold")
			}
			if r.Status == "succeeded" {
				if _, e = s.postJournal(ctx, tx, payment, "external_payment", map[string]Money{"wallet:" + owner: -p.Total, "house:clearing": p.Total}); e != nil {
					return e
				}
			}
			if e = exec(tx, ctx, `UPDATE payments SET status=$2,updated_at=$3 WHERE id=$1`, payment, r.Status, s.Now()); e != nil {
				return e
			}
			workflow := "transfer-completed"
			if r.Status == "failed" {
				workflow = "transfer-failed"
			}
			if p.Kind == "bill" {
				workflow = "bills-fulfilment-delayed"
				if r.Status == "failed" {
					workflow = "bills-failed"
				}
			}
			if p.Kind != "bill" || !r.Fulfilled || r.Status == "failed" {
				if e = s.notify(ctx, tx, owner, workflow, payment, map[string]any{"amount_minor": p.Amount, "currency": "NGN"}, false); e != nil {
					return e
				}
			}
			if e = s.audit(ctx, tx, "worker", "payment."+r.Status, payment, map[string]any{"observation_id": obs}); e != nil {
				return e
			}
		}
		jobState := "done"
		if p.Kind == "bill" && r.Status == "succeeded" {
			if r.Fulfilled && p.FulfilmentStatus != "ready" {
				sealed, e := s.Config.Box.Seal(r.Fulfilment, "fulfilment:"+payment)
				if e != nil {
					return e
				}
				if e = exec(tx, ctx, `UPDATE payments SET fulfilment_status='ready',fulfilment_enc=$2,updated_at=$3 WHERE id=$1`, payment, sealed, s.Now()); e != nil {
					return e
				}
				if e = s.notify(ctx, tx, owner, "bills-other-completed", payment, map[string]any{"amount_minor": p.Amount, "currency": "NGN"}, false); e != nil {
					return e
				}
			} else if p.FulfilmentStatus != "ready" {
				jobState = "queued"
				if attempt >= 30 {
					jobState = "dead"
				}
			}
		}
		return exec(tx, ctx, `UPDATE jobs SET status=$2,available_at=$3,lease_until=NULL,last_error='' WHERE id=$1`, job, jobState, s.Now().Add(retryDelay(attempt)))
	})
}
func (s *Service) Fulfilment(ctx context.Context, owner, id string) (map[string]any, error) {
	var state, sealed, status string
	e := s.DB.QueryRowContext(ctx, `SELECT fulfilment_status,fulfilment_enc,status FROM payments WHERE id=$1 AND owner_id=$2 AND kind='bill'`, id, owner).Scan(&state, &sealed, &status)
	if e != nil {
		return nil, isMissing(e)
	}
	result := map[string]any{"status": state, "payment_status": status}
	if state == "ready" {
		raw, e := s.Config.Box.Open(sealed, "fulfilment:"+id)
		if e != nil {
			return nil, e
		}
		result["value"] = raw
	}
	return result, nil
}
func (s *Service) Scheduler(ctx context.Context) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		if e := exec(tx, ctx, `UPDATE notification_intents SET state='expired',payload_enc=CASE WHEN secret THEN '' ELSE payload_enc END WHERE expires_at<$1 AND state IN('queued','leased')`, s.Now()); e != nil {
			return e
		}
		if e := exec(tx, ctx, `DELETE FROM rate_limits WHERE window_at<$1`, s.Now().Add(-48*time.Hour)); e != nil {
			return e
		}
		if e := exec(tx, ctx, `UPDATE challenges SET consumed=true WHERE expires_at<$1 AND NOT consumed`, s.Now()); e != nil {
			return e
		}
		return exec(tx, ctx, `UPDATE sessions SET revoked=true WHERE refresh_expires_at<$1 AND NOT revoked`, s.Now())
	})
}
