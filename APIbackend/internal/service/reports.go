package service

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type LedgerEntry struct {
	ID        int64     `json:"id"`
	JournalID string    `json:"journal_id"`
	Reference string    `json:"reference"`
	Kind      string    `json:"kind"`
	Delta     string    `json:"delta_minor"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) Entries(ctx context.Context, owner string, limit int, before int64) ([]LedgerEntry, error) {
	if limit < 1 || limit > 100 {
		return nil, Invalid("limit must be 1 to 100")
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT p.id,j.id,j.reference,j.kind,p.amount,j.created_at FROM postings p JOIN journals j ON j.id=p.journal_id JOIN accounts a ON a.id=p.account_id WHERE a.owner_id=$1 AND ($2::bigint=0 OR p.id<$2) ORDER BY p.id DESC LIMIT $3`, owner, before, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []LedgerEntry{}
	for rows.Next() {
		var entry LedgerEntry
		var amount int64
		if e = rows.Scan(&entry.ID, &entry.JournalID, &entry.Reference, &entry.Kind, &amount, &entry.CreatedAt); e != nil {
			return nil, e
		}
		entry.Delta = strconv.FormatInt(amount, 10)
		out = append(out, entry)
	}
	return out, rows.Err()
}

// Statement uses a repeatable-read snapshot and refuses truncation; narrow the
// time range rather than silently returning an incomplete financial statement.
func (s *Service) Statement(ctx context.Context, owner string, from, to time.Time) (map[string]any, error) {
	if from.IsZero() || !to.After(from) || to.Sub(from) > 366*24*time.Hour || to.After(s.Now().Add(time.Minute)) {
		return nil, Invalid("statement range must be positive, at most 366 days, and not in the future")
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	var opening string
	e = tx.QueryRowContext(ctx, `SELECT coalesce(sum(p.amount::numeric),0)::text FROM postings p JOIN journals j ON j.id=p.journal_id JOIN accounts a ON a.id=p.account_id WHERE a.owner_id=$1 AND j.created_at<$2`, owner, from).Scan(&opening)
	if e != nil {
		return nil, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT p.id,j.id,j.reference,j.kind,p.amount,j.created_at FROM postings p JOIN journals j ON j.id=p.journal_id JOIN accounts a ON a.id=p.account_id WHERE a.owner_id=$1 AND j.created_at>=$2 AND j.created_at<$3 ORDER BY j.created_at,p.id LIMIT 10001`, owner, from, to)
	if e != nil {
		return nil, e
	}
	out := []LedgerEntry{}
	closing, e := strconv.ParseInt(opening, 10, 64)
	if e != nil {
		rows.Close()
		return nil, e
	}
	for rows.Next() {
		var v LedgerEntry
		var delta int64
		if e = rows.Scan(&v.ID, &v.JournalID, &v.Reference, &v.Kind, &delta, &v.CreatedAt); e != nil {
			rows.Close()
			return nil, e
		}
		v.Delta = strconv.FormatInt(delta, 10)
		closing += delta
		out = append(out, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	if len(out) > 10000 {
		return nil, &Fault{422, "range_too_large", "narrow the range; no partial statement was produced"}
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return map[string]any{"currency": "NGN", "from": from, "to_exclusive": to, "opening_minor": opening, "closing_minor": strconv.FormatInt(closing, 10), "entries": out, "generated_at": s.Now()}, nil
}
func StatementCSV(data map[string]any) (string, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	if e := w.Write([]string{"date_utc", "journal_id", "reference", "kind", "delta_minor", "currency"}); e != nil {
		return "", e
	}
	for _, v := range data["entries"].([]LedgerEntry) {
		cells := []string{v.CreatedAt.UTC().Format(time.RFC3339Nano), v.JournalID, v.Reference, v.Kind, v.Delta, "NGN"}
		for i := 0; i < 4; i++ {
			if strings.HasPrefix(cells[i], "=") || strings.HasPrefix(cells[i], "+") || strings.HasPrefix(cells[i], "-") || strings.HasPrefix(cells[i], "@") || strings.HasPrefix(cells[i], "\t") {
				cells[i] = "'" + cells[i]
			}
		}
		if e := w.Write(cells); e != nil {
			return "", e
		}
	}
	w.Flush()
	return b.String(), w.Error()
}
func (s *Service) Receipt(ctx context.Context, owner, id string) (map[string]any, error) {
	p, e := s.Payment(ctx, owner, id)
	if e != nil {
		return nil, e
	}
	if p.Status != "succeeded" {
		return nil, conflict("a success receipt is available only for a confirmed successful payment")
	}
	return map[string]any{"payment": p, "generated_at": s.Now(), "copy": true, "environment": s.Config.Environment}, nil
}
func (s *Service) FundingHistory(ctx context.Context, owner string) ([]map[string]any, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,amount,currency,created_at FROM funding WHERE owner_id=$1 ORDER BY id DESC LIMIT 100`, owner)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, currency string
		var amount Money
		var t time.Time
		if e = rows.Scan(&id, &amount, &currency, &t); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "amount_minor": amount, "currency": currency, "created_at": t})
	}
	return out, rows.Err()
}

type ReconciliationInput struct {
	Source  string                `json:"source"`
	Entries []ReconciliationEntry `json:"entries"`
}
type ReconciliationEntry struct {
	Reference string `json:"reference"`
	Amount    Money  `json:"amount_minor"`
	Currency  string `json:"currency"`
	Status    string `json:"status"`
}

func (s *Service) Reconcile(ctx context.Context, p Principal, in ReconciliationInput) (map[string]any, error) {
	if e := requireRole(p, "admin", "finance"); e != nil {
		return nil, e
	}
	if in.Source != "provider" && in.Source != "bank" {
		return nil, Invalid("invalid source")
	}
	if len(in.Entries) == 0 || len(in.Entries) > 2000 {
		return nil, Invalid("provide 1 to 2000 records")
	}
	seen := map[string]bool{}
	for _, v := range in.Entries {
		if !validID(v.Reference) || v.Amount <= 0 || v.Currency != "NGN" || !safeText(v.Status, 40) || seen[v.Reference] {
			return nil, Invalid("invalid or duplicate reconciliation record")
		}
		seen[v.Reference] = true
	}
	id := stringID("recon_")
	results := []map[string]any{}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		p.User = u
		if e = requireRole(p, "admin", "finance"); e != nil {
			return e
		}
		results = nil
		if e = exec(tx, ctx, `INSERT INTO reconciliation_runs(id,actor_id,source) VALUES($1,$2,$3)`, id, u.ID, in.Source); e != nil {
			return e
		}
		for i, v := range in.Entries {
			var amount Money
			var currency, state string
			var journalCount int
			e = tx.QueryRowContext(ctx, `SELECT p.amount,p.currency,p.status,(SELECT count(*) FROM journals j WHERE j.reference=p.id) FROM payments p WHERE p.id=$1`, v.Reference).Scan(&amount, &currency, &state, &journalCount)
			reason := "matched"
			if e == sql.ErrNoRows {
				reason = "unknown_reference"
			} else if e != nil {
				return e
			} else if amount != v.Amount || currency != v.Currency {
				reason = "amount_or_currency_mismatch"
			} else if state != v.Status {
				reason = "state_mismatch"
			} else if state == "succeeded" && journalCount != 1 {
				reason = "ledger_posting_missing"
			} else if state != "succeeded" {
				reason = "not_final_success"
			}
			if e = exec(tx, ctx, `INSERT INTO reconciliation_items(id,run_id,reference,amount,currency,status,matched,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, fmt.Sprintf("%s_%d", id, i), id, v.Reference, int64(v.Amount), v.Currency, v.Status, reason == "matched", reason); e != nil {
				return e
			}
			results = append(results, map[string]any{"reference": v.Reference, "matched": reason == "matched", "reason": reason})
		}
		return s.audit(ctx, tx, u.ID, "reconciliation.imported", id, map[string]any{"source": in.Source, "count": len(in.Entries)})
	})
	return map[string]any{"id": id, "results": results, "scope": "imported rows only; not proof of complete settlement"}, e
}
func (s *Service) Reconciliations(ctx context.Context, p Principal) ([]map[string]any, error) {
	if e := requireRole(p, "admin", "finance"); e != nil {
		return nil, e
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT r.id,r.source,r.created_at,count(i.id),count(i.id) FILTER(WHERE NOT i.matched) FROM reconciliation_runs r LEFT JOIN reconciliation_items i ON i.run_id=r.id GROUP BY r.id ORDER BY r.created_at DESC LIMIT 100`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, source string
		var at time.Time
		var total, breaks int
		if e = rows.Scan(&id, &source, &at, &total, &breaks); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "source": source, "created_at": at, "rows": total, "exceptions": breaks})
	}
	return out, rows.Err()
}
func (s *Service) PaymentAdmin(ctx context.Context, p Principal, id string) (map[string]any, error) {
	if e := requireRole(p, "admin", "finance", "support", "compliance"); e != nil {
		return nil, e
	}
	v, e := scanPayment(s.DB.QueryRowContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE id=$1`, id))
	if e != nil {
		return nil, e
	}
	var observations json.RawMessage
	e = s.DB.QueryRowContext(ctx, `SELECT coalesce(jsonb_agg(jsonb_build_object('status',status,'created_at',created_at,'amount_minor',amount::text,'currency',currency) ORDER BY created_at),'[]'::jsonb) FROM observations WHERE payment_id=$1`, id).Scan(&observations)
	if e != nil {
		return nil, e
	}
	return map[string]any{"payment": v, "observations": observations}, nil
}
