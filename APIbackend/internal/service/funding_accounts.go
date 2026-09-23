package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"time"
)

// An execution partner must prove these contracts before being configured. A
// signed notification is only a requery hint, never itself a funding credit.
type FundingAccountResult struct {
	Reference     string `json:"reference"`
	State         string `json:"state"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
	Institution   string `json:"institution"`
	Synthetic     bool   `json:"synthetic"`
}
type FundingEvidence struct {
	Reference        string
	AccountReference string
	Amount           Money
	Currency         string
	Final            bool
}
type FundingGateway interface {
	Name() string
	Provision(context.Context, string, User) (FundingAccountResult, error)
	Account(context.Context, string) (FundingAccountResult, error)
	VerifyCredit(context.Context, string) (FundingEvidence, error)
}

func (s *Service) FundingAccount(ctx context.Context, p Principal) (map[string]any, error) {
	var id, state, sealed string
	var updated time.Time
	e := s.DB.QueryRowContext(ctx, `SELECT id,state,details_enc,updated_at FROM funding_accounts WHERE owner_id=$1`, p.User.ID).Scan(&id, &state, &sealed, &updated)
	if errors.Is(e, sql.ErrNoRows) {
		return map[string]any{"state": "not_created", "configured": s.Config.FundingGateway != nil}, nil
	}
	if e != nil {
		return nil, e
	}
	out := map[string]any{"id": id, "state": state, "updated_at": updated, "configured": s.Config.FundingGateway != nil}
	if state == "active" && sealed != "" {
		raw, e := s.Config.Box.Open(sealed, "funding-account:"+id)
		if e != nil {
			return nil, e
		}
		var detail FundingAccountResult
		if e = json.Unmarshal([]byte(raw), &detail); e != nil {
			return nil, e
		}
		out["details"] = detail
	}
	return out, nil
}
func (s *Service) RequestFundingAccount(ctx context.Context, p Principal) (map[string]any, error) {
	if e := p.Customer(); e != nil {
		return nil, e
	}
	if s.Config.FundingGateway == nil {
		return nil, &Fault{503, "funding_not_configured", "funding-account partner is not configured; no account details were invented"}
	}
	id := stringID("fundacct_")
	e := s.transact(ctx, func(tx *sql.Tx) error {
		u, e := s.activePrincipal(ctx, tx, p)
		if e != nil {
			return e
		}
		if e = s.eligible(u); e != nil {
			return e
		}
		return exec(tx, ctx, `INSERT INTO funding_accounts(id,owner_id,provider,state,idempotency_key,created_at,updated_at) VALUES($1,$2,$3,'requested',$1,$4,$4) ON CONFLICT(owner_id) DO NOTHING`, id, u.ID, s.Config.FundingGateway.Name(), s.Now())
	})
	if e != nil {
		return nil, e
	}
	return s.FundingAccount(ctx, p)
}
func (s *Service) WorkFundingAccount(ctx context.Context) (bool, error) {
	if s.Config.FundingGateway == nil {
		return false, nil
	}
	var id, uid, state, key string
	submit := false
	e := s.transact(ctx, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx, `SELECT id,owner_id,state,idempotency_key FROM funding_accounts WHERE provider=$1 AND state IN('requested','pending') AND updated_at<=$2 ORDER BY updated_at FOR UPDATE SKIP LOCKED LIMIT 1`, s.Config.FundingGateway.Name(), s.Now()).Scan(&id, &uid, &state, &key)
		if e != nil {
			return e
		}
		submit = state == "requested"
		return exec(tx, ctx, `UPDATE funding_accounts SET state='pending',updated_at=$2 WHERE id=$1`, id, s.Now().Add(time.Minute))
	})
	if errors.Is(e, sql.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	u, e := s.Me(ctx, uid)
	if e != nil {
		return true, e
	}
	var result FundingAccountResult
	if submit {
		result, e = s.Config.FundingGateway.Provision(ctx, key, u)
	} else {
		result, e = s.Config.FundingGateway.Account(ctx, key)
	}
	if e != nil {
		return true, e
	}
	if result.State != "active" || !safeText(result.Reference, 150) || !safeText(result.AccountName, 150) || !safeText(result.Institution, 150) || !safeText(result.AccountNumber, 32) || result.Synthetic && s.Config.Environment != "local" {
		return true, nil
	}
	sealed, e := s.Config.Box.Seal(jsonText(result), "funding-account:"+id)
	if e != nil {
		return true, e
	}
	e = s.transact(ctx, func(tx *sql.Tx) error {
		if e := exec(tx, ctx, `UPDATE funding_accounts SET state='active',provider_ref=$2,details_enc=$3,updated_at=$4 WHERE id=$1 AND state='pending'`, id, result.Reference, sealed, s.Now()); e != nil {
			return e
		}
		return s.notify(ctx, tx, uid, "funding-account-ready", id, nil, false)
	})
	return true, e
}
func (s *Service) VerifyIncomingFunding(ctx context.Context, reference string) (string, error) {
	if s.Config.FundingGateway == nil {
		return "", unavailable()
	}
	if !safeText(reference, 150) {
		return "", Invalid("invalid funding reference")
	}
	evidence, e := s.Config.FundingGateway.VerifyCredit(ctx, reference)
	if e != nil {
		return "", unavailable()
	}
	if !evidence.Final || evidence.Reference != reference || evidence.Amount <= 0 || evidence.Currency != "NGN" {
		return "", conflict("incoming payment is not independently confirmed")
	}
	var owner string
	e = s.DB.QueryRowContext(ctx, `SELECT owner_id FROM funding_accounts WHERE provider=$1 AND provider_ref=$2 AND state='active'`, s.Config.FundingGateway.Name(), evidence.AccountReference).Scan(&owner)
	if e != nil {
		return "", isMissing(e)
	}
	return s.CreditFunding(ctx, s.Config.FundingGateway.Name(), reference, owner, evidence.Amount, evidence.Currency)
}

// LocalFunding demonstrates account details only. It cannot verify any incoming
// real payment and no public endpoint can mint synthetic funds.
type LocalFunding struct{}

func (LocalFunding) Name() string { return "synthetic-local" }
func (LocalFunding) Provision(_ context.Context, key string, u User) (FundingAccountResult, error) {
	return FundingAccountResult{Reference: key, State: "active", AccountName: "SYNTHETIC — " + u.Name, AccountNumber: "TEST-" + security.Digest(key)[:10], Institution: "Local simulator — not a bank", Synthetic: true}, nil
}
func (LocalFunding) Account(ctx context.Context, key string) (FundingAccountResult, error) {
	return LocalFunding{}.Provision(ctx, key, User{Name: "Review account"})
}
func (LocalFunding) VerifyCredit(context.Context, string) (FundingEvidence, error) {
	return FundingEvidence{}, errors.New("synthetic funding accepts no provider credit callbacks")
}
