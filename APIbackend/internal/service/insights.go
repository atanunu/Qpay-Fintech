package service

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

var SpendingCategories = []string{"transfers", "airtime", "data", "electricity", "television", "internet", "groceries", "transport", "housing", "education", "health", "entertainment", "other"}

func validCategory(v string) bool {
	for _, c := range SpendingCategories {
		if c == v {
			return true
		}
	}
	return false
}
func MonthRange(month string) (time.Time, time.Time, error) {
	start, e := time.ParseInLocation("2006-01", month, lagosZone)
	if e != nil || start.Format("2006-01") != month {
		return time.Time{}, time.Time{}, Invalid("month must be YYYY-MM")
	}
	return start.UTC(), start.AddDate(0, 1, 0).UTC(), nil
}

type AnnotationInput struct {
	Category string `json:"category"`
	Note     string `json:"note"`
	Excluded bool   `json:"excluded"`
}

func (s *Service) Annotate(ctx context.Context, p Principal, id string, in AnnotationInput) error {
	if !validCategory(in.Category) || len(in.Note) > 1000 || strings.ContainsAny(in.Note, "\x00") {
		return Invalid("invalid category or note")
	}
	sealed, e := s.Config.Box.Seal(in.Note, "annotation:"+p.User.ID+":"+id)
	if e != nil {
		return e
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		if _, e := s.activePrincipal(ctx, tx, p); e != nil {
			return e
		}
		var found string
		if e := tx.QueryRowContext(ctx, `SELECT id FROM payments WHERE id=$1 AND (owner_id=$2 OR recipient_id=$2)`, id, p.User.ID).Scan(&found); e != nil {
			return isMissing(e)
		}
		return exec(tx, ctx, `INSERT INTO payment_annotations(owner_id,payment_id,category,note_enc,excluded) VALUES($1,$2,$3,$4,$5) ON CONFLICT(owner_id,payment_id) DO UPDATE SET category=excluded.category,note_enc=excluded.note_enc,excluded=excluded.excluded,updated_at=now()`, p.User.ID, id, in.Category, sealed, in.Excluded)
	})
}
func (s *Service) Annotation(ctx context.Context, p Principal, id string) (AnnotationInput, error) {
	if _, e := s.Payment(ctx, p.User.ID, id); e != nil {
		return AnnotationInput{}, e
	}
	var out AnnotationInput
	var sealed string
	e := s.DB.QueryRowContext(ctx, `SELECT category,note_enc,excluded FROM payment_annotations WHERE owner_id=$1 AND payment_id=$2`, p.User.ID, id).Scan(&out.Category, &sealed, &out.Excluded)
	if e == sql.ErrNoRows {
		return AnnotationInput{Category: "other"}, nil
	}
	if e != nil {
		return out, e
	}
	out.Note, e = s.Config.Box.Open(sealed, "annotation:"+p.User.ID+":"+id)
	return out, e
}

type BudgetInput struct {
	Month    string `json:"month"`
	Category string `json:"category"`
	Amount   Money  `json:"amount_minor"`
}

func (s *Service) SetBudget(ctx context.Context, p Principal, in BudgetInput) error {
	from, _, e := MonthRange(in.Month)
	if e != nil {
		return e
	}
	if !validCategory(in.Category) || in.Amount <= 0 || in.Amount > MaxMoney {
		return Invalid("valid category and positive budget required")
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		if _, e := s.activePrincipal(ctx, tx, p); e != nil {
			return e
		}
		return exec(tx, ctx, `INSERT INTO budgets(owner_id,month,category,amount) VALUES($1,$2,$3,$4) ON CONFLICT(owner_id,month,category) DO UPDATE SET amount=excluded.amount,updated_at=now()`, p.User.ID, from.In(lagosZone).Format("2006-01-02"), in.Category, int64(in.Amount))
	})
}
func (s *Service) RemoveBudget(ctx context.Context, p Principal, month, category string) error {
	if _, _, e := MonthRange(month); e != nil {
		return e
	}
	if !validCategory(category) {
		return Invalid("invalid category")
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		if _, e := s.activePrincipal(ctx, tx, p); e != nil {
			return e
		}
		return exec(tx, ctx, `DELETE FROM budgets WHERE owner_id=$1 AND month=$2 AND category=$3`, p.User.ID, month+"-01", category)
	})
}
func (s *Service) Insights(ctx context.Context, p Principal, month string) (map[string]any, error) {
	start, end, e := MonthRange(month)
	if e != nil {
		return nil, e
	}
	if start.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) || start.After(s.Now().AddDate(2, 0, 0)) {
		return nil, Invalid("unsupported month")
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	// Complete server-side aggregates, not just the first activity page. Credits are 'money in', never all labelled earned income.
	var incoming, outgoing, fees, spent, excluded string
	e = tx.QueryRowContext(ctx, `SELECT coalesce(sum(p.amount::numeric) FILTER(WHERE p.amount>0),0)::text,coalesce(-sum(p.amount::numeric) FILTER(WHERE p.amount<0),0)::text FROM postings p JOIN journals j ON j.id=p.journal_id JOIN accounts a ON a.id=p.account_id WHERE a.owner_id=$1 AND j.created_at>=$2 AND j.created_at<$3`, p.User.ID, start, end).Scan(&incoming, &outgoing)
	if e != nil {
		return nil, e
	}
	e = tx.QueryRowContext(ctx, `SELECT coalesce(sum(p.fee::numeric),0)::text,coalesce(sum(p.total::numeric) FILTER(WHERE NOT coalesce(a.excluded,false)),0)::text,coalesce(sum(p.total::numeric) FILTER(WHERE coalesce(a.excluded,false)),0)::text FROM payments p JOIN journals j ON j.reference=p.id LEFT JOIN payment_annotations a ON a.payment_id=p.id AND a.owner_id=$1 WHERE p.owner_id=$1 AND p.status='succeeded' AND j.created_at>=$2 AND j.created_at<$3`, p.User.ID, start, end).Scan(&fees, &spent, &excluded)
	if e != nil {
		return nil, e
	}
	rows, e := tx.QueryContext(ctx, `WITH spending AS (SELECT coalesce(a.category,CASE WHEN p.kind='bill' THEN 'other' ELSE 'transfers' END) AS category,sum(p.total::numeric) AS spent FROM payments p JOIN journals j ON j.reference=p.id LEFT JOIN payment_annotations a ON a.payment_id=p.id AND a.owner_id=$1 WHERE p.owner_id=$1 AND p.status='succeeded' AND NOT coalesce(a.excluded,false) AND j.created_at>=$2 AND j.created_at<$3 GROUP BY 1), plans AS (SELECT category,amount FROM budgets WHERE owner_id=$1 AND month=$4) SELECT coalesce(s.category,b.category),coalesce(s.spent,0)::text,coalesce(b.amount,0)::text FROM spending s FULL OUTER JOIN plans b ON b.category=s.category ORDER BY 1`, p.User.ID, start, end, month+"-01")
	if e != nil {
		return nil, e
	}
	categories := []map[string]any{}
	for rows.Next() {
		var name, amount, budget string
		if e = rows.Scan(&name, &amount, &budget); e != nil {
			rows.Close()
			return nil, e
		}
		categories = append(categories, map[string]any{"category": name, "spent_minor": amount, "budget_minor": budget})
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	var priorOut string
	e = tx.QueryRowContext(ctx, `SELECT coalesce(-sum(p.amount::numeric) FILTER(WHERE p.amount<0),0)::text FROM postings p JOIN journals j ON j.id=p.journal_id JOIN accounts a ON a.id=p.account_id WHERE a.owner_id=$1 AND j.created_at>=$2 AND j.created_at<$3`, p.User.ID, start.In(lagosZone).AddDate(0, -1, 0).UTC(), start).Scan(&priorOut)
	if e != nil {
		return nil, e
	}
	var upcoming string
	e = tx.QueryRowContext(ctx, `SELECT coalesce(sum(amount::numeric),0)::text FROM reminders WHERE owner_id=$1 AND status='active' AND due_at>=$2 AND due_at<$3`, p.User.ID, s.Now(), s.Now().AddDate(0, 0, 30)).Scan(&upcoming)
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return map[string]any{"month": month, "currency": "NGN", "timezone": "Africa/Lagos", "money_in_minor": incoming, "money_out_minor": outgoing, "spending_minor": spent, "excluded_minor": excluded, "fees_minor": fees, "previous_money_out_minor": priorOut, "next_30_days_reminders_minor": upcoming, "categories": categories, "generated_at": s.Now(), "estimate_note": "Upcoming reminder amounts are plans, not reserved funds. Money in includes funding and transfers, not only income."}, nil
}

// Search only returns the current customer's records and bounded pages, never another user's names or balances.
func (s *Service) Search(ctx context.Context, p Principal, query string) (map[string]any, error) {
	if len(query) < 2 || len(query) > 100 {
		return nil, Invalid("search must contain 2 to 100 characters")
	}
	if e := s.Rate(ctx, "search:"+p.User.ID, 60, time.Minute); e != nil {
		return nil, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT id,kind,status,amount,created_at FROM payments WHERE (owner_id=$1 OR recipient_id=$1) AND (strpos(lower(id),lower($2))>0 OR (owner_id=$1 AND strpos(lower(narration),lower($2))>0)) ORDER BY id DESC LIMIT 50`, p.User.ID, query)
	if e != nil {
		return nil, e
	}
	out := []map[string]any{}
	for rows.Next() {
		var id, kind, status string
		var amount Money
		var at time.Time
		if e = rows.Scan(&id, &kind, &status, &amount, &at); e != nil {
			rows.Close()
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "kind": kind, "status": status, "amount_minor": amount, "created_at": at})
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	return map[string]any{"payments": out, "limit": 50}, nil
}
