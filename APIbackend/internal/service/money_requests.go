package service

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

type RequestShareInput struct {
	PayerID string `json:"payer_id"`
	Amount  Money  `json:"amount_minor"`
}
type MoneyRequestInput struct {
	Memo      string              `json:"memo"`
	Amount    Money               `json:"amount_minor"`
	ExpiresAt time.Time           `json:"expires_at"`
	Shares    []RequestShareInput `json:"shares"`
}

func validIdempotency(key string) bool {
	return len(key) >= 16 && len(key) <= 100 && safeText(key, 100)
}
func (s *Service) RequestMoney(ctx context.Context, p Principal, in MoneyRequestInput, key string) (string, error) {
	if e := p.Customer(); e != nil {
		return "", e
	}
	if !validIdempotency(key) || !safeText(in.Memo, 200) || in.Amount <= 0 || in.Amount > MaxMoney || !in.ExpiresAt.After(s.Now()) || in.ExpiresAt.After(s.Now().AddDate(0, 3, 0)) || len(in.Shares) < 1 || len(in.Shares) > 20 {
		return "", Invalid("provide a positive amount, memo, expiry within three months and 1 to 20 payers")
	}
	seen := map[string]bool{}
	total := Money(0)
	for _, share := range in.Shares {
		if !validID(share.PayerID) || share.PayerID == p.User.ID || share.Amount <= 0 || seen[share.PayerID] {
			return "", Invalid("payers must be unique other customers with positive shares")
		}
		seen[share.PayerID] = true
		var e error
		total, e = addMoney(total, share.Amount)
		if e != nil {
			return "", e
		}
	}
	if total != in.Amount {
		return "", Invalid("shares must add up exactly to the requested amount")
	}
	if e := s.Rate(ctx, "money-request:"+p.User.ID, 30, time.Hour); e != nil {
		return "", e
	}
	fingerprint := security.Digest(jsonText(in))
	id := stringID("request_")
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = s.eligible(u); e != nil {
			return e
		}
		var oldID, hash string
		e = tx.QueryRowContext(ctx, `SELECT id,fingerprint FROM money_requests WHERE owner_id=$1 AND idempotency_key=$2`, u.ID, key).Scan(&oldID, &hash)
		if e == nil {
			if hash != fingerprint {
				return conflict("request key already belongs to different details")
			}
			id = oldID
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		for _, share := range in.Shares {
			recipient, e := s.user(ctx, tx, share.PayerID, false)
			if e != nil || recipient.Role != "customer" || recipient.Status != "active" {
				return Invalid("a payer is not available")
			}
		}
		if e = exec(tx, ctx, `INSERT INTO money_requests(id,owner_id,amount,currency,memo,expires_at,idempotency_key,fingerprint) VALUES($1,$2,$3,'NGN',$4,$5,$6,$7)`, id, u.ID, int64(in.Amount), in.Memo, in.ExpiresAt, key, fingerprint); e != nil {
			return e
		}
		for _, share := range in.Shares {
			if e = exec(tx, ctx, `INSERT INTO request_shares(id,request_id,payer_id,amount) VALUES($1,$2,$3,$4)`, stringID("share_"), id, share.PayerID, int64(share.Amount)); e != nil {
				return e
			}
			if e = s.notify(ctx, tx, share.PayerID, "requests-received", id, map[string]any{"amount_minor": share.Amount}, false); e != nil {
				return e
			}
		}
		return s.audit(ctx, tx, u.ID, "money_request.created", id, map[string]any{"participants": len(in.Shares)})
	})
	return id, e
}
func (s *Service) MoneyRequests(ctx context.Context, p Principal, before string) ([]map[string]any, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT r.id,r.owner_id,u.name,r.amount,r.memo,r.expires_at,CASE WHEN r.status='open' AND r.expires_at<=$3 THEN 'expired' ELSE r.status END,r.created_at,coalesce((SELECT sum(received) FROM request_shares WHERE request_id=r.id),0) FROM money_requests r JOIN users u ON u.id=r.owner_id WHERE (r.owner_id=$1 OR EXISTS(SELECT 1 FROM request_shares s WHERE s.request_id=r.id AND s.payer_id=$1)) AND ($2='' OR r.id<$2) ORDER BY r.id DESC LIMIT 50`, p.User.ID, before, s.Now())
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, owner, name, memo, status string
		var amount, received Money
		var expiry, at time.Time
		if e = rows.Scan(&id, &owner, &name, &amount, &memo, &expiry, &status, &at, &received); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "owner_id": owner, "requester_name": name, "currency": "NGN", "amount_minor": amount, "received_minor": received, "memo": memo, "expires_at": expiry, "status": status, "created_at": at, "direction": func() string {
			if owner == p.User.ID {
				return "outgoing"
			}
			return "incoming"
		}()})
	}
	return out, rows.Err()
}
func (s *Service) MoneyRequest(ctx context.Context, p Principal, id string) (map[string]any, error) {
	var owner, name, memo, status string
	var amount Money
	var expiry time.Time
	e := s.DB.QueryRowContext(ctx, `SELECT r.owner_id,u.name,r.amount,r.memo,r.expires_at,r.status FROM money_requests r JOIN users u ON u.id=r.owner_id WHERE r.id=$1 AND (r.owner_id=$2 OR EXISTS(SELECT 1 FROM request_shares WHERE request_id=r.id AND payer_id=$2))`, id, p.User.ID).Scan(&owner, &name, &amount, &memo, &expiry, &status)
	if e != nil {
		return nil, isMissing(e)
	}
	if status == "open" && !expiry.After(s.Now()) {
		status = "expired"
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT s.id,s.payer_id,u.name,s.amount,s.received,s.status FROM request_shares s JOIN users u ON u.id=s.payer_id WHERE s.request_id=$1 AND ($2 OR s.payer_id=$3) ORDER BY s.id`, id, owner == p.User.ID, p.User.ID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	shares := []map[string]any{}
	for rows.Next() {
		var sid, payer, pname, ss string
		var owed, received Money
		if e = rows.Scan(&sid, &payer, &pname, &owed, &received, &ss); e != nil {
			return nil, e
		}
		shares = append(shares, map[string]any{"id": sid, "payer_id": payer, "name": pname, "amount_minor": owed, "received_minor": received, "remaining_minor": owed - received, "status": ss})
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	return map[string]any{"id": id, "owner_id": owner, "requester_name": name, "amount_minor": amount, "currency": "NGN", "memo": memo, "expires_at": expiry, "status": status, "shares": shares}, nil
}
func (s *Service) MoneyRequestState(ctx context.Context, p Principal, id, action string) error {
	if action != "cancel" && action != "decline" && action != "remind" {
		return Invalid("invalid request action")
	}
	if e := s.Rate(ctx, "request-action:"+p.User.ID, 20, time.Hour); e != nil {
		return e
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		if _, e := s.activePrincipal(ctx, tx, p); e != nil {
			return e
		}
		var owner, status string
		var expires time.Time
		if e := tx.QueryRowContext(ctx, `SELECT owner_id,status,expires_at FROM money_requests WHERE id=$1 FOR UPDATE`, id).Scan(&owner, &status, &expires); e != nil {
			return isMissing(e)
		}
		if status != "open" || !expires.After(s.Now()) {
			return conflict("request is no longer open")
		}
		if action == "decline" {
			r, e := tx.ExecContext(ctx, `UPDATE request_shares SET status='declined' WHERE request_id=$1 AND payer_id=$2 AND status='open'`, id, p.User.ID)
			if e != nil {
				return e
			}
			n, _ := r.RowsAffected()
			if n != 1 {
				return missing()
			}
		} else {
			if owner != p.User.ID {
				return missing()
			}
			if action == "cancel" {
				if e := exec(tx, ctx, `UPDATE money_requests SET status='cancelled' WHERE id=$1`, id); e != nil {
					return e
				}
			} else {
				if e := s.Rate(ctx, "request-reminder:"+id, 1, 24*time.Hour); e != nil {
					return e
				}
				rows, e := tx.QueryContext(ctx, `SELECT payer_id,amount-received FROM request_shares WHERE request_id=$1 AND status='open'`, id)
				if e != nil {
					return e
				}
				targets := []RequestShareInput{}
				for rows.Next() {
					var v RequestShareInput
					if e = rows.Scan(&v.PayerID, &v.Amount); e != nil {
						rows.Close()
						return e
					}
					targets = append(targets, v)
				}
				e = rows.Err()
				rows.Close()
				if e != nil {
					return e
				}
				for _, v := range targets {
					if e = s.notifyOccurrence(ctx, tx, v.PayerID, "requests-received", id, stringID("nudge_"), map[string]any{"amount_minor": v.Amount}, false); e != nil {
						return e
					}
				}
			}
		}
		return s.audit(ctx, tx, p.User.ID, "money_request."+action, id, map[string]any{})
	})
}
func (s *Service) RequestQuote(ctx context.Context, p Principal, id string, amount Money) (Quote, error) {
	var owner, memo, status string
	var expires time.Time
	var shareID string
	var remaining Money
	e := s.DB.QueryRowContext(ctx, `SELECT r.owner_id,r.memo,r.status,r.expires_at,s.id,s.amount-s.received FROM money_requests r JOIN request_shares s ON s.request_id=r.id WHERE r.id=$1 AND s.payer_id=$2 AND s.status='open'`, id, p.User.ID).Scan(&owner, &memo, &status, &expires, &shareID, &remaining)
	if e != nil {
		return Quote{}, isMissing(e)
	}
	if status != "open" || !expires.After(s.Now()) || amount <= 0 || amount > remaining {
		return Quote{}, conflict("request expired, changed, or amount exceeds your unpaid share")
	}
	q, e := s.CreateQuote(ctx, p, QuoteInput{Kind: "internal", Amount: amount, Currency: "NGN", RecipientID: owner, Narration: "Money request " + id})
	if e != nil {
		return q, e
	}
	// Financial commit re-checks this linkage, expiry and unpaid share under row locks.
	e = s.transact(ctx, func(tx *sql.Tx) error {
		if _, e := s.activePrincipal(ctx, tx, p); e != nil {
			return e
		}
		return exec(tx, ctx, `INSERT INTO request_quote_links(quote_id,share_id,amount) VALUES($1,$2,$3)`, q.ID, shareID, int64(amount))
	})
	return q, e
}
func (s *Service) applyRequestShare(ctx context.Context, tx *sql.Tx, q Quote, payment string, owner string) error {
	var rid, sid string
	e := tx.QueryRowContext(ctx, `SELECT s.request_id,s.id FROM request_quote_links l JOIN request_shares s ON s.id=l.share_id WHERE l.quote_id=$1`, q.ID).Scan(&rid, &sid)
	if errors.Is(e, sql.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	var recipient, status string
	var expires time.Time
	if e = tx.QueryRowContext(ctx, `SELECT owner_id,status,expires_at FROM money_requests WHERE id=$1 FOR UPDATE`, rid).Scan(&recipient, &status, &expires); e != nil {
		return e
	}
	if status != "open" || !expires.After(s.Now()) || q.Kind != "internal" || q.Destination.UserID != recipient {
		return conflict("money request is no longer payable")
	}
	var payer, shareState string
	var amount, received Money
	if e = tx.QueryRowContext(ctx, `SELECT payer_id,amount,received,status FROM request_shares WHERE id=$1 FOR UPDATE`, sid).Scan(&payer, &amount, &received, &shareState); e != nil {
		return e
	}
	if payer != owner || shareState != "open" || q.Amount > amount-received {
		return conflict("request share changed; obtain a new quote")
	}
	if e = exec(tx, ctx, `UPDATE request_shares SET received=received+$2,status=CASE WHEN received+$2=amount THEN 'paid' ELSE 'open' END WHERE id=$1`, sid, int64(q.Amount)); e != nil {
		return e
	}
	if e = exec(tx, ctx, `UPDATE request_quote_links SET payment_id=$2 WHERE quote_id=$1`, q.ID, payment); e != nil {
		return e
	}
	if e = exec(tx, ctx, `UPDATE money_requests SET status='completed' WHERE id=$1 AND NOT EXISTS(SELECT 1 FROM request_shares WHERE request_id=$1 AND received<amount)`, rid); e != nil {
		return e
	}
	return s.notifyOccurrence(ctx, tx, recipient, "requests-payment-received", rid, payment, map[string]any{"amount_minor": q.Amount}, false)
}
