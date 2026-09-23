package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

// These are product controls, not representations of regulatory tier limits.
type CustomerControls struct {
	Handle         string `json:"handle"`
	Discoverable   bool   `json:"discoverable"`
	Frozen         bool   `json:"frozen"`
	PerPayment     Money  `json:"per_payment_minor"`
	Daily          Money  `json:"daily_minor"`
	DailyRemaining Money  `json:"daily_remaining_minor"`
}
type ControlsInput struct {
	Handle       string `json:"handle"`
	Discoverable bool   `json:"discoverable"`
	PerPayment   Money  `json:"per_payment_minor"`
	Daily        Money  `json:"daily_minor"`
	Password     string `json:"password"`
	MFACode      string `json:"mfa_code,omitempty"`
}

func (s *Service) controls(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, owner string) (CustomerControls, error) {
	var out CustomerControls
	e := db.QueryRowContext(ctx, `SELECT coalesce(c.handle,''),coalesce(c.discoverable,false),coalesce(c.frozen,false),least(coalesce(c.per_limit,p.per_payment),p.per_payment),least(coalesce(c.daily_limit,p.daily),p.daily) FROM policies p LEFT JOIN customer_controls c ON c.owner_id=$1 WHERE p.id=1`, owner).Scan(&out.Handle, &out.Discoverable, &out.Frozen, &out.PerPayment, &out.Daily)
	if e != nil {
		return out, e
	}
	var spent Money
	day := s.Now().Truncate(24 * time.Hour)
	e = db.QueryRowContext(ctx, `SELECT coalesce(sum(total),0) FROM payments WHERE owner_id=$1 AND created_at>=$2 AND created_at<$3 AND status<>'failed'`, owner, day, day.Add(24*time.Hour)).Scan(&spent)
	out.DailyRemaining = out.Daily - spent
	if out.DailyRemaining < 0 {
		out.DailyRemaining = 0
	}
	return out, e
}
func (s *Service) Controls(ctx context.Context, p Principal) (CustomerControls, error) {
	if e := p.Customer(); e != nil {
		return CustomerControls{}, e
	}
	return s.controls(ctx, s.DB, p.User.ID)
}
func (s *Service) ChangeControls(ctx context.Context, p Principal, in ControlsInput) error {
	if e := p.Customer(); e != nil {
		return e
	}
	in.Handle = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(in.Handle, "@")))
	if in.Handle != "" && !regexp.MustCompile(`^[a-z][a-z0-9_]{3,23}$`).MatchString(in.Handle) {
		return Invalid("handle must have 4 to 24 lowercase letters, digits or underscores and start with a letter")
	}
	if in.Discoverable && in.Handle == "" {
		return Invalid("choose a handle before enabling discovery")
	}
	if in.PerPayment <= 0 || in.Daily < in.PerPayment || in.Daily > MaxMoney {
		return Invalid("invalid personal limits")
	}
	if e := s.Rate(ctx, "controls:"+p.User.ID, 6, 15*time.Minute); e != nil {
		return e
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if u.Status != "active" {
			return denied()
		}
		if e = s.reauthenticate(ctx, tx, u, in.Password, in.MFACode); e != nil {
			return e
		}
		var per, daily Money
		if e = tx.QueryRowContext(ctx, `SELECT per_payment,daily FROM policies WHERE id=1 FOR SHARE`).Scan(&per, &daily); e != nil {
			return e
		}
		if in.PerPayment > per || in.Daily > daily {
			return Invalid("personal limits cannot exceed account policy")
		}
		if in.Handle != "" {
			if e = exec(tx, ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "handle:"+in.Handle); e != nil {
				return e
			}
			var taken bool
			if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM customer_controls WHERE handle=$1 AND owner_id<>$2)`, in.Handle, u.ID).Scan(&taken); e != nil {
				return e
			}
			if taken {
				return conflict("handle is not available")
			}
		}
		if e = exec(tx, ctx, `INSERT INTO customer_controls(owner_id,handle,discoverable,per_limit,daily_limit,updated_at) VALUES($1,nullif($2,''),$3,$4,$5,$6) ON CONFLICT(owner_id) DO UPDATE SET handle=excluded.handle,discoverable=excluded.discoverable,per_limit=excluded.per_limit,daily_limit=excluded.daily_limit,updated_at=excluded.updated_at`, u.ID, in.Handle, in.Discoverable, int64(in.PerPayment), int64(in.Daily), s.Now()); e != nil {
			return e
		}
		if e = s.notify(ctx, tx, u.ID, "security-personal-controls-changed", stringID("control_"), nil, false); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "customer.controls_updated", u.ID, map[string]any{"discoverable": in.Discoverable})
	})
}
func (s *Service) SecureAccount(ctx context.Context, p Principal, password, code string, frozen bool) error {
	if e := p.Customer(); e != nil {
		return e
	}
	if e := s.Rate(ctx, "sensitive:"+p.User.ID, 6, 15*time.Minute); e != nil {
		return e
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = s.reauthenticate(ctx, tx, u, password, code); e != nil {
			return e
		}
		// Removing a customer freeze does not remove a staff/compliance restriction.
		if e = exec(tx, ctx, `INSERT INTO customer_controls(owner_id,frozen) VALUES($1,$2) ON CONFLICT(owner_id) DO UPDATE SET frozen=excluded.frozen,updated_at=now()`, u.ID, frozen); e != nil {
			return e
		}
		if frozen {
			if e = exec(tx, ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1 AND id<>$2`, u.ID, p.SessionID); e != nil {
				return e
			}
		}
		if e = s.notify(ctx, tx, u.ID, "security-personal-controls-changed", stringID("freeze_"), nil, false); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "customer.freeze_changed", u.ID, map[string]any{"frozen": frozen})
	})
}
func (s *Service) SignOutAll(ctx context.Context, p Principal) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1`, u.ID); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "session.all_revoked", u.ID, map[string]any{})
	})
}
func (s *Service) LoginHistory(ctx context.Context, p Principal) ([]map[string]any, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT action,details,created_at FROM audit_events WHERE actor=$1 AND action IN('session.created','session.revoked','session.all_revoked','passkey.login') ORDER BY created_at DESC LIMIT 100`, p.User.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var action string
		var details json.RawMessage
		var at time.Time
		if e = rows.Scan(&action, &details, &at); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"action": action, "details": details, "created_at": at})
	}
	return out, rows.Err()
}
func (s *Service) FindRecipient(ctx context.Context, p Principal, handle string) (map[string]any, error) {
	if e := p.Customer(); e != nil {
		return nil, e
	}
	if e := s.eligible(p.User); e != nil {
		return nil, e
	}
	if e := s.Rate(ctx, "recipient-lookup:"+p.User.ID, 30, time.Hour); e != nil {
		return nil, e
	}
	handle = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(handle), "@"))
	if !regexp.MustCompile(`^[a-z][a-z0-9_]{3,23}$`).MatchString(handle) {
		return nil, Invalid("enter an exact Qpay handle")
	}
	var id, name string
	e := s.DB.QueryRowContext(ctx, `SELECT u.id,u.name FROM customer_controls c JOIN users u ON u.id=c.owner_id WHERE c.handle=$1 AND c.discoverable AND NOT c.frozen AND u.role='customer' AND u.status='active' AND u.verified AND u.tier>=1 AND u.id<>$2`, handle, p.User.ID).Scan(&id, &name)
	if e != nil {
		return nil, isMissing(e)
	}
	return map[string]any{"id": id, "name": name, "handle": handle, "name_kind": "display_name"}, nil
}
func (s *Service) CustomerCapabilities(ctx context.Context, p Principal) (map[string]any, error) {
	if e := p.Customer(); e != nil {
		return nil, e
	}
	u, e := s.Me(ctx, p.User.ID)
	if e != nil {
		return nil, e
	}
	c, e := s.Controls(ctx, p)
	if e != nil {
		return nil, e
	}
	state := "eligible"
	if u.Status != "active" || c.Frozen {
		state = "restricted"
	} else if !u.Verified || u.Tier < 1 {
		state = "verification_required"
	}
	var enabled bool
	if e = s.DB.QueryRowContext(ctx, `SELECT payments_enabled FROM policies WHERE id=1`).Scan(&enabled); e != nil {
		return nil, e
	}
	if state == "eligible" && !enabled {
		state = "temporarily_unavailable"
	}
	features := map[string]string{"transfers": state, "internal_transfers": state, "saved_bills": "eligible", "reminders": "eligible", "money_requests": state, "insights": "eligible", "budgets": "eligible", "personal_limits": "eligible", "account_closure": "eligible", "privacy_export": "eligible", "contact_phone": "not_offered", "autopay": "not_offered", "scheduled_internal_transfers": state, "cards": "not_offered", "interest_savings": "not_offered", "credit": "not_offered", "investments": "not_offered", "international_payments": "not_offered"}
	if !s.Config.SchedulesEnabled {
		features["scheduled_internal_transfers"] = "not_offered"
	}
	features["bank_transfers"] = state
	features["bill_payments"] = state
	if !s.Config.ExternalEnabled {
		features["bank_transfers"] = "temporarily_unavailable"
		features["bill_payments"] = "temporarily_unavailable"
	}
	features["private_uploads"] = "not_offered"
	features["funding_accounts"] = "not_offered"
	features["passkeys"] = "not_offered"
	// Optional integrations fill these from actual configured capabilities, never UI guesses.
	if s.Config.UploadStore != nil && s.Config.UploadScanner != nil {
		features["private_uploads"] = "eligible"
	}
	if s.Config.FundingGateway != nil {
		features["funding_accounts"] = state
	}
	if s.Config.Passkeys != nil {
		features["passkeys"] = "eligible"
	}
	return map[string]any{"version": 1, "currency": "NGN", "account_status": u.Status, "tier": u.Tier, "email_verified": u.Verified, "controls": c, "features": features, "checked_at": s.Now(), "limit_day_timezone": "UTC", "synthetic": s.Config.Environment == "local"}, nil
}
func (s *Service) LookupPayment(ctx context.Context, p Principal, key, quote string) (map[string]any, error) {
	if e := p.Customer(); e != nil {
		return nil, e
	}
	if key == "" && quote == "" {
		return nil, Invalid("provide the original idempotency key or quote")
	}
	if len(key) > 100 || len(quote) > 100 {
		return nil, Invalid("invalid original reference")
	}
	payment, e := scanPayment(s.DB.QueryRowContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE owner_id=$1 AND ($2='' OR idempotency_key=$2) AND ($3='' OR quote_id=$3)`, p.User.ID, key, quote))
	var f *Fault
	if errors.As(e, &f) && f.Status == 404 {
		return map[string]any{"found": false, "conclusive_failure": false}, nil
	}
	if e != nil {
		return nil, e
	}
	payment.Direction = "outgoing"
	return map[string]any{"found": true, "payment": payment}, nil
}
func (s *Service) PaymentTimeline(ctx context.Context, p Principal, id string) (map[string]any, error) {
	payment, e := s.Payment(ctx, p.User.ID, id)
	if e != nil {
		return nil, e
	}
	events := []map[string]any{{"type": "request_received", "at": payment.CreatedAt}}
	rows, e := s.DB.QueryContext(ctx, `SELECT status,created_at FROM observations WHERE payment_id=$1 ORDER BY created_at,id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var at time.Time
		if e = rows.Scan(&status, &at); e != nil {
			return nil, e
		}
		events = append(events, map[string]any{"type": "provider_observation", "status": status, "at": at})
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	var hold Money
	var holdState string
	e = s.DB.QueryRowContext(ctx, `SELECT amount,status FROM holds WHERE payment_id=$1`, id).Scan(&hold, &holdState)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return nil, e
	}
	next := "Review your confirmed transaction."
	if payment.Status != "succeeded" && payment.Status != "failed" {
		next = "We are checking the original payment. Do not submit a replacement. You can open a linked support case."
	}
	return map[string]any{"payment": payment, "events": events, "hold_minor": hold, "hold_state": holdState, "next_action": next, "last_updated": payment.UpdatedAt}, nil
}
func (s *Service) FavourBeneficiary(ctx context.Context, p Principal, id string, favourite bool) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		if _, e := s.activePrincipal(ctx, tx, p); e != nil {
			return e
		}
		r, e := tx.ExecContext(ctx, `UPDATE beneficiaries SET favourite=$3 WHERE id=$1 AND owner_id=$2 AND active`, id, p.User.ID, favourite)
		if e != nil {
			return e
		}
		n, _ := r.RowsAffected()
		if n != 1 {
			return missing()
		}
		return nil
	})
}

// ensureSpendingControl is called under the sender user lock in the committing payment transaction.
func (s *Service) ensureSpendingControl(ctx context.Context, tx *sql.Tx, owner string, total Money) error {
	c, e := s.controls(ctx, tx, owner)
	if e != nil {
		return e
	}
	if c.Frozen {
		return &Fault{403, "customer_frozen", "You have frozen outgoing payments. Review your security controls."}
	}
	if total > c.PerPayment || total > c.DailyRemaining {
		return &Fault{409, "personal_limit", "Payment exceeds your personal or account limit."}
	}
	return nil
}
