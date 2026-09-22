package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

// BootstrapStaff is an offline first-administrator operation. It is not exposed by HTTP.
func (s *Service) BootstrapStaff(ctx context.Context, email, name, password string) (string, error) {
	email, e := normalEmail(email)
	if e != nil {
		return "", e
	}
	if !safeText(name, 80) {
		return "", Invalid("invalid name")
	}
	hash, e := security.Password(password, s.Config.Pepper)
	if e != nil {
		return "", Invalid(e.Error())
	}
	id := security.Random("usr_", 18)
	e = s.transact(ctx, func(tx *sql.Tx) error {
		if e := exec(tx, ctx, `SELECT pg_advisory_xact_lock(818426002)`); e != nil {
			return e
		}
		var n int
		if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE role<>'customer'`).Scan(&n); e != nil {
			return e
		}
		if n != 0 {
			return conflict("staff bootstrap is closed")
		}
		if e := exec(tx, ctx, `INSERT INTO users(id,email,name,password_hash,role,verified) VALUES($1,$2,$3,$4,'admin',true)`, id, email, name, hash); e != nil {
			return e
		}
		return s.audit(ctx, tx, "offline-bootstrap", "staff.created", id, map[string]any{"role": "admin"})
	})
	return id, e
}

// CreateStaff requires an authenticated MFA-ready administrator. The temporary password
// is chosen by the operator and is never returned, logged, or emailed by this method.
func (s *Service) CreateStaff(ctx context.Context, p Principal, email, name, password, role string) (string, error) {
	if e := requireRole(p, "admin"); e != nil {
		return "", e
	}
	if role != "admin" && role != "finance" && role != "compliance" && role != "support" && role != "platform" {
		return "", Invalid("invalid staff role")
	}
	email, e := normalEmail(email)
	if e != nil {
		return "", e
	}
	if !safeText(name, 80) {
		return "", Invalid("invalid name")
	}
	hash, e := security.Password(password, s.Config.Pepper)
	if e != nil {
		return "", Invalid(e.Error())
	}
	id := security.Random("usr_", 18)
	e = s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		p.User = u
		if e = requireRole(p, "admin"); e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO users(id,email,name,password_hash,role,verified) VALUES($1,$2,$3,$4,$5,true)`, id, email, name, hash, role); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "staff.created", id, map[string]any{"role": role})
	})
	return id, e
}
func (s *Service) CustomerList(ctx context.Context, p Principal, limit int, before string) ([]User, error) {
	if e := requireRole(p, "admin", "support", "compliance", "finance"); e != nil {
		return nil, e
	}
	if limit < 1 || limit > 100 {
		return nil, Invalid("invalid limit")
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,email,name,role,status,verified,tier,mfa_enabled,pin_hash<>'',version,created_at FROM users WHERE role='customer' AND ($1='' OR id<$1) ORDER BY id DESC LIMIT $2`, before, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if e = rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Status, &u.Verified, &u.Tier, &u.MFA, &u.PINSet, &u.Version, &u.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
func (s *Service) KYCSubmit(ctx context.Context, p Principal, evidence string) (string, error) {
	if e := p.Customer(); e != nil {
		return "", e
	}
	if !safeText(evidence, 250) || strings.Contains(evidence, "://") {
		return "", Invalid("a private evidence reference, not a public URL, is required")
	}
	id := stringID("kyc_")
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if !u.Verified || u.Status != "active" {
			return denied()
		}
		var exists bool
		if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM kyc_cases WHERE owner_id=$1 AND status IN('submitted','information_required'))`, u.ID).Scan(&exists); e != nil {
			return e
		}
		if exists {
			return conflict("an identity review is already open")
		}
		if e = exec(tx, ctx, `INSERT INTO kyc_cases(id,owner_id,evidence_ref) VALUES($1,$2,$3)`, id, u.ID, evidence); e != nil {
			return e
		}
		return s.notify(ctx, tx, u.ID, "kyc-submitted", id, nil, false)
	})
	return id, e
}
func (s *Service) KYCList(ctx context.Context, p Principal) ([]map[string]any, error) {
	staff := p.Audience == "staff"
	if staff {
		if e := requireRole(p, "admin", "compliance"); e != nil {
			return nil, e
		}
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,owner_id,status,evidence_ref,created_at FROM kyc_cases WHERE ($1 OR owner_id=$2) ORDER BY created_at DESC LIMIT 100`, staff, p.User.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, owner, status, evidence string
		var at time.Time
		if e = rows.Scan(&id, &owner, &status, &evidence, &at); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "owner_id": owner, "status": status, "evidence_reference": evidence, "created_at": at})
	}
	return out, rows.Err()
}

type PolicyInput struct {
	PerPayment  Money `json:"per_payment_minor"`
	Daily       Money `json:"daily_minor"`
	InternalFee Money `json:"internal_fee_minor"`
	Enabled     bool  `json:"payments_enabled"`
}
type ProposalInput struct {
	Action string       `json:"action"`
	Target string       `json:"target"`
	Reason string       `json:"reason"`
	Tier   int          `json:"tier"`
	Policy *PolicyInput `json:"policy,omitempty"`
}

func proposalRoles(action string) []string {
	if action == "policy" {
		return []string{"admin", "finance"}
	}
	return []string{"admin", "compliance"}
}
func (s *Service) Propose(ctx context.Context, p Principal, in ProposalInput) (string, error) {
	if e := requireRole(p, proposalRoles(in.Action)...); e != nil {
		return "", e
	}
	if !safeText(in.Reason, 500) {
		return "", Invalid("a bounded review reason is required")
	}
	if in.Action != "kyc_approve" && in.Action != "kyc_decline" && in.Action != "restrict" && in.Action != "restore" && in.Action != "policy" {
		return "", Invalid("unsupported proposal")
	}
	if in.Action == "kyc_approve" && (in.Tier < 1 || in.Tier > 3) {
		return "", Invalid("approved tier must be 1 to 3")
	}
	if in.Action == "policy" {
		if in.Policy == nil || in.Policy.PerPayment <= 0 || in.Policy.Daily < in.Policy.PerPayment || in.Policy.Daily > MaxMoney || in.Policy.InternalFee < 0 || in.Policy.InternalFee > in.Policy.PerPayment {
			return "", Invalid("invalid payment policy")
		}
		in.Target = "payment-policy"
	}
	id := stringID("prop_")
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		p.User = u
		if e = requireRole(p, proposalRoles(in.Action)...); e != nil {
			return e
		}
		var version int64
		if in.Action == "policy" {
			e = tx.QueryRowContext(ctx, `SELECT version FROM policies WHERE id=1`).Scan(&version)
		} else {
			target := in.Target
			if strings.HasPrefix(in.Action, "kyc_") {
				e = tx.QueryRowContext(ctx, `SELECT owner_id FROM kyc_cases WHERE id=$1 AND status IN('submitted','information_required')`, in.Target).Scan(&target)
				if e != nil {
					return isMissing(e)
				}
			}
			e = tx.QueryRowContext(ctx, `SELECT version FROM users WHERE id=$1 AND role='customer'`, target).Scan(&version)
		}
		if e != nil {
			return isMissing(e)
		}
		if e = exec(tx, ctx, `INSERT INTO proposals(id,maker_id,action,target,target_version,payload,reason,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, u.ID, in.Action, in.Target, version, jsonText(in), in.Reason, s.Now().Add(24*time.Hour)); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "proposal.created", id, map[string]any{"action": in.Action})
	})
	return id, e
}
func (s *Service) Decide(ctx context.Context, p Principal, id string, approve bool) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		p.User = u
		var maker, action, target, status string
		var version int64
		var payload []byte
		var expires time.Time
		e = tx.QueryRowContext(ctx, `SELECT maker_id,action,target,target_version,payload,status,expires_at FROM proposals WHERE id=$1 FOR UPDATE`, id).Scan(&maker, &action, &target, &version, &payload, &status, &expires)
		if e != nil {
			return isMissing(e)
		}
		if e = requireRole(p, proposalRoles(action)...); e != nil {
			return e
		}
		if maker == u.ID {
			return &Fault{403, "self_approval_forbidden", "a different eligible operator must decide"}
		}
		if status != "pending" {
			return conflict("proposal already decided")
		}
		if !expires.After(s.Now()) {
			return conflict("proposal expired")
		}
		makerUser, e := s.user(ctx, tx, maker, false)
		if e != nil {
			return e
		}
		if makerUser.Status != "active" {
			return denied()
		}
		if e = requireRole(Principal{User: makerUser, Audience: "staff", MFAReady: true}, proposalRoles(action)...); e != nil {
			return e
		}
		next := "rejected"
		if approve {
			next = "approved"
			var in ProposalInput
			if e = json.Unmarshal(payload, &in); e != nil {
				return e
			}
			if action == "policy" {
				r, e := tx.ExecContext(ctx, `UPDATE policies SET version=version+1,per_payment=$2,daily=$3,internal_fee=$4,payments_enabled=$5 WHERE id=1 AND version=$1`, version, int64(in.Policy.PerPayment), int64(in.Policy.Daily), int64(in.Policy.InternalFee), in.Policy.Enabled)
				if e != nil {
					return e
				}
				n, _ := r.RowsAffected()
				if n != 1 {
					return conflict("policy changed since proposal")
				}
			} else {
				owner := target
				if strings.HasPrefix(action, "kyc_") {
					if e = tx.QueryRowContext(ctx, `SELECT owner_id FROM kyc_cases WHERE id=$1 AND status IN('submitted','information_required') FOR UPDATE`, target).Scan(&owner); e != nil {
						return isMissing(e)
					}
				}
				customer, e := s.user(ctx, tx, owner, true)
				if e != nil {
					return e
				}
				if customer.Version != version {
					return conflict("customer changed since proposal")
				}
				workflow := "security-account-restored"
				switch action {
				case "restrict":
					if customer.Status == "closed" {
						return conflict("closed account cannot be restricted")
					}
					e = exec(tx, ctx, `UPDATE users SET status='restricted',version=version+1 WHERE id=$1`, owner)
					workflow = "security-account-restricted"
				case "restore":
					if customer.Status != "restricted" {
						return conflict("account is not restricted")
					}
					e = exec(tx, ctx, `UPDATE users SET status='active',version=version+1 WHERE id=$1`, owner)
				case "kyc_approve":
					if !customer.Verified || customer.Status != "active" {
						return denied()
					}
					if e = exec(tx, ctx, `UPDATE kyc_cases SET status='approved' WHERE id=$1`, target); e == nil {
						e = exec(tx, ctx, `UPDATE users SET tier=$2,version=version+1 WHERE id=$1`, owner, in.Tier)
					}
					workflow = "kyc-approved"
				case "kyc_decline":
					if e = exec(tx, ctx, `UPDATE kyc_cases SET status='declined' WHERE id=$1`, target); e == nil {
						e = exec(tx, ctx, `UPDATE users SET version=version+1 WHERE id=$1`, owner)
					}
					workflow = "kyc-unsuccessful"
				}
				if e != nil {
					return e
				}
				if e = s.notify(ctx, tx, owner, workflow, id, nil, false); e != nil {
					return e
				}
			}
		}
		if e = exec(tx, ctx, `UPDATE proposals SET status=$2,checker_id=$3 WHERE id=$1`, id, next, u.ID); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "proposal."+next, id, map[string]any{"action": action})
	})
}
func (s *Service) Proposals(ctx context.Context, p Principal) ([]map[string]any, error) {
	if e := requireRole(p, "admin", "finance", "compliance"); e != nil {
		return nil, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,maker_id,action,target,status,reason,expires_at FROM proposals WHERE ($1='admin' OR ($1='finance' AND action='policy') OR ($1='compliance' AND action<>'policy')) ORDER BY created_at DESC LIMIT 100`, p.User.Role)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, maker, action, target, status, reason string
		var expires time.Time
		if e = rows.Scan(&id, &maker, &action, &target, &status, &reason, &expires); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "maker_id": maker, "action": action, "target": target, "status": status, "reason": reason, "expires_at": expires})
	}
	return out, rows.Err()
}
func (s *Service) Policy(ctx context.Context) (map[string]any, error) {
	var version int64
	var per, daily, fee Money
	var enabled bool
	e := s.DB.QueryRowContext(ctx, `SELECT version,per_payment,daily,internal_fee,payments_enabled FROM policies WHERE id=1`).Scan(&version, &per, &daily, &fee, &enabled)
	return map[string]any{"version": version, "per_payment_minor": per, "daily_minor": daily, "internal_fee_minor": fee, "payments_enabled": enabled, "currency": "NGN"}, e
}
func (s *Service) Overview(ctx context.Context, p Principal) (map[string]any, error) {
	if e := requireRole(p, "admin", "finance", "platform"); e != nil {
		return nil, e
	}
	var customers, pending, dead int
	var book, held Money
	e := s.DB.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM users WHERE role='customer'),(SELECT count(*) FROM payments WHERE status IN('accepted','submitted','pending','pending_review')),(SELECT count(*) FROM jobs WHERE status='dead'),coalesce(sum(balance),0),coalesce(sum(reserved),0) FROM accounts WHERE kind='wallet'`).Scan(&customers, &pending, &dead, &book, &held)
	return map[string]any{"customers": customers, "pending_payments": pending, "dead_jobs": dead, "wallet_liability_minor": book, "held_minor": held, "currency": "NGN"}, e
}
func (s *Service) Audit(ctx context.Context, p Principal, limit int, before string) ([]map[string]any, error) {
	if e := requireRole(p, "admin", "compliance"); e != nil {
		return nil, e
	}
	if limit < 1 || limit > 100 {
		return nil, Invalid("invalid limit")
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,actor,action,target,details,created_at FROM audit_events WHERE ($1='' OR id<$1) ORDER BY id DESC LIMIT $2`, before, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, actor, action, target string
		var details json.RawMessage
		var at time.Time
		if e = rows.Scan(&id, &actor, &action, &target, &details, &at); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "actor": actor, "action": action, "target": target, "details": details, "created_at": at})
	}
	return out, rows.Err()
}
func (s *Service) RetryJob(ctx context.Context, p Principal, payment string) error {
	if e := requireRole(p, "admin", "finance"); e != nil {
		return e
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		p.User = u
		if e = requireRole(p, "admin", "finance"); e != nil {
			return e
		}
		var status string
		if e := tx.QueryRowContext(ctx, `SELECT status FROM payments WHERE id=$1 FOR UPDATE`, payment).Scan(&status); e != nil {
			return isMissing(e)
		}
		if status != "pending" && status != "submitted" && status != "succeeded" {
			return conflict("only original-reference queries may be requeued")
		}
		r, e := tx.ExecContext(ctx, `UPDATE jobs SET status='queued',attempts=0,available_at=$2,lease_until=NULL,lease_token='' WHERE object_id=$1 AND status='dead'`, payment, s.Now())
		if e != nil {
			return e
		}
		n, _ := r.RowsAffected()
		if n != 1 {
			return conflict("job is not awaiting retry")
		}
		return s.audit(ctx, tx, p.User.ID, "payment.requery_requested", payment, map[string]any{})
	})
}

// SeedLocal creates synthetic integration data only in the explicit local environment.
func (s *Service) SeedLocal(ctx context.Context, email, name, password, pin string) (string, error) {
	if !safeText(name, 80) {
		return "", Invalid("invalid fixture name")
	}
	if s.Config.Environment != "local" {
		return "", denied()
	}
	email, e := normalEmail(email)
	if e != nil {
		return "", e
	}
	hash, e := security.Password(password, s.Config.Pepper)
	if e != nil {
		return "", e
	}
	pinHash, e := security.PIN(pin, s.Config.Pepper)
	if e != nil {
		return "", e
	}
	id := security.Random("usr_", 18)
	e = s.transact(ctx, func(tx *sql.Tx) error {
		var existing string
		e := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE email=$1`, email).Scan(&existing)
		if e == nil {
			return conflict("synthetic account already exists")
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO users(id,email,name,password_hash,pin_hash,role,verified,tier) VALUES($1,$2,$3,$4,$5,'customer',true,1)`, id, email, name, hash, pinHash); e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO accounts(id,owner_id,kind,currency) VALUES($1,$2,'wallet','NGN')`, "wallet:"+id, id); e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO preferences(owner_id) VALUES($1)`, id); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE policies SET payments_enabled=true WHERE id=1`); e != nil {
			return e
		}
		return s.audit(ctx, tx, "local-fixture", "synthetic-account.created", id, map[string]any{})
	})
	if e != nil {
		return "", e
	}
	_, e = s.CreditFunding(ctx, "local-fixture", id, id, 10000000, "NGN")
	return id, e
}
