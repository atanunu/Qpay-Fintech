package service

import (
	"context"
	"database/sql"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"time"
)

func (s *Service) AdminPaymentControl(ctx context.Context, p Principal) (map[string]any, error) {
	out := map[string]any{}
	err := s.transact(ctx, func(tx *sql.Tx) error {
		var e error
		p, e = s.consolePrincipal(ctx, tx, p, "admin", "platform")
		if e != nil {
			return e
		}
		var version int64
		var enabled bool
		if e = tx.QueryRowContext(ctx, `SELECT version,payments_enabled FROM policies WHERE id=1`).Scan(&version, &enabled); e != nil {
			return e
		}
		out["version"] = version
		out["payments_enabled"] = enabled
		return s.audit(ctx, tx, p.User.ID, "admin.payment_control_read", "payment-policy", map[string]any{})
	})
	return out, err
}
func (s *Service) AcceptStaffRecovery(ctx context.Context, in StaffInvitationAccept) error {
	if len(in.Token) != 43 {
		return denied()
	}
	if e := s.Rate(ctx, "staff-recovery:"+security.Digest(in.Token), 5, time.Hour); e != nil {
		return e
	}
	hash, e := security.Password(in.Password, s.Config.Pepper)
	if e != nil {
		return Invalid(e.Error())
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		if e := exec(tx, ctx, `SELECT pg_advisory_xact_lock(818426003)`); e != nil {
			return e
		}
		var id, owner, maker, checker string
		var version int64
		var expires time.Time
		var consumed sql.NullTime
		e := tx.QueryRowContext(ctx, `SELECT g.id,g.user_id,g.target_version,g.expires_at,g.consumed_at,c.maker_id,c.checker_id FROM staff_recovery_grants g JOIN admin_controls c ON c.id=g.control_id WHERE g.token_hash=$1 AND c.status='approved' FOR UPDATE OF g`, security.Digest(in.Token)).Scan(&id, &owner, &version, &expires, &consumed, &maker, &checker)
		if e == sql.ErrNoRows {
			return denied()
		}
		if e != nil {
			return e
		}
		if consumed.Valid || !expires.After(s.Now()) || maker == checker || owner == maker || owner == checker {
			return denied()
		}
		for _, staff := range []string{maker, checker} {
			u, e := s.user(ctx, tx, staff, false)
			if e != nil {
				return e
			}
			if u.Status != "active" || u.Role != "admin" || !u.MFA {
				return denied()
			}
		}
		u, e := s.user(ctx, tx, owner, true)
		if e != nil {
			return e
		}
		if u.Version != version || u.Role == "customer" || u.Status == "closed" {
			return conflict("staff changed since recovery approval")
		}
		if e = exec(tx, ctx, `UPDATE users SET password_hash=$2,mfa_enabled=false,mfa_secret='',mfa_pending='',mfa_pending_expires=NULL,mfa_counter=-1,version=version+1 WHERE id=$1`, owner, hash); e != nil {
			return e
		}
		if e = exec(tx, ctx, `DELETE FROM mfa_recovery_codes WHERE user_id=$1`, owner); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1`, owner); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE staff_recovery_grants SET consumed_at=$2 WHERE id=$1`, id, s.Now()); e != nil {
			return e
		}
		return s.audit(ctx, tx, owner, "staff.recovery_consumed", id, map[string]any{"mfa_reenrolment_required": true, "status_unchanged": true})
	})
}

// Access to bill value is separately authorised, reasoned and never included in lists/exports.
func (s *Service) AdminFulfilment(ctx context.Context, p Principal, id, reason string) (map[string]any, error) {
	out := map[string]any{}
	if len(reason) < 8 || !safeText(reason, 1000) {
		return nil, Invalid("access explanation required")
	}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = s.consolePrincipal(ctx, tx, p, "admin", "support")
		if err != nil {
			return err
		}
		var state, status, sealed string
		if err = tx.QueryRowContext(ctx, `SELECT status,fulfilment_status,fulfilment_enc FROM payments WHERE id=$1 AND kind='bill'`, id).Scan(&status, &state, &sealed); err != nil {
			return isMissing(err)
		}
		out["status"] = state
		out["payment_status"] = status
		if status == "succeeded" && state == "ready" {
			raw, err := s.Config.Box.Open(sealed, "fulfilment:"+id)
			if err != nil {
				return err
			}
			out["value"] = raw
		}
		return s.audit(ctx, tx, p.User.ID, "admin.bill_value_access", id, map[string]any{"reason": reason})
	})
	return out, e
}
