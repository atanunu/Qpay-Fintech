package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type SavedBillInput struct {
	Label      string `json:"label"`
	ProductID  string `json:"product_id"`
	CustomerID string `json:"customer_id"`
	Amount     Money  `json:"amount_minor"`
	Favourite  bool   `json:"favourite"`
}
type SavedBill struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	ProductID  string `json:"product_id"`
	CustomerID string `json:"customer_id"`
	Amount     Money  `json:"amount_minor"`
	Currency   string `json:"currency"`
	Favourite  bool   `json:"favourite"`
}

func (s *Service) SaveBill(ctx context.Context, p Principal, id string, in SavedBillInput) (string, error) {
	if e := p.Customer(); e != nil {
		return "", e
	}
	if !safeText(in.Label, 80) || !safeText(in.ProductID, 500) || !safeText(in.CustomerID, 100) || in.Amount <= 0 || in.Amount > MaxMoney {
		return "", Invalid("provide a label, product, customer identifier and positive amount")
	}
	creating := id == ""
	if creating {
		id = stringID("savedbill_")
	}
	sealed, e := s.Config.Box.Seal(in.CustomerID, "savedbill:"+id)
	if e != nil {
		return "", e
	}
	e = s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if u.Status != "active" {
			return denied()
		}
		if creating {
			var count int
			if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM bill_favourites WHERE owner_id=$1 AND active`, u.ID).Scan(&count); e != nil {
				return e
			}
			if count >= 100 {
				return Invalid("saved bill limit reached")
			}
			e = exec(tx, ctx, `INSERT INTO bill_favourites(id,owner_id,label,product_id,customer_enc,amount,currency,favourite) VALUES($1,$2,$3,$4,$5,$6,'NGN',$7)`, id, u.ID, in.Label, in.ProductID, sealed, int64(in.Amount), in.Favourite)
		} else {
			r, err := tx.ExecContext(ctx, `UPDATE bill_favourites SET label=$3,product_id=$4,customer_enc=$5,amount=$6,favourite=$7,updated_at=$8 WHERE id=$1 AND owner_id=$2 AND active`, id, u.ID, in.Label, in.ProductID, sealed, int64(in.Amount), in.Favourite, s.Now())
			if err != nil {
				return err
			}
			n, _ := r.RowsAffected()
			if n != 1 {
				return missing()
			}
		}
		if e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "bill.saved", id, map[string]any{})
	})
	return id, e
}
func (s *Service) SavedBills(ctx context.Context, p Principal) ([]SavedBill, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,label,product_id,customer_enc,amount,currency,favourite FROM bill_favourites WHERE owner_id=$1 AND active ORDER BY favourite DESC,created_at DESC LIMIT 100`, p.User.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []SavedBill{}
	for rows.Next() {
		var v SavedBill
		var sealed string
		if e = rows.Scan(&v.ID, &v.Label, &v.ProductID, &sealed, &v.Amount, &v.Currency, &v.Favourite); e != nil {
			return nil, e
		}
		v.CustomerID, e = s.Config.Box.Open(sealed, "savedbill:"+v.ID)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) DeleteSavedBill(ctx context.Context, p Principal, id string) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		if _, e := s.activePrincipal(ctx, tx, p); e != nil {
			return e
		}
		r, e := tx.ExecContext(ctx, `UPDATE bill_favourites SET active=false WHERE id=$1 AND owner_id=$2`, id, p.User.ID)
		if e != nil {
			return e
		}
		n, _ := r.RowsAffected()
		if n != 1 {
			return missing()
		}
		if e = exec(tx, ctx, `UPDATE reminders SET status='cancelled',version=version+1 WHERE bill_id=$1 AND owner_id=$2 AND status IN('active','paused')`, id, p.User.ID); e != nil {
			return e
		}
		return s.audit(ctx, tx, p.User.ID, "bill.removed", id, map[string]any{})
	})
}

type ReminderInput struct {
	Title   string     `json:"title"`
	Amount  Money      `json:"amount_minor"`
	BillID  string     `json:"bill_id,omitempty"`
	DueAt   time.Time  `json:"due_at"`
	EndsAt  *time.Time `json:"ends_at,omitempty"`
	Cadence string     `json:"cadence"`
}
type Reminder struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Amount  Money      `json:"amount_minor"`
	BillID  string     `json:"bill_id"`
	DueAt   time.Time  `json:"due_at"`
	EndsAt  *time.Time `json:"ends_at"`
	Cadence string     `json:"cadence"`
	Status  string     `json:"status"`
	Version int64      `json:"version"`
}

func (s *Service) CreateReminder(ctx context.Context, p Principal, in ReminderInput) (string, error) {
	if e := p.Customer(); e != nil {
		return "", e
	}
	if !safeText(in.Title, 140) || in.Amount < 0 || in.Amount > MaxMoney || !in.DueAt.After(s.Now()) || in.DueAt.After(s.Now().AddDate(2, 0, 0)) {
		return "", Invalid("provide a title and future reminder within two years")
	}
	if in.Cadence != "once" && in.Cadence != "weekly" && in.Cadence != "monthly" {
		return "", Invalid("invalid cadence")
	}
	if in.EndsAt != nil && (in.EndsAt.Before(in.DueAt) || in.EndsAt.After(s.Now().AddDate(5, 0, 0))) {
		return "", Invalid("invalid reminder end date")
	}
	id := stringID("reminder_")
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if u.Status != "active" {
			return denied()
		}
		var count int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM reminders WHERE owner_id=$1 AND status IN('active','paused')`, u.ID).Scan(&count); e != nil {
			return e
		}
		if count >= 100 {
			return Invalid("active reminder limit reached")
		}
		var bill any
		if in.BillID != "" {
			var found string
			if e = tx.QueryRowContext(ctx, `SELECT id FROM bill_favourites WHERE id=$1 AND owner_id=$2 AND active`, in.BillID, u.ID).Scan(&found); e != nil {
				return isMissing(e)
			}
			bill = found
		}
		if e = exec(tx, ctx, `INSERT INTO reminders(id,owner_id,title,amount,bill_id,due_at,anchor_day,cadence,ends_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, u.ID, in.Title, int64(in.Amount), bill, in.DueAt, in.DueAt.In(lagosZone).Day(), in.Cadence, in.EndsAt); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "reminder.created", id, map[string]any{"cadence": in.Cadence})
	})
	return id, e
}

var lagosZone = time.FixedZone("Africa/Lagos", 3600)

// NextDue preserves the original day of month, including January 31 -> February 28 -> March 31.
func NextDue(due time.Time, cadence string, anchor int) time.Time {
	local := due.In(lagosZone)
	if cadence == "weekly" {
		return local.AddDate(0, 0, 7).UTC()
	}
	if cadence != "monthly" {
		return time.Time{}
	}
	first := time.Date(local.Year(), local.Month()+1, 1, local.Hour(), local.Minute(), local.Second(), 0, lagosZone)
	last := first.AddDate(0, 1, -1).Day()
	if anchor > last {
		anchor = last
	}
	return first.AddDate(0, 0, anchor-1).UTC()
}
func (s *Service) Reminders(ctx context.Context, p Principal) ([]Reminder, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,title,amount,coalesce(bill_id,''),due_at,ends_at,cadence,status,version FROM reminders WHERE owner_id=$1 ORDER BY due_at,id LIMIT 200`, p.User.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Reminder{}
	for rows.Next() {
		var v Reminder
		if e = rows.Scan(&v.ID, &v.Title, &v.Amount, &v.BillID, &v.DueAt, &v.EndsAt, &v.Cadence, &v.Status, &v.Version); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) ReminderState(ctx context.Context, p Principal, id, state string, version int64) error {
	if state != "active" && state != "paused" && state != "cancelled" {
		return Invalid("unsupported reminder state")
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		if _, e := s.activePrincipal(ctx, tx, p); e != nil {
			return e
		}
		r, e := tx.ExecContext(ctx, `UPDATE reminders SET status=$3,version=version+1 WHERE id=$1 AND owner_id=$2 AND version=$4 AND status IN('active','paused')`, id, p.User.ID, state, version)
		if e != nil {
			return e
		}
		n, _ := r.RowsAffected()
		if n != 1 {
			return conflict("reminder changed or is no longer active")
		}
		return s.audit(ctx, tx, p.User.ID, "reminder."+state, id, map[string]any{})
	})
}
func (s *Service) WorkReminder(ctx context.Context) (bool, error) {
	worked := false
	e := s.transact(ctx, func(tx *sql.Tx) error {
		worked = false
		var id, owner, cadence string
		var due time.Time
		var ends sql.NullTime
		var anchor int
		e := tx.QueryRowContext(ctx, `SELECT r.id,r.owner_id,r.cadence,r.due_at,r.anchor_day,r.ends_at FROM reminders r JOIN users u ON u.id=r.owner_id WHERE r.status='active' AND r.due_at<=$1 AND u.status<>'closed' ORDER BY r.due_at FOR UPDATE OF r SKIP LOCKED LIMIT 1`, s.Now()).Scan(&id, &owner, &cadence, &due, &anchor, &ends)
		if errors.Is(e, sql.ErrNoRows) {
			return nil
		}
		if e != nil {
			return e
		}
		worked = true
		occ := stringID("remocc_")
		if e = exec(tx, ctx, `INSERT INTO reminder_occurrences(id,reminder_id,due_at,status) VALUES($1,$2,$3,'notified')`, occ, id, due); e != nil {
			return e
		}
		if e = s.notify(ctx, tx, owner, "planner-reminder-due", occ, nil, false); e != nil {
			return e
		}
		next := NextDue(due, cadence, anchor)
		for !next.IsZero() && !next.After(s.Now()) {
			next = NextDue(next, cadence, anchor)
		}
		state := "active"
		if next.IsZero() || (ends.Valid && next.After(ends.Time)) {
			state = "completed"
			next = due
		}
		return exec(tx, ctx, `UPDATE reminders SET due_at=$2,status=$3,version=version+1 WHERE id=$1`, id, next, state)
	})
	return worked, e
}
func (s *Service) WatchBiller(ctx context.Context, p Principal, product string, enabled bool) error {
	if !safeText(product, 500) {
		return Invalid("invalid product")
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if enabled {
			var n int
			if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM service_subscriptions WHERE owner_id=$1`, u.ID).Scan(&n); e != nil {
				return e
			}
			if n >= 100 {
				return Invalid("subscription limit reached")
			}
			return exec(tx, ctx, `INSERT INTO service_subscriptions(owner_id,product_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, u.ID, product)
		}
		return exec(tx, ctx, `DELETE FROM service_subscriptions WHERE owner_id=$1 AND product_id=$2`, u.ID, product)
	})
}
func (s *Service) BillerHealth(ctx context.Context, p Principal) ([]map[string]any, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT a.product_id,CASE WHEN a.observed_at<$2 THEN 'unknown' ELSE a.state END,a.observed_at,EXISTS(SELECT 1 FROM service_subscriptions s WHERE s.owner_id=$1 AND s.product_id=a.product_id) FROM service_availability a ORDER BY a.product_id LIMIT 1000`, p.User.ID, s.Now().Add(-5*time.Minute))
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, state string
		var at time.Time
		var watching bool
		if e = rows.Scan(&id, &state, &at, &watching); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"product_id": id, "state": state, "observed_at": at, "watching": watching})
	}
	return out, rows.Err()
}
func (s *Service) ObserveBiller(ctx context.Context, p Principal, product, state string) error {
	if e := requireRole(p, "admin", "platform"); e != nil {
		return e
	}
	if !safeText(product, 500) || (state != "available" && state != "unavailable" && state != "unknown") {
		return Invalid("invalid availability observation")
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		p.User = u
		if e = requireRole(p, "admin", "platform"); e != nil {
			return e
		}
		if e = exec(tx, ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "biller:"+product); e != nil {
			return e
		}
		var previous string
		e = tx.QueryRowContext(ctx, `SELECT state FROM service_availability WHERE product_id=$1 FOR UPDATE`, product).Scan(&previous)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO service_availability(product_id,state,observed_at) VALUES($1,$2,$3) ON CONFLICT(product_id) DO UPDATE SET state=excluded.state,observed_at=excluded.observed_at,version=service_availability.version+1`, product, state, s.Now()); e != nil {
			return e
		}
		if previous == "unavailable" && state == "available" {
			rows, e := tx.QueryContext(ctx, `SELECT owner_id FROM service_subscriptions WHERE product_id=$1`, product)
			if e != nil {
				return e
			}
			owners := []string{}
			for rows.Next() {
				var owner string
				if e = rows.Scan(&owner); e != nil {
					rows.Close()
					return e
				}
				owners = append(owners, owner)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return e
			}
			for _, owner := range owners {
				if e = s.notify(ctx, tx, owner, "bills-service-restored", stringID("biller_"), nil, false); e != nil {
					return e
				}
			}
		}
		return s.audit(ctx, tx, u.ID, "biller.availability_observed", product, map[string]any{"state": state})
	})
}
func (s *Service) TokenArchive(ctx context.Context, p Principal, before string) ([]map[string]any, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,amount,currency,created_at,fulfilment_status FROM payments WHERE owner_id=$1 AND kind='bill' AND status='succeeded' AND ($2='' OR id<$2) ORDER BY id DESC LIMIT 50`, p.User.ID, before)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, currency, status string
		var amount Money
		var at time.Time
		if e = rows.Scan(&id, &amount, &currency, &at, &status); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "amount_minor": amount, "currency": currency, "created_at": at, "fulfilment_status": status})
	}
	return out, rows.Err()
}

// PaymentDraft provides owned historical details as a new editable draft. No authorisation or price is reused.
func (s *Service) PaymentDraft(ctx context.Context, p Principal, id string) (map[string]any, error) {
	var kind, sealed, narration string
	var amount Money
	e := s.DB.QueryRowContext(ctx, `SELECT kind,destination_enc,narration,amount FROM payments WHERE id=$1 AND owner_id=$2`, id, p.User.ID).Scan(&kind, &sealed, &narration, &amount)
	if e != nil {
		return nil, isMissing(e)
	}
	raw, e := s.Config.Box.Open(sealed, "payment:"+id)
	if e != nil {
		return nil, e
	}
	var d Destination
	if e = json.Unmarshal([]byte(raw), &d); e != nil {
		return nil, e
	}
	return map[string]any{"kind": kind, "destination": d, "amount_minor": amount, "narration": narration, "requires_new_quote": true}, nil
}
