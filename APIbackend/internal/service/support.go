package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

type CaseInput struct {
	Kind      string `json:"kind"`
	Subject   string `json:"subject"`
	Message   string `json:"message"`
	PaymentID string `json:"payment_id,omitempty"`
	IssueType string `json:"issue_type,omitempty"`
}

func (s *Service) CaseCreate(ctx context.Context, p Principal, in CaseInput) (string, error) {
	if e := p.Customer(); e != nil {
		return "", e
	}
	if in.Kind != "support" && in.Kind != "complaint" && in.Kind != "dispute" {
		return "", Invalid("invalid case kind")
	}
	if !safeText(in.Subject, 140) || !safeText(in.Message, 4000) {
		return "", Invalid("case subject and bounded message required")
	}
	if e := s.Rate(ctx, "case:"+p.User.ID, 10, time.Hour); e != nil {
		return "", e
	}
	if in.IssueType == "" {
		in.IssueType = "general"
	}
	switch in.IssueType {
	case "general", "money-not-received", "wrong-recipient", "duplicate-debit", "missing-token", "unfulfilled-bill", "suspicious-activity":
	default:
		return "", Invalid("unsupported issue type")
	}
	id := stringID("case_")
	e := s.transact(ctx, func(tx *sql.Tx) error {
		if _, e := s.activePrincipal(ctx, tx, p); e != nil {
			return e
		}
		var payment any
		if in.PaymentID != "" {
			var found string
			if e := tx.QueryRowContext(ctx, `SELECT id FROM payments WHERE id=$1 AND (owner_id=$2 OR recipient_id=$2)`, in.PaymentID, p.User.ID).Scan(&found); e != nil {
				return isMissing(e)
			}
			payment = found
		}
		if in.Kind == "dispute" && payment == nil {
			return Invalid("dispute must reference your payment")
		}
		if e := exec(tx, ctx, `INSERT INTO support_cases(id,owner_id,payment_id,kind,subject) VALUES($1,$2,$3,$4,$5)`, id, p.User.ID, payment, in.Kind, in.Subject); e != nil {
			return e
		}
		if e := exec(tx, ctx, `UPDATE support_cases SET issue_type=$2,response_due_at=$3 WHERE id=$1`, id, in.IssueType, s.Now().Add(24*time.Hour)); e != nil {
			return e
		}
		if e := exec(tx, ctx, `INSERT INTO case_events(id,case_id,actor_id,event) VALUES($1,$2,$3,'case_opened')`, stringID("caseevt_"), id, p.User.ID); e != nil {
			return e
		}
		if e := s.appendMessage(ctx, tx, id, p.User.ID, in.Message); e != nil {
			return e
		}
		return s.notify(ctx, tx, p.User.ID, "support-case-opened", id, nil, false)
	})
	return id, e
}
func (s *Service) appendMessage(ctx context.Context, tx *sql.Tx, caseID, author, body string) error {
	id := stringID("msg_")
	sealed, e := s.Config.Box.Seal(body, "case-message:"+id)
	if e != nil {
		return e
	}
	return exec(tx, ctx, `INSERT INTO case_messages(id,case_id,author_id,body) VALUES($1,$2,$3,$4)`, id, caseID, author, sealed)
}
func caseStaff(p Principal) bool {
	return p.Audience == "staff" && p.MFAReady && p.User.Status == "active" && (p.User.Role == "admin" || p.User.Role == "support")
}
func (s *Service) Cases(ctx context.Context, p Principal) ([]map[string]any, error) {
	staff := caseStaff(p)
	if p.Audience == "staff" && !staff {
		return nil, denied()
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,owner_id,coalesce(payment_id,''),kind,subject,status,created_at,updated_at FROM support_cases WHERE ($1 OR owner_id=$2) ORDER BY created_at DESC LIMIT 100`, staff, p.User.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, owner, payment, kind, subject, status string
		var created, updated time.Time
		if e = rows.Scan(&id, &owner, &payment, &kind, &subject, &status, &created, &updated); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "owner_id": owner, "payment_id": payment, "kind": kind, "subject": subject, "status": status, "created_at": created, "updated_at": updated})
	}
	return out, rows.Err()
}
func (s *Service) CaseMessages(ctx context.Context, p Principal, id string) ([]map[string]any, error) {
	staff := caseStaff(p)
	if p.Audience == "staff" && !staff {
		return nil, denied()
	}
	var found string
	if e := s.DB.QueryRowContext(ctx, `SELECT id FROM support_cases WHERE id=$1 AND ($2 OR owner_id=$3)`, id, staff, p.User.ID).Scan(&found); e != nil {
		return nil, isMissing(e)
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,author_id,body,created_at FROM case_messages WHERE case_id=$1 ORDER BY id LIMIT 1001`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var mid, author, sealed string
		var at time.Time
		if e = rows.Scan(&mid, &author, &sealed, &at); e != nil {
			return nil, e
		}
		if len(out) >= 1000 {
			return nil, Invalid("case exceeds current export bound; request an authorised support export")
		}
		body, e := s.Config.Box.Open(sealed, "case-message:"+mid)
		if e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": mid, "author_id": author, "message": body, "created_at": at})
	}
	return out, rows.Err()
}
func (s *Service) CaseReply(ctx context.Context, p Principal, id, message, status string) error {
	staff := caseStaff(p)
	if p.Audience == "staff" && !staff {
		return denied()
	}
	if !safeText(message, 4000) {
		return Invalid("bounded message required")
	}
	if status != "" && (!staff || (status != "open" && status != "awaiting_customer" && status != "resolved" && status != "escalated")) {
		return denied()
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		p.User = u
		staff = caseStaff(p)
		var owner string
		if e = tx.QueryRowContext(ctx, `SELECT owner_id FROM support_cases WHERE id=$1 AND ($2 OR owner_id=$3)`, id, staff, p.User.ID).Scan(&owner); e != nil {
			return isMissing(e)
		}
		customer, e := s.user(ctx, tx, owner, true)
		if e != nil {
			return e
		}
		if customer.Status == "closed" {
			return conflict("closed-account case requires the controlled retention process")
		}
		if e = tx.QueryRowContext(ctx, `SELECT owner_id FROM support_cases WHERE id=$1 AND ($2 OR owner_id=$3) FOR UPDATE`, id, staff, p.User.ID).Scan(&owner); e != nil {
			return isMissing(e)
		}
		if e = s.appendMessage(ctx, tx, id, p.User.ID, message); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE support_cases SET status=CASE WHEN $2='' THEN status ELSE $2 END,version=version+1,updated_at=$3 WHERE id=$1`, id, status, s.Now()); e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO case_events(id,case_id,actor_id,event) VALUES($1,$2,$3,$4)`, stringID("caseevt_"), id, p.User.ID, "reply_"+status); e != nil {
			return e
		}
		if staff {
			if e = s.notifyOccurrence(ctx, tx, owner, "support-reply-received", id, security.Random("reply_", 18), nil, false); e != nil {
				return e
			}
		}
		return s.audit(ctx, tx, p.User.ID, "support.reply", id, map[string]any{"new_status": status})
	})
}
