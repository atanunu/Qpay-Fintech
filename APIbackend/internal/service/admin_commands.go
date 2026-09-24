package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

var adminActionRoles = map[string][]string{
	"note":        {"admin", "finance", "compliance", "support", "platform"},
	"work_create": {"admin", "finance", "compliance", "platform"}, "work_update": {"admin", "finance", "compliance", "platform"},
	"support_assign": {"admin", "support"}, "customer_sessions_revoke": {"admin", "compliance"},
	"schedule_stop": {"admin", "compliance"}, "notification_suppress": {"admin", "platform"},
	"emergency_stop": {"admin", "platform"}, "control_propose": {"admin", "platform"}, "control_decide": {"admin", "platform"},
	"invite_propose": {"admin"}, "invite_decide": {"admin"}, "invite_revoke": {"admin"},
}

type AdminCommandInput struct {
	Action     string `json:"action"`
	Target     string `json:"target"`
	Resource   string `json:"resource"`
	Reason     string `json:"reason"`
	Value      string `json:"value"`
	Title      string `json:"title"`
	AssignedTo string `json:"assigned_to"`
	Severity   string `json:"severity"`
	Version    int64  `json:"version"`
}

func workRoles(kind string) []string {
	switch kind {
	case "risk":
		return []string{"admin", "compliance"}
	case "incident":
		return []string{"admin", "platform"}
	default:
		return []string{"admin", "finance"}
	}
}
func controlRoles(kind string) []string {
	if kind == "product" || kind == "resume" {
		return []string{"admin", "platform"}
	}
	return []string{"admin"}
}
func validStaffRole(role string) bool {
	switch role {
	case "admin", "finance", "compliance", "support", "platform", "auditor":
		return true
	}
	return false
}
func (s *Service) RequireStaffElevation(ctx context.Context, p Principal) error {
	var ok bool
	e := s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sessions s JOIN users u ON u.id=s.user_id LEFT JOIN staff_elevations e ON e.session_id=s.id WHERE s.id=$1 AND s.user_id=$2 AND s.audience='staff' AND s.mfa_ready AND NOT s.revoked AND s.expires_at>$3 AND u.mfa_enabled AND u.status='active' AND u.role<>'customer' AND (s.created_at>$4 OR e.expires_at>$3))`, p.SessionID, p.User.ID, s.Now(), s.Now().Add(-10*time.Minute)).Scan(&ok)
	if e != nil {
		return e
	}
	if !ok {
		return &Fault{403, "staff_step_up_required", "Reauthenticate with your password and a fresh MFA code before this action"}
	}
	return nil
}
func (s *Service) StaffElevate(ctx context.Context, p Principal, password, code string) error {
	if e := s.Rate(ctx, "staff-step-up:"+p.User.ID, 6, 15*time.Minute); e != nil {
		return e
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var e error
		p, e = s.consolePrincipal(ctx, tx, p, "admin", "finance", "compliance", "support", "platform", "auditor")
		if e != nil {
			return e
		}
		if e = s.reauthenticate(ctx, tx, p.User, password, code); e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO staff_elevations(session_id,expires_at) VALUES($1,$2) ON CONFLICT(session_id) DO UPDATE SET expires_at=excluded.expires_at`, p.SessionID, s.Now().Add(10*time.Minute)); e != nil {
			return e
		}
		return s.audit(ctx, tx, p.User.ID, "staff.reauthenticated", p.SessionID, map[string]any{})
	})
}
func (s *Service) adminNote(ctx context.Context, tx *sql.Tx, p Principal, resource, target, body string) error {
	id := stringID("note_")
	sealed, e := s.Config.Box.Seal(body, "admin-note:"+id)
	if e != nil {
		return e
	}
	return exec(tx, ctx, `INSERT INTO admin_notes(id,resource,target,actor_id,body_enc) VALUES($1,$2,$3,$4,$5)`, id, resource, target, p.User.ID, sealed)
}
func (s *Service) AdminCommand(ctx context.Context, p Principal, in AdminCommandInput) (map[string]any, error) {
	out := map[string]any{}
	roles, ok := adminActionRoles[in.Action]
	if !ok {
		return nil, Invalid("unsupported admin command")
	}
	if !safeText(in.Reason, 1000) || len(strings.TrimSpace(in.Reason)) < 8 || len(in.Target) > 150 {
		return nil, Invalid("an explanation of 8 to 1000 characters and a bounded target are required")
	}
	if e := s.RequireStaffElevation(ctx, p); e != nil {
		return nil, e
	}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		var e error
		p, e = s.consolePrincipal(ctx, tx, p, roles...)
		if e != nil {
			return e
		}
		// Serialises cross-staff changes without arbitrary dynamic identifiers.
		if e = exec(tx, ctx, `SELECT pg_advisory_xact_lock(818426003)`); e != nil {
			return e
		}
		var result sql.Result
		switch in.Action {
		case "note":
			r, err := resourceSQL(in.Resource, p.User.Role)
			if err != nil {
				return err
			}
			if err = requireRole(p, r.Roles...); err != nil {
				return err
			}
			var found string
			if err = tx.QueryRowContext(ctx, `SELECT id FROM (`+r.SQL+`) t WHERE id=$1`, in.Target).Scan(&found); err != nil {
				return isMissing(err)
			}
			if err = s.adminNote(ctx, tx, p, in.Resource, in.Target, in.Reason); err != nil {
				return err
			}
		case "work_create":
			kind := in.Resource
			if kind != "risk" && kind != "incident" && kind != "exception" && kind != "refund" && kind != "return" {
				return Invalid("unsupported work type")
			}
			if e = requireRole(p, workRoles(kind)...); e != nil {
				return e
			}
			if !safeText(in.Title, 140) {
				return Invalid("work title required")
			}
			if in.Severity != "low" && in.Severity != "medium" && in.Severity != "high" && in.Severity != "critical" {
				return Invalid("invalid severity")
			}
			if kind == "refund" || kind == "return" {
				var id string
				if e = tx.QueryRowContext(ctx, `SELECT id FROM payments WHERE id=$1 AND status='succeeded'`, in.Target).Scan(&id); e != nil {
					return isMissing(e)
				}
			}
			if kind == "exception" {
				var id string
				if e = tx.QueryRowContext(ctx, `SELECT id FROM reconciliation_items WHERE id=$1 AND NOT matched`, in.Target).Scan(&id); e != nil {
					return isMissing(e)
				}
			}
			id := stringID("work_")
			out["id"] = id
			if e = exec(tx, ctx, `INSERT INTO admin_work_items(id,kind,subject,target,severity,created_by,due_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, kind, in.Title, in.Target, in.Severity, p.User.ID, s.Now().Add(24*time.Hour)); e != nil {
				return e
			}
			resource := "investigations"
			if kind == "risk" {
				resource = "risk"
			}
			if kind == "incident" {
				resource = "incidents"
			}
			if e = s.adminNote(ctx, tx, p, resource, id, in.Reason); e != nil {
				return e
			}
			out["financial_effect"] = false
		case "work_update":
			var kind, status string
			var version int64
			if e = tx.QueryRowContext(ctx, `SELECT kind,status,version FROM admin_work_items WHERE id=$1 FOR UPDATE`, in.Target).Scan(&kind, &status, &version); e != nil {
				return isMissing(e)
			}
			if e = requireRole(p, workRoles(kind)...); e != nil {
				return e
			}
			if version != in.Version || status == "resolved" {
				return conflict("work item changed or is resolved")
			}
			if in.Value != "investigating" && in.Value != "awaiting_evidence" && in.Value != "resolved" {
				return Invalid("invalid work status")
			}
			var assignee any
			if in.AssignedTo != "" {
				u, err := s.user(ctx, tx, in.AssignedTo, false)
				if err != nil {
					return err
				}
				if !u.MFA || requireRole(Principal{User: u, Audience: "staff", MFAReady: true}, workRoles(kind)...) != nil {
					return Invalid("assignee must be an active eligible staff member with MFA")
				}
				assignee = in.AssignedTo
			}
			result, e = tx.ExecContext(ctx, `UPDATE admin_work_items SET status=$2,assigned_to=$3,version=version+1,updated_at=$4 WHERE id=$1`, in.Target, in.Value, assignee, s.Now())
			if e == nil {
				resource := "investigations"
				if kind == "risk" {
					resource = "risk"
				}
				if kind == "incident" {
					resource = "incidents"
				}
				e = s.adminNote(ctx, tx, p, resource, in.Target, in.Reason)
			}
			out["financial_effect"] = false
		case "support_assign":
			var assignee any
			if in.AssignedTo != "" {
				u, err := s.user(ctx, tx, in.AssignedTo, false)
				if err != nil {
					return err
				}
				if !u.MFA || !caseStaff(Principal{User: u, Audience: "staff", MFAReady: true}) {
					return Invalid("assignee requires active support permissions and MFA")
				}
				assignee = in.AssignedTo
			}
			result, e = tx.ExecContext(ctx, `UPDATE support_cases SET assigned_to=$2,version=version+1,updated_at=$4 WHERE id=$1 AND version=$3 AND status<>'resolved'`, in.Target, assignee, in.Version, s.Now())
			if e == nil {
				e = s.adminNote(ctx, tx, p, "support", in.Target, in.Reason)
			}
		case "customer_sessions_revoke":
			var id string
			if e = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND role='customer' FOR UPDATE`, in.Target).Scan(&id); e != nil {
				return isMissing(e)
			}
			_, e = tx.ExecContext(ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1`, id)
		case "schedule_stop":
			result, e = tx.ExecContext(ctx, `UPDATE payment_mandates SET status='paused',version=version+1 WHERE id=$1 AND version=$2 AND status='active'`, in.Target, in.Version)
		case "notification_suppress":
			result, e = tx.ExecContext(ctx, `UPDATE notification_intents SET state='suppressed' WHERE id=$1 AND NOT secret AND state IN('queued','dead')`, in.Target)
		case "emergency_stop":
			result, e = tx.ExecContext(ctx, `UPDATE policies SET payments_enabled=false,version=version+1 WHERE id=1 AND version=$1`, in.Version)
			out["blast_radius"] = "New payment authorisation/acceptance and future schedule occurrences. Already accepted obligations continue recovery."
		case "control_propose":
			if e = requireRole(p, controlRoles(in.Resource)...); e != nil {
				return e
			}
			var version int64
			switch in.Resource {
			case "staff_status", "staff_role", "staff_recovery":
				u, err := s.user(ctx, tx, in.Target, false)
				if err != nil {
					return err
				}
				if u.Role == "customer" || u.Status == "closed" || u.ID == p.User.ID {
					return denied()
				}
				version = u.Version
				if in.Resource == "staff_recovery" {
					in.Value = "reset_credentials"
				}
				if in.Resource == "staff_status" && (in.Value != "active" && in.Value != "restricted") {
					return Invalid("invalid staff status")
				}
				if in.Resource == "staff_role" && !validStaffRole(in.Value) {
					return Invalid("invalid staff role")
				}
			case "product":
				if !safeText(in.Target, 150) || (in.Value != "enabled" && in.Value != "disabled") {
					return Invalid("invalid product control")
				}
				if e = exec(tx, ctx, `INSERT INTO product_controls(id) VALUES($1) ON CONFLICT DO NOTHING`, in.Target); e != nil {
					return e
				}
				e = tx.QueryRowContext(ctx, `SELECT version FROM product_controls WHERE id=$1`, in.Target).Scan(&version)
			case "resume":
				in.Target = "payment-policy"
				in.Value = "enabled"
				e = tx.QueryRowContext(ctx, `SELECT version FROM policies WHERE id=1 AND NOT payments_enabled`).Scan(&version)
			default:
				return Invalid("unsupported control kind")
			}
			if e != nil {
				return isMissing(e)
			}
			if in.Version != version {
				return conflict("target version changed; refresh before proposing")
			}
			id := stringID("control_")
			out["id"] = id
			e = exec(tx, ctx, `INSERT INTO admin_controls(id,kind,target,value,target_version,maker_id,reason,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, in.Resource, in.Target, in.Value, version, p.User.ID, in.Reason, s.Now().Add(24*time.Hour))
		case "control_decide":
			var kind, target, value, maker, status string
			var version int64
			var expires time.Time
			if e = tx.QueryRowContext(ctx, `SELECT kind,target,value,target_version,maker_id,status,expires_at FROM admin_controls WHERE id=$1 FOR UPDATE`, in.Target).Scan(&kind, &target, &value, &version, &maker, &status, &expires); e != nil {
				return isMissing(e)
			}
			if e = requireRole(p, controlRoles(kind)...); e != nil {
				return e
			}
			if maker == p.User.ID || target == p.User.ID {
				return &Fault{403, "self_approval_forbidden", "a different eligible operator, not the subject, must decide"}
			}
			if status != "pending" || !expires.After(s.Now()) {
				return conflict("control expired or already decided")
			}
			makerUser, err := s.user(ctx, tx, maker, false)
			if err != nil {
				return err
			}
			if !makerUser.MFA || requireRole(Principal{User: makerUser, Audience: "staff", MFAReady: true}, controlRoles(kind)...) != nil {
				return denied()
			}
			if in.Value != "approve" && in.Value != "reject" {
				return Invalid("approve or reject required")
			}
			state := "rejected"
			if in.Value == "approve" {
				state = "approved"
				switch kind {
				case "staff_status", "staff_role", "staff_recovery":
					u, err := s.user(ctx, tx, target, true)
					if err != nil {
						return err
					}
					if u.Version != version || u.Role == "customer" || u.Status == "closed" {
						return conflict("staff changed since proposal")
					}
					if u.Role == "admin" && (value == "restricted" || (kind == "staff_role" && value != "admin")) {
						var n int
						if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE role='admin' AND status='active' AND mfa_enabled AND id<>$1`, target).Scan(&n); err != nil {
							return err
						}
						if n < 1 {
							return conflict("cannot remove last active MFA administrator")
						}
					}
					if kind == "staff_recovery" {
						if e = exec(tx, ctx, `UPDATE staff_recovery_grants SET consumed_at=$2 WHERE user_id=$1 AND consumed_at IS NULL`, target, s.Now()); e != nil {
							return e
						}
						token := security.Random("", 32)
						e = exec(tx, ctx, `INSERT INTO staff_recovery_grants(id,control_id,user_id,target_version,token_hash,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, stringID("recovery_"), in.Target, target, version, security.Digest(token), s.Now().Add(time.Hour))
						out["recovery_token"] = token
						out["expires_at"] = s.Now().Add(time.Hour)
						out["delivery"] = "Display once; privately hand over after independent identity checks. No email delivery is claimed."
					} else if kind == "staff_status" {
						e = exec(tx, ctx, `UPDATE users SET status=$2,version=version+1 WHERE id=$1`, target, value)
					} else {
						e = exec(tx, ctx, `UPDATE users SET role=$2,version=version+1 WHERE id=$1`, target, value)
					}
					if e == nil {
						e = exec(tx, ctx, `UPDATE sessions SET revoked=true WHERE user_id=$1`, target)
					}
				case "product":
					result, e = tx.ExecContext(ctx, `UPDATE product_controls SET enabled=$2,version=version+1,updated_at=$4 WHERE id=$1 AND version=$3`, target, value == "enabled", version, s.Now())
				case "resume":
					result, e = tx.ExecContext(ctx, `UPDATE policies SET payments_enabled=true,version=version+1 WHERE id=1 AND version=$1 AND NOT payments_enabled`, version)
				}
			}
			if e == nil {
				e = exec(tx, ctx, `UPDATE admin_controls SET status=$2,checker_id=$3 WHERE id=$1`, in.Target, state, p.User.ID)
			}
		default:
			return Invalid("use the dedicated invitation endpoints")
		}
		if e != nil {
			return e
		}
		if result != nil {
			n, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if n != 1 {
				return conflict("record changed or action is not applicable")
			}
		}
		out["status"] = "recorded"
		return s.audit(ctx, tx, p.User.ID, "admin."+in.Action, in.Target, map[string]any{"reason": in.Reason, "resource": in.Resource, "value": in.Value})
	})
	return out, e
}

type StaffInvitationInput struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Reason string `json:"reason"`
}
type StaffInvitationAccept struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (s *Service) InviteStaff(ctx context.Context, p Principal, in StaffInvitationInput) (string, error) {
	email, e := normalEmail(in.Email)
	if e != nil {
		return "", e
	}
	if !validStaffRole(in.Role) || !safeText(in.Name, 80) || !safeText(in.Reason, 500) {
		return "", Invalid("valid invitation name, role and reason required")
	}
	if e = s.RequireStaffElevation(ctx, p); e != nil {
		return "", e
	}
	id := stringID("invite_")
	e = s.transact(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = s.consolePrincipal(ctx, tx, p, "admin")
		if err != nil {
			return err
		}
		if err = exec(tx, ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, email); err != nil {
			return err
		}
		var exists bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email=$1)`, email).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return conflict("email already registered")
		}
		if err = exec(tx, ctx, `UPDATE staff_invitations SET state='revoked' WHERE email=$1 AND state IN('proposed','approved') AND expires_at<=$2`, email, s.Now()); err != nil {
			return err
		}
		if err = exec(tx, ctx, `INSERT INTO staff_invitations(id,email,name,role,maker_id,reason,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, email, in.Name, in.Role, p.User.ID, in.Reason, s.Now().Add(48*time.Hour)); err != nil {
			return err
		}
		return s.audit(ctx, tx, p.User.ID, "staff.invitation_proposed", id, map[string]any{"role": in.Role})
	})
	return id, e
}
func (s *Service) DecideInvitation(ctx context.Context, p Principal, id, decision, reason string) (map[string]any, error) {
	if !safeText(reason, 500) || (decision != "approve" && decision != "reject" && decision != "revoke") {
		return nil, Invalid("decision and reason required")
	}
	if e := s.RequireStaffElevation(ctx, p); e != nil {
		return nil, e
	}
	out := map[string]any{}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = s.consolePrincipal(ctx, tx, p, "admin")
		if err != nil {
			return err
		}
		var maker, state string
		var expires time.Time
		if err = tx.QueryRowContext(ctx, `SELECT maker_id,state,expires_at FROM staff_invitations WHERE id=$1 FOR UPDATE`, id).Scan(&maker, &state, &expires); err != nil {
			return isMissing(err)
		}
		if state != "proposed" && state != "approved" {
			return conflict("invitation is no longer actionable")
		}
		next := "revoked"
		var checker any
		var tokenHash any
		if decision != "revoke" {
			if maker == p.User.ID {
				return &Fault{403, "self_approval_forbidden", "another administrator must review the invitation"}
			}
			if state != "proposed" || !expires.After(s.Now()) {
				return conflict("invitation expired or already reviewed")
			}
			checker = p.User.ID
			next = "rejected"
			makerUser, err := s.user(ctx, tx, maker, false)
			if err != nil {
				return err
			}
			if makerUser.Role != "admin" || makerUser.Status != "active" || !makerUser.MFA {
				return denied()
			}
			if decision == "approve" {
				next = "approved"
				token := security.Random("", 32)
				tokenHash = security.Digest(token)
				out["invitation_token"] = token
				out["expires_at"] = expires
				out["delivery"] = "Display once. Hand over privately to the intended recipient; no email delivery is claimed."
			}
		}
		if err = exec(tx, ctx, `UPDATE staff_invitations SET state=$2,checker_id=coalesce($3,checker_id),token_hash=$4 WHERE id=$1`, id, next, checker, tokenHash); err != nil {
			return err
		}
		out["status"] = next
		return s.audit(ctx, tx, p.User.ID, "staff.invitation_"+next, id, map[string]any{"reason": reason})
	})
	return out, e
}
func (s *Service) AcceptStaffInvitation(ctx context.Context, in StaffInvitationAccept) (string, error) {
	if len(in.Token) != 43 {
		return "", unauthorized()
	}
	hash, e := security.Password(in.Password, s.Config.Pepper)
	if e != nil {
		return "", Invalid(e.Error())
	}
	id := stringID("usr_")
	e = s.transact(ctx, func(tx *sql.Tx) error {
		var invite, email, name, role, maker, checker string
		e := tx.QueryRowContext(ctx, `SELECT id,email,name,role,maker_id,checker_id FROM staff_invitations WHERE token_hash=$1 AND state='approved' AND expires_at>$2 FOR UPDATE`, security.Digest(in.Token), s.Now()).Scan(&invite, &email, &name, &role, &maker, &checker)
		if errors.Is(e, sql.ErrNoRows) {
			return unauthorized()
		}
		if e != nil {
			return e
		}
		for _, actor := range []string{maker, checker} {
			u, e := s.user(ctx, tx, actor, false)
			if e != nil {
				return e
			}
			if u.Role != "admin" || u.Status != "active" || !u.MFA {
				return denied()
			}
		}
		// Possession of a manually delivered invitation is not a claim of automated mailbox verification.
		if e = exec(tx, ctx, `INSERT INTO users(id,email,name,password_hash,role) VALUES($1,$2,$3,$4,$5)`, id, email, name, hash, role); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE staff_invitations SET state='accepted',token_hash=NULL,accepted_user_id=$2 WHERE id=$1`, invite, id); e != nil {
			return e
		}
		return s.audit(ctx, tx, id, "staff.invitation_accepted", invite, map[string]any{"mfa_required": true})
	})
	return id, e
}

// BootstrapChecker is offline-only and closes permanently once a second staff record exists.
func (s *Service) BootstrapChecker(ctx context.Context, email, name, password string) (string, error) {
	email, e := normalEmail(email)
	if e != nil {
		return "", e
	}
	if !safeText(name, 80) {
		return "", Invalid("name required")
	}
	hash, e := security.Password(password, s.Config.Pepper)
	if e != nil {
		return "", e
	}
	id := stringID("usr_")
	e = s.transact(ctx, func(tx *sql.Tx) error {
		if e := exec(tx, ctx, `SELECT pg_advisory_xact_lock(818426002)`); e != nil {
			return e
		}
		var n int
		if e := tx.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE role<>'customer'`).Scan(&n); e != nil {
			return e
		}
		if n != 1 {
			return conflict("offline second-administrator bootstrap requires exactly one existing staff member")
		}
		if e := exec(tx, ctx, `INSERT INTO users(id,email,name,password_hash,role) VALUES($1,$2,$3,$4,'admin')`, id, email, name, hash); e != nil {
			return e
		}
		return s.audit(ctx, tx, "offline-bootstrap", "staff.checker_created", id, map[string]any{})
	})
	return id, e
}
