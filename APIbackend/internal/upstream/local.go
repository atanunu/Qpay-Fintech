// Package upstream adapts execution services. The local gateway is synthetic and
// must never be selected outside an explicitly local development environment.
package upstream

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/service"
)

type Local struct{ DB *sql.DB }

func (l *Local) Banks(context.Context) ([]service.Bank, error) {
	return []service.Bank{{Code: "999", Name: "SYNTHETIC TEST BANK — no real payments"}}, nil
}
func (l *Local) Products(context.Context) ([]service.Product, error) {
	return []service.Product{{ID: "synthetic-airtime", Name: "Synthetic airtime", Category: "airtime", Variable: true}, {ID: "synthetic-electricity", Name: "Synthetic electricity", Category: "electricity", Variable: true}}, nil
}
func (l *Local) Enquire(_ context.Context, d service.Destination) (service.Enquiry, error) {
	if d.BankCode != "999" {
		return service.Enquiry{}, service.Invalid("local mode accepts only synthetic bank 999")
	}
	return service.Enquiry{Name: "SYNTHETIC TEST RECIPIENT", Reference: security.Digest(d.AccountNumber)}, nil
}
func (l *Local) ValidateBill(_ context.Context, d service.Destination, a service.Money) (service.Enquiry, error) {
	if d.ProductID != "synthetic-airtime" && d.ProductID != "synthetic-electricity" {
		return service.Enquiry{}, service.Invalid("unknown synthetic product")
	}
	if a <= 0 {
		return service.Enquiry{}, service.Invalid("positive amount required")
	}
	return service.Enquiry{Name: "SYNTHETIC BILL CUSTOMER", Reference: security.Digest(d.CustomerID)}, nil
}
func (l *Local) Quote(context.Context, string, service.Destination, service.Money) (service.Money, error) {
	return 100, nil
}
func (l *Local) Submit(ctx context.Context, p service.ProviderRequest) (service.ProviderResult, error) {
	r := service.ProviderResult{Reference: p.Reference, UpstreamID: "sim_" + p.Reference, Status: "succeeded", Amount: p.Amount, Currency: p.Currency, Fulfilled: p.Kind == "bill"}
	if p.Kind == "bill" {
		r.Fulfilment = "SYNTHETIC VALUE — NOT VALID FOR REDEMPTION"
	}
	raw, e := json.Marshal(r)
	if e != nil {
		return r, e
	}
	request, e := json.Marshal(p)
	if e != nil {
		return r, e
	}
	hash := security.Digest(string(request))
	_, e = l.DB.ExecContext(ctx, `INSERT INTO simulated_operations(reference,request_hash,result) VALUES($1,$2,$3) ON CONFLICT(reference) DO NOTHING`, p.Reference, hash, string(raw))
	if e != nil {
		return r, e
	}
	var savedHash string
	var saved []byte
	e = l.DB.QueryRowContext(ctx, `SELECT request_hash,result FROM simulated_operations WHERE reference=$1`, p.Reference).Scan(&savedHash, &saved)
	if e != nil {
		return r, e
	}
	if savedHash != hash {
		return r, errors.New("synthetic request conflict")
	}
	e = json.Unmarshal(saved, &r)
	return r, e
}
func (l *Local) Query(ctx context.Context, p service.ProviderRequest) (service.ProviderResult, error) {
	var r service.ProviderResult
	var raw []byte
	e := l.DB.QueryRowContext(ctx, `SELECT result FROM simulated_operations WHERE reference=$1`, p.Reference).Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return r, fmt.Errorf("original synthetic submission not found; no replacement submitted")
	}
	if e != nil {
		return r, e
	}
	e = json.Unmarshal(raw, &r)
	return r, e
}
