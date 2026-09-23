package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"time"
)

type MandateInput struct {
	QuoteID        string    `json:"quote_id"`
	Title          string    `json:"title"`
	Cadence        string    `json:"cadence"`
	FirstAt        time.Time `json:"first_at"`
	EndsAt         time.Time `json:"ends_at"`
	MaxDebit       Money     `json:"max_debit_minor"`
	MaxTotal       Money     `json:"max_total_minor"`
	MaxOccurrences int       `json:"max_occurrences"`
	PIN            string    `json:"pin"`
	MFACode        string    `json:"mfa_code,omitempty"`
	Consent        bool      `json:"consent"`
}

func (s *Service) CreateMandate(ctx context.Context, p Principal, in MandateInput) (string, error) {
	if !s.Config.SchedulesEnabled {
		return "", unavailable()
	}
	if e := p.Customer(); e != nil {
		return "", e
	}
	if !in.Consent || !safeText(in.Title, 100) || !in.FirstAt.After(s.Now()) || in.EndsAt.Before(in.FirstAt) || in.EndsAt.After(s.Now().AddDate(5, 0, 0)) || in.MaxOccurrences < 1 || in.MaxOccurrences > 120 || in.MaxDebit <= 0 || in.MaxTotal < in.MaxDebit || in.MaxTotal > MaxMoney {
		return "", Invalid("review the schedule, dates, consent and maximum authorised debit")
	}
	if in.Cadence != "once" && in.Cadence != "weekly" && in.Cadence != "monthly" {
		return "", Invalid("invalid recurrence")
	}
	if in.Cadence == "once" && in.MaxOccurrences != 1 {
		return "", Invalid("one-time schedules must have exactly one occurrence")
	}
	if e := s.Rate(ctx, "pin:"+p.User.ID, 5, 15*time.Minute); e != nil {
		return "", e
	}
	id := stringID("mandate_")
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = s.eligible(u); e != nil {
			return e
		}
		q, e := s.loadQuote(ctx, tx, u.ID, in.QuoteID, true)
		if e != nil {
			return e
		}
		if q.Used || !q.ExpiresAt.After(s.Now()) {
			return conflict("obtain a fresh quote before creating the schedule")
		}
		// v1 mandates support registered internal recipients only. External auto-vending
		// is withheld until the upstream quote/mandate contract is qualified.
		if q.Kind != "internal" {
			return &Fault{422, "schedule_kind_unavailable", "bank and bill payments currently support reminders; automatic execution is enabled only for internal transfers"}
		}
		if q.Total > in.MaxDebit {
			return Invalid("maximum debit is below the reviewed quote")
		}
		if e = s.ensureSpendingControl(ctx, tx, u.ID, q.Total); e != nil {
			return e
		}
		var hash string
		if e = tx.QueryRowContext(ctx, `SELECT pin_hash FROM users WHERE id=$1`, u.ID).Scan(&hash); e != nil {
			return e
		}
		if !security.Verify(in.PIN, hash, s.Config.Pepper) {
			return unauthorized()
		}
		if u.MFA {
			if e = s.checkTOTP(ctx, tx, u.ID, in.MFACode); e != nil {
				return e
			}
		}
		var count int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM payment_mandates WHERE owner_id=$1 AND status IN('active','paused')`, u.ID).Scan(&count); e != nil {
			return e
		}
		if count >= 50 {
			return Invalid("maximum fifty active schedules")
		}
		terms := in
		terms.PIN = ""
		terms.MFACode = ""
		termsHash := security.Digest(jsonText(terms))
		if e = exec(tx, ctx, `INSERT INTO payment_mandates(id,owner_id,quote_id,title,cadence,next_at,anchor_day,ends_at,max_debit,max_total,max_occurrences,terms_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, u.ID, q.ID, in.Title, in.Cadence, in.FirstAt, in.FirstAt.In(lagosZone).Day(), in.EndsAt, int64(in.MaxDebit), int64(in.MaxTotal), in.MaxOccurrences, termsHash); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE quotes SET used=true WHERE id=$1`, q.ID); e != nil {
			return e
		}
		if e = s.notify(ctx, tx, u.ID, "planner-mandate-created", id, nil, false); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "mandate.authorised", id, map[string]any{"terms_hash": termsHash})
	})
	return id, e
}
func (s *Service) Mandates(ctx context.Context, p Principal) ([]map[string]any, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,title,cadence,next_at,ends_at,max_debit,max_total,max_occurrences,occurrences,committed_total,status,version FROM payment_mandates WHERE owner_id=$1 ORDER BY created_at DESC LIMIT 100`, p.User.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, title, cadence, state string
		var next, end time.Time
		var maxDebit, maxTotal, spent Money
		var max, count int
		var version int64
		if e = rows.Scan(&id, &title, &cadence, &next, &end, &maxDebit, &maxTotal, &max, &count, &spent, &state, &version); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "title": title, "cadence": cadence, "next_at": next, "ends_at": end, "max_debit_minor": maxDebit, "max_total_minor": maxTotal, "max_occurrences": max, "occurrences": count, "committed_total_minor": spent, "status": state, "version": version})
	}
	return out, rows.Err()
}
func (s *Service) MandateState(ctx context.Context, p Principal, id, state string, version int64) error {
	if state != "paused" && state != "cancelled" {
		return Invalid("pause or cancel this mandate; create a newly authorised mandate to resume")
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		r, e := tx.ExecContext(ctx, `UPDATE payment_mandates SET status=$3,version=version+1 WHERE id=$1 AND owner_id=$2 AND version=$4 AND status IN('active','paused')`, id, u.ID, state, version)
		if e != nil {
			return e
		}
		n, _ := r.RowsAffected()
		if n != 1 {
			return conflict("schedule changed; reload")
		}
		return s.audit(ctx, tx, u.ID, "mandate."+state, id, map[string]any{})
	})
}
func (s *Service) MandateHistory(ctx context.Context, p Principal, id string) ([]map[string]any, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT o.id,o.due_at,coalesce(o.payment_id,''),o.status,o.reason FROM mandate_occurrences o JOIN payment_mandates m ON m.id=o.mandate_id WHERE m.id=$1 AND m.owner_id=$2 ORDER BY o.due_at DESC LIMIT 120`, id, p.User.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var oid, pay, state, reason string
		var due time.Time
		if e = rows.Scan(&oid, &due, &pay, &state, &reason); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": oid, "due_at": due, "payment_id": pay, "status": state, "reason": reason})
	}
	return out, rows.Err()
}
func (s *Service) WorkMandate(ctx context.Context) (bool, error) {
	if !s.Config.SchedulesEnabled {
		return false, nil
	}
	var id, owner, qid string
	e := s.DB.QueryRowContext(ctx, `SELECT id,owner_id,quote_id FROM payment_mandates WHERE status='active' AND next_at<=$1 ORDER BY next_at LIMIT 1`, s.Now()).Scan(&id, &owner, &qid)
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	snapshot, e := s.loadQuote(ctx, s.DB, owner, qid, false)
	if e != nil {
		return true, e
	}
	e = s.transact(ctx, func(tx *sql.Tx) error {
		if e := lockUsers(ctx, tx, owner, snapshot.Destination.UserID); e != nil {
			return e
		}
		var due, end time.Time
		var cadence, state, title string
		var maxDebit, maxTotal, spent Money
		var max, count, anchor int
		if e := tx.QueryRowContext(ctx, `SELECT next_at,ends_at,cadence,status,max_debit,max_total,committed_total,max_occurrences,occurrences,anchor_day,title FROM payment_mandates WHERE id=$1 FOR UPDATE`, id).Scan(&due, &end, &cadence, &state, &maxDebit, &maxTotal, &spent, &max, &count, &anchor, &title); e != nil {
			return e
		}
		if state != "active" || due.After(s.Now()) {
			return nil
		}
		occurrence := id + "_" + due.UTC().Format("20060102T150405")
		reason := ""
		u, e := s.user(ctx, tx, owner, false)
		if e != nil {
			return e
		}
		receiver, e := s.user(ctx, tx, snapshot.Destination.UserID, false)
		if e != nil {
			return e
		}
		var enabled bool
		var version int64
		var fee Money
		if e = tx.QueryRowContext(ctx, `SELECT payments_enabled,version,internal_fee FROM policies WHERE id=1 FOR SHARE`).Scan(&enabled, &version, &fee); e != nil {
			return e
		}
		total, e := addMoney(snapshot.Amount, fee)
		if e != nil {
			return e
		}
		if e = s.eligible(u); e != nil {
			reason = "account_not_eligible"
		} else if e = s.eligible(receiver); e != nil {
			reason = "recipient_not_eligible"
		} else if !enabled {
			reason = "payments_unavailable"
		} else if s.Now().After(end) || count >= max || total > maxDebit || spent > maxTotal-total {
			reason = "authorisation_limit"
		} else if s.Now().Sub(due) > 24*time.Hour {
			reason = "missed_occurrence_requires_review"
		} else if e = s.ensureSpendingControl(ctx, tx, owner, total); e != nil {
			reason = "personal_or_daily_limit"
		}
		if e = lockAccounts(ctx, tx, "wallet:"+owner, "wallet:"+receiver.ID, "house:fees"); e != nil {
			return e
		}
		var available Money
		if e = tx.QueryRowContext(ctx, `SELECT balance-reserved FROM accounts WHERE owner_id=$1`, owner).Scan(&available); e != nil {
			return e
		}
		if available < total {
			reason = "insufficient_funds"
		}
		if reason != "" {
			if e = exec(tx, ctx, `INSERT INTO mandate_occurrences(id,mandate_id,due_at,status,reason) VALUES($1,$2,$3,'action_required',$4) ON CONFLICT(mandate_id,due_at) DO NOTHING`, occurrence, id, due, reason); e != nil {
				return e
			}
			if e = exec(tx, ctx, `UPDATE payment_mandates SET status='paused',version=version+1 WHERE id=$1`, id); e != nil {
				return e
			}
			return s.notifyOccurrence(ctx, tx, owner, "planner-mandate-action", id, occurrence, nil, false)
		}
		quote := stringID("quo_")
		payment := stringID("pay_")
		qenc, e := s.Config.Box.Seal(jsonText(snapshot.Destination), "quote:"+quote)
		if e != nil {
			return e
		}
		penc, e := s.Config.Box.Seal(jsonText(snapshot.Destination), "payment:"+payment)
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO quotes(id,owner_id,kind,destination_enc,amount,fee,total,currency,narration,policy_version,expires_at,used) VALUES($1,$2,'internal',$3,$4,$5,$6,'NGN',$7,$8,$9,true)`, quote, owner, qenc, int64(snapshot.Amount), int64(fee), int64(total), title, version, s.Now()); e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO payments(id,owner_id,recipient_id,quote_id,idempotency_key,request_hash,kind,status,amount,fee,total,currency,destination_enc,narration,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,'internal','succeeded',$7,$8,$9,'NGN',$10,$11,$12,$12)`, payment, owner, receiver.ID, quote, occurrence, security.Digest(occurrence), int64(snapshot.Amount), int64(fee), int64(total), penc, title, s.Now()); e != nil {
			return e
		}
		if _, e = s.postJournal(ctx, tx, payment, "scheduled_internal_transfer", map[string]Money{"wallet:" + owner: -total, "wallet:" + receiver.ID: snapshot.Amount, "house:fees": fee}); e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO mandate_occurrences(id,mandate_id,due_at,payment_id,status) VALUES($1,$2,$3,$4,'submitted')`, occurrence, id, due, payment); e != nil {
			return e
		}
		next := NextDue(due, cadence, anchor)
		count++
		state = "active"
		if cadence == "once" || count >= max || spent+total >= maxTotal || next.After(end) {
			state = "completed"
			next = due
		}
		if e = exec(tx, ctx, `UPDATE payment_mandates SET next_at=$2,occurrences=$3,committed_total=committed_total+$4,status=$5,version=version+1 WHERE id=$1`, id, next, count, int64(total), state); e != nil {
			return e
		}
		for _, v := range []struct{ uid, key string }{{owner, "transfer-completed"}, {receiver.ID, "transfer-internal-received"}} {
			if e = s.notify(ctx, tx, v.uid, v.key, payment, map[string]any{"amount_minor": snapshot.Amount}, false); e != nil {
				return e
			}
		}
		return s.audit(ctx, tx, "scheduler:"+id, "mandate.executed", payment, map[string]any{"occurrence": occurrence})
	})
	return true, e
}
