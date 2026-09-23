package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

func (s *Service) Balance(ctx context.Context, owner string) (Balance, error) {
	b := Balance{Currency: "NGN"}
	e := s.DB.QueryRowContext(ctx, `SELECT balance,reserved,balance-reserved FROM accounts WHERE owner_id=$1`, owner).Scan(&b.Book, &b.Held, &b.Available)
	return b, isMissing(e)
}
func lockAccounts(ctx context.Context, tx *sql.Tx, ids ...string) error {
	sort.Strings(ids)
	last := ""
	for _, id := range ids {
		if id == last {
			continue
		}
		last = id
		var found string
		if e := tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE id=$1 FOR UPDATE`, id).Scan(&found); e != nil {
			return isMissing(e)
		}
	}
	return nil
}
func lockUsers(ctx context.Context, tx *sql.Tx, ids ...string) error {
	sort.Strings(ids)
	last := ""
	for _, id := range ids {
		if id == last {
			continue
		}
		last = id
		var found string
		if e := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&found); e != nil {
			return isMissing(e)
		}
	}
	return nil
}
func (s *Service) postJournal(ctx context.Context, tx *sql.Tx, reference, kind string, entries map[string]Money) (string, error) {
	keys := []string{}
	var sum Money
	for account, amount := range entries {
		if amount == 0 {
			continue
		}
		sum += amount
		keys = append(keys, account)
	}
	if len(keys) < 2 || sum != 0 {
		return "", errors.New("unbalanced journal request")
	}
	if e := lockAccounts(ctx, tx, keys...); e != nil {
		return "", e
	}
	id := security.Random("jnl_", 18)
	if e := exec(tx, ctx, `INSERT INTO journals(id,reference,kind,currency) VALUES($1,$2,$3,'NGN')`, id, reference, kind); e != nil {
		return "", e
	}
	sort.Strings(keys)
	for _, key := range keys {
		if e := exec(tx, ctx, `INSERT INTO postings(journal_id,account_id,amount) VALUES($1,$2,$3)`, id, key, int64(entries[key])); e != nil {
			return "", e
		}
	}
	return id, nil
}
func (s *Service) Banks(ctx context.Context) ([]Bank, error) {
	if !s.Config.ExternalEnabled || s.Config.Gateway == nil {
		return nil, unavailable()
	}
	v, e := s.Config.Gateway.Banks(ctx)
	return v, ExternalError(e)
}
func (s *Service) Products(ctx context.Context) ([]Product, error) {
	if !s.Config.ExternalEnabled || s.Config.Gateway == nil {
		return nil, unavailable()
	}
	v, e := s.Config.Gateway.Products(ctx)
	return v, ExternalError(e)
}
func (s *Service) Enquire(ctx context.Context, p Principal, d Destination, amount Money, kind string) (map[string]any, error) {
	if e := p.Customer(); e != nil {
		return nil, e
	}
	if e := s.eligible(p.User); e != nil {
		return nil, e
	}
	if !s.Config.ExternalEnabled || s.Config.Gateway == nil {
		return nil, unavailable()
	}
	if e := s.Rate(ctx, "enquiry:"+p.User.ID, 20, time.Hour); e != nil {
		return nil, e
	}
	var result Enquiry
	var e error
	if kind == "bank" {
		if len(d.AccountNumber) != 10 || strings.Trim(d.AccountNumber, "0123456789") != "" || !safeText(d.BankCode, 16) {
			return nil, Invalid("valid bank code and ten-digit account required")
		}
		d = Destination{BankCode: d.BankCode, AccountNumber: d.AccountNumber}
		result, e = s.Config.Gateway.Enquire(ctx, d)
	} else if kind == "bill" {
		if !safeText(d.ProductID, 100) || !safeText(d.CustomerID, 100) || amount <= 0 || amount > MaxMoney {
			return nil, Invalid("valid bill product, customer and amount required")
		}
		d = Destination{ProductID: d.ProductID, CustomerID: d.CustomerID}
		result, e = s.Config.Gateway.ValidateBill(ctx, d, amount)
	} else {
		return nil, Invalid("unsupported enquiry")
	}
	if e != nil {
		return nil, ExternalError(e)
	}
	if !safeText(result.Name, 150) || !safeText(result.Reference, 150) {
		return nil, unavailable()
	}
	d.AccountName = result.Name
	d.ValidationID = result.Reference
	id := security.Random("enq_", 18)
	sealed, e := s.Config.Box.Seal(jsonText(d), "enquiry:"+id)
	if e != nil {
		return nil, e
	}
	expires := s.Now().Add(5 * time.Minute)
	e = s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = s.eligible(u); e != nil {
			return e
		}
		return exec(tx, ctx, `INSERT INTO enquiries(id,owner_id,kind,destination_enc,amount,name,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, u.ID, kind, sealed, int64(amount), result.Name, expires)
	})
	if e != nil {
		return nil, e
	}
	return map[string]any{"id": id, "name": result.Name, "expires_at": expires, "amount_minor": amount}, nil
}
func (s *Service) BeneficiaryCreate(ctx context.Context, p Principal, enquiry, label string) (string, error) {
	if e := p.Customer(); e != nil {
		return "", e
	}
	if !safeText(label, 80) {
		return "", Invalid("invalid beneficiary label")
	}
	id := security.Random("ben_", 18)
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = s.eligible(u); e != nil {
			return e
		}
		var found string
		e = tx.QueryRowContext(ctx, `SELECT id FROM enquiries WHERE id=$1 AND owner_id=$2 AND kind='bank' AND expires_at>$3`, enquiry, u.ID, s.Now()).Scan(&found)
		if e != nil {
			return isMissing(e)
		}
		if e = exec(tx, ctx, `INSERT INTO beneficiaries(id,owner_id,enquiry_id,label) VALUES($1,$2,$3,$4)`, id, u.ID, enquiry, label); e != nil {
			return e
		}
		if e = s.notify(ctx, tx, u.ID, "transfer-beneficiary-added", id, nil, false); e != nil {
			return e
		}
		return s.audit(ctx, tx, u.ID, "beneficiary.created", id, map[string]any{})
	})
	return id, e
}
func (s *Service) Beneficiaries(ctx context.Context, owner string) ([]map[string]any, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT b.id,b.label,e.name,b.favourite FROM beneficiaries b JOIN enquiries e ON e.id=b.enquiry_id WHERE b.owner_id=$1 AND b.active ORDER BY b.favourite DESC,b.created_at DESC LIMIT 100`, owner)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, label, name string
		var favourite bool
		if e = rows.Scan(&id, &label, &name, &favourite); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "label": label, "account_name": name, "favourite": favourite})
	}
	return out, rows.Err()
}
func (s *Service) BeneficiaryDelete(ctx context.Context, p Principal, id string) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		r, e := tx.ExecContext(ctx, `UPDATE beneficiaries SET active=false WHERE id=$1 AND owner_id=$2`, id, p.User.ID)
		if e != nil {
			return e
		}
		n, _ := r.RowsAffected()
		if n == 0 {
			return missing()
		}
		return s.audit(ctx, tx, p.User.ID, "beneficiary.deleted", id, map[string]any{})
	})
}
func (s *Service) CreateQuote(ctx context.Context, p Principal, in QuoteInput) (Quote, error) {
	var q Quote
	if e := p.Customer(); e != nil {
		return q, e
	}
	if e := s.eligible(p.User); e != nil {
		return q, e
	}
	if in.Amount <= 0 || in.Amount > MaxMoney || in.Currency != "NGN" || !safeText(in.Narration, 100) {
		return q, Invalid("valid NGN amount and narration required")
	}
	if e := s.Rate(ctx, "quote:"+p.User.ID, 60, time.Hour); e != nil {
		return q, e
	}
	var enabled bool
	var cap, fee Money
	var version int64
	e := s.DB.QueryRowContext(ctx, `SELECT version,per_payment,internal_fee,payments_enabled FROM policies WHERE id=1`).Scan(&version, &cap, &fee, &enabled)
	if e != nil {
		return q, e
	}
	if !enabled {
		return q, &Fault{503, "payments_disabled", "payments are currently disabled"}
	}
	if in.Amount > cap {
		return q, Invalid("amount exceeds current policy")
	}
	d := Destination{}
	switch in.Kind {
	case "internal":
		if in.EnquiryID != "" || in.RecipientID == p.User.ID || !validID(in.RecipientID) || in.BeneficiaryID != "" || in.ValidationID != "" {
			return q, Invalid("invalid internal recipient")
		}
		u, e := s.user(ctx, s.DB, in.RecipientID, false)
		if e != nil {
			return q, e
		}
		if e = s.eligible(u); e != nil {
			return q, denied()
		}
		d = Destination{UserID: u.ID, AccountName: u.Name}
	case "bank":
		if in.RecipientID != "" || in.ValidationID != "" {
			return q, Invalid("bank quote requires only a beneficiary")
		}
		if !s.Config.ExternalEnabled || s.Config.Gateway == nil {
			return q, unavailable()
		}
		var eid, sealed string
		if (in.BeneficiaryID == "") == (in.EnquiryID == "") {
			return q, Invalid("use exactly one saved beneficiary or current enquiry")
		}
		if in.EnquiryID != "" {
			e = s.DB.QueryRowContext(ctx, `SELECT id,destination_enc FROM enquiries WHERE id=$1 AND owner_id=$2 AND kind='bank' AND expires_at>$3`, in.EnquiryID, p.User.ID, s.Now()).Scan(&eid, &sealed)
		} else {
			e = s.DB.QueryRowContext(ctx, `SELECT e.id,e.destination_enc FROM beneficiaries b JOIN enquiries e ON e.id=b.enquiry_id WHERE b.id=$1 AND b.owner_id=$2 AND b.active`, in.BeneficiaryID, p.User.ID).Scan(&eid, &sealed)
		}
		if e != nil {
			return q, isMissing(e)
		}
		raw, e := s.Config.Box.Open(sealed, "enquiry:"+eid)
		if e != nil {
			return q, e
		}
		if e = json.Unmarshal([]byte(raw), &d); e != nil {
			return q, e
		}
		enquiry, e := s.Config.Gateway.Enquire(ctx, d)
		if e != nil {
			return q, ExternalError(e)
		}
		if !safeText(enquiry.Name, 150) {
			return q, unavailable()
		}
		d.AccountName = enquiry.Name
	case "bill":
		if in.EnquiryID != "" || in.RecipientID != "" || in.BeneficiaryID != "" {
			return q, Invalid("bill quote requires only a validation")
		}
		if !s.Config.ExternalEnabled || s.Config.Gateway == nil {
			return q, unavailable()
		}
		var sealed string
		var validatedAmount Money
		e = s.DB.QueryRowContext(ctx, `SELECT destination_enc,amount FROM enquiries WHERE id=$1 AND owner_id=$2 AND kind='bill' AND expires_at>$3`, in.ValidationID, p.User.ID, s.Now()).Scan(&sealed, &validatedAmount)
		if e != nil {
			return q, isMissing(e)
		}
		if validatedAmount != in.Amount {
			return q, conflict("bill validation amount differs")
		}
		raw, e := s.Config.Box.Open(sealed, "enquiry:"+in.ValidationID)
		if e != nil {
			return q, e
		}
		if e = json.Unmarshal([]byte(raw), &d); e != nil {
			return q, e
		}
	default:
		return q, Invalid("kind must be internal, bank or bill")
	}
	if in.Kind != "internal" {
		fee, e = s.Config.Gateway.Quote(ctx, in.Kind, d, in.Amount)
		if e != nil {
			return q, ExternalError(e)
		}
	}
	total, e := addMoney(in.Amount, fee)
	if e != nil {
		return q, e
	}
	q = Quote{ID: security.Random("quo_", 18), Kind: in.Kind, Amount: in.Amount, Fee: fee, Total: total, Currency: "NGN", Destination: d, Narration: in.Narration, PolicyVersion: version, ExpiresAt: s.Now().Add(5 * time.Minute)}
	sealed, e := s.Config.Box.Seal(jsonText(d), "quote:"+q.ID)
	if e != nil {
		return q, e
	}
	e = s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = s.eligible(u); e != nil {
			return e
		}
		return exec(tx, ctx, `INSERT INTO quotes(id,owner_id,kind,destination_enc,amount,fee,total,currency,narration,policy_version,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, q.ID, u.ID, q.Kind, sealed, int64(q.Amount), int64(q.Fee), int64(q.Total), q.Currency, q.Narration, q.PolicyVersion, q.ExpiresAt)
	})
	return q, e
}
func (s *Service) loadQuote(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, owner, id string, lock bool) (Quote, error) {
	var q Quote
	var encrypted string
	query := `SELECT id,kind,amount,fee,total,currency,destination_enc,narration,policy_version,expires_at,used FROM quotes WHERE id=$1 AND owner_id=$2`
	if lock {
		query += " FOR UPDATE"
	}
	e := db.QueryRowContext(ctx, query, id, owner).Scan(&q.ID, &q.Kind, &q.Amount, &q.Fee, &q.Total, &q.Currency, &encrypted, &q.Narration, &q.PolicyVersion, &q.ExpiresAt, &q.Used)
	if e != nil {
		return q, isMissing(e)
	}
	raw, e := s.Config.Box.Open(encrypted, "quote:"+q.ID)
	if e != nil {
		return q, e
	}
	e = json.Unmarshal([]byte(raw), &q.Destination)
	return q, e
}
func (s *Service) Quote(ctx context.Context, p Principal, id string) (Quote, error) {
	return s.loadQuote(ctx, s.DB, p.User.ID, id, false)
}
func (s *Service) Authorise(ctx context.Context, p Principal, id, pin, mfa string) (map[string]any, error) {
	if e := p.Customer(); e != nil {
		return nil, e
	}
	if e := s.Rate(ctx, "pin:"+p.User.ID, 5, 15*time.Minute); e != nil {
		return nil, e
	}
	token := security.Random("", 32)
	var expires time.Time
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = s.eligible(u); e != nil {
			return e
		}
		var hash string
		if e = tx.QueryRowContext(ctx, `SELECT pin_hash FROM users WHERE id=$1`, u.ID).Scan(&hash); e != nil {
			return e
		}
		if !security.Verify(pin, hash, s.Config.Pepper) {
			return unauthorized()
		}
		if u.MFA {
			if e = s.checkTOTP(ctx, tx, u.ID, mfa); e != nil {
				return e
			}
		}
		q, e := s.loadQuote(ctx, tx, u.ID, id, true)
		if e != nil {
			return e
		}
		if q.Used || !q.ExpiresAt.After(s.Now()) {
			return conflict("quote is used or expired")
		}
		expires = s.Now().Add(2 * time.Minute)
		if q.ExpiresAt.Before(expires) {
			expires = q.ExpiresAt
		}
		return exec(tx, ctx, `INSERT INTO authorisations(token_hash,quote_id,session_id,expires_at) VALUES($1,$2,$3,$4)`, security.Digest(token), id, p.SessionID, expires)
	})
	if e != nil {
		return nil, e
	}
	return map[string]any{"authorisation_token": token, "quote_id": id, "expires_at": expires}, nil
}
func (s *Service) CreatePayment(ctx context.Context, p Principal, quoteID, token, key string) (Payment, error) {
	var out Payment
	if e := p.Customer(); e != nil {
		return out, e
	}
	if len(key) < 16 || len(key) > 100 || strings.Trim(key, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-") != "" {
		return out, Invalid("Idempotency-Key must contain 16 to 100 safe characters")
	}
	preview, e := s.loadQuote(ctx, s.DB, p.User.ID, quoteID, false)
	if e != nil {
		return out, e
	}
	hash := security.Digest(quoteID)
	e = s.transact(ctx, func(tx *sql.Tx) error {
		ids := []string{p.User.ID}
		if preview.Kind == "internal" {
			ids = append(ids, preview.Destination.UserID)
		}
		if e := lockUsers(ctx, tx, ids...); e != nil {
			return e
		}
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		var previous, stored string
		e = tx.QueryRowContext(ctx, `SELECT id,request_hash FROM payments WHERE owner_id=$1 AND idempotency_key=$2`, u.ID, key).Scan(&previous, &stored)
		if e == nil {
			if stored != hash {
				return conflict("idempotency key is already bound to another request")
			}
			out, e = scanPayment(tx.QueryRowContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE id=$1`, previous))
			return e
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if e = s.eligible(u); e != nil {
			return e
		}
		q, e := s.loadQuote(ctx, tx, u.ID, quoteID, true)
		if e != nil {
			return e
		}
		if q.Used || !q.ExpiresAt.After(s.Now()) {
			return conflict("quote is used or expired")
		}
		var valid bool
		e = tx.QueryRowContext(ctx, `SELECT NOT consumed AND expires_at>$4 FROM authorisations WHERE token_hash=$1 AND quote_id=$2 AND session_id=$3 FOR UPDATE`, security.Digest(token), q.ID, p.SessionID, s.Now()).Scan(&valid)
		if errors.Is(e, sql.ErrNoRows) || !valid {
			return unauthorized()
		}
		if e != nil {
			return e
		}
		var version int64
		var per, daily Money
		var enabled bool
		e = tx.QueryRowContext(ctx, `SELECT version,per_payment,daily,payments_enabled FROM policies WHERE id=1 FOR SHARE`).Scan(&version, &per, &daily, &enabled)
		if e != nil {
			return e
		}
		if !enabled {
			return &Fault{503, "payments_disabled", "payments are disabled"}
		}
		if version != q.PolicyVersion {
			return conflict("policy changed; obtain and authorise a new quote")
		}
		if e = s.ensureSpendingControl(ctx, tx, u.ID, q.Total); e != nil {
			return e
		}
		if q.Total > per {
			return Invalid("total exceeds per-payment limit")
		}
		day := s.Now().Truncate(24 * time.Hour)
		var spent Money
		e = tx.QueryRowContext(ctx, `SELECT coalesce(sum(total),0) FROM payments WHERE owner_id=$1 AND created_at>=$2 AND created_at<$3 AND status<>'failed'`, u.ID, day, day.Add(24*time.Hour)).Scan(&spent)
		if e != nil {
			return e
		}
		if spent > daily-q.Total {
			return &Fault{409, "daily_limit", "daily payment limit exceeded"}
		}
		accountIDs := []string{"wallet:" + u.ID, "house:clearing", "house:fees"}
		var recipient any
		if q.Kind == "internal" {
			receiver, e := s.user(ctx, tx, q.Destination.UserID, false)
			if e != nil {
				return e
			}
			if e = s.eligible(receiver); e != nil {
				return denied()
			}
			recipient = receiver.ID
			accountIDs = append(accountIDs, "wallet:"+receiver.ID)
		} else if !s.Config.ExternalEnabled || s.Config.Gateway == nil {
			return unavailable()
		}
		if e = lockAccounts(ctx, tx, accountIDs...); e != nil {
			return e
		}
		var available Money
		if e = tx.QueryRowContext(ctx, `SELECT balance-reserved FROM accounts WHERE owner_id=$1`, u.ID).Scan(&available); e != nil {
			return e
		}
		if available < q.Total {
			return &Fault{409, "insufficient_funds", "available balance is insufficient"}
		}
		id := stringID("pay_")
		sealed, e := s.Config.Box.Seal(jsonText(q.Destination), "payment:"+id)
		if e != nil {
			return e
		}
		state := "accepted"
		fulfilment := "not_applicable"
		if q.Kind == "internal" {
			state = "succeeded"
		}
		if q.Kind == "bill" {
			fulfilment = "pending"
		}
		if e = exec(tx, ctx, `INSERT INTO payments(id,owner_id,recipient_id,quote_id,idempotency_key,request_hash,kind,status,amount,fee,total,currency,destination_enc,narration,fulfilment_status,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$16)`, id, u.ID, recipient, q.ID, key, hash, q.Kind, state, int64(q.Amount), int64(q.Fee), int64(q.Total), q.Currency, sealed, q.Narration, fulfilment, s.Now()); e != nil {
			return e
		}
		if e = s.applyRequestShare(ctx, tx, q, id, u.ID); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE quotes SET used=true WHERE id=$1`, q.ID); e != nil {
			return e
		}
		if e = exec(tx, ctx, `UPDATE authorisations SET consumed=true WHERE token_hash=$1`, security.Digest(token)); e != nil {
			return e
		}
		workflow := "transfer-submitted"
		if q.Kind == "bill" {
			workflow = "bills-order-received"
		}
		if q.Kind == "internal" {
			_, e = s.postJournal(ctx, tx, id, "internal_transfer", map[string]Money{"wallet:" + u.ID: -q.Total, "wallet:" + q.Destination.UserID: q.Amount, "house:fees": q.Fee})
			if e != nil {
				return e
			}
			workflow = "transfer-completed"
			if e = s.notify(ctx, tx, q.Destination.UserID, "transfer-internal-received", id, map[string]any{"amount_minor": q.Amount, "currency": "NGN"}, false); e != nil {
				return e
			}
		} else {
			if e = exec(tx, ctx, `INSERT INTO holds(id,payment_id,account_id,amount) VALUES($1,$2,$3,$4)`, "hold_"+id, id, "wallet:"+u.ID, int64(q.Total)); e != nil {
				return e
			}
			if e = exec(tx, ctx, `INSERT INTO jobs(id,kind,object_id,available_at) VALUES($1,'payment',$2,$3)`, "job_"+id, id, s.Now()); e != nil {
				return e
			}
		}
		if e = s.notify(ctx, tx, u.ID, workflow, id, map[string]any{"amount_minor": q.Amount, "currency": "NGN"}, false); e != nil {
			return e
		}
		if e = s.audit(ctx, tx, u.ID, "payment."+state, id, map[string]any{"kind": q.Kind, "quote_id": q.ID}); e != nil {
			return e
		}
		out, e = scanPayment(tx.QueryRowContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE id=$1`, id))
		return e
	})
	out.Direction = "outgoing"
	return out, e
}

// CreditFunding is called only by a verified upstream-evidence adapter or the isolated local fixture CLI.
// It has no customer-accessible credit endpoint.
func (s *Service) CreditFunding(ctx context.Context, source, reference, owner string, amount Money, currency string) (string, error) {
	if !safeText(source, 80) || !safeText(reference, 150) || amount <= 0 || amount > MaxMoney || currency != "NGN" {
		return "", Invalid("invalid funding evidence")
	}
	fingerprint := security.Digest(jsonText([]any{owner, amount, currency}))
	id := ""
	e := s.transact(ctx, func(tx *sql.Tx) error {
		if e := exec(tx, ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, source+":"+reference); e != nil {
			return e
		}
		var stored string
		e := tx.QueryRowContext(ctx, `SELECT id,fingerprint FROM funding WHERE source=$1 AND reference=$2`, source, reference).Scan(&id, &stored)
		if e == nil {
			if stored != fingerprint {
				return conflict("funding reference has conflicting evidence")
			}
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		u, e := s.user(ctx, tx, owner, true)
		if e != nil {
			return e
		}
		if u.Role != "customer" || u.Status == "closed" || !u.Verified {
			return denied()
		}
		id = stringID("fund_")
		journal, e := s.postJournal(ctx, tx, source+":"+reference, "funding", map[string]Money{"wallet:" + owner: amount, "house:clearing": -amount})
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO funding(id,source,reference,owner_id,amount,currency,fingerprint,journal_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, source, reference, owner, int64(amount), currency, fingerprint, journal); e != nil {
			return e
		}
		if e = s.notify(ctx, tx, owner, "funding-received", id, map[string]any{"amount_minor": amount, "currency": "NGN"}, false); e != nil {
			return e
		}
		return s.audit(ctx, tx, "upstream:"+source, "funding.credited", id, map[string]any{"reference_hash": security.Digest(reference)})
	})
	return id, e
}
