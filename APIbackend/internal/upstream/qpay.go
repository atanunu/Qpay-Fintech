package upstream

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/service"
)

// QPay uses the inspected MercuryGo application API. Live capability acceptance
// is separate from HTTP contract testing and must be explicitly enabled.
type QPay struct {
	BaseURL, ApplicationID, Key, Secret, Environment string
	Sender                                           service.Destination
	Client                                           *http.Client
	Now                                              func() time.Time
}

func NewQPay(base, app, key, secret, environment string, sender service.Destination) (*QPay, error) {
	u, e := url.Parse(base)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" {
		return nil, errors.New("QPay requires an HTTPS origin without credentials, path or query")
	}
	if app == "" || key == "" || len(secret) < 32 || (environment != "sandbox" && environment != "production") {
		return nil, errors.New("QPay application credentials and environment are required")
	}
	if !regexp.MustCompile(`^[0-9]{10}$`).MatchString(sender.AccountNumber) || sender.BankCode == "" || sender.AccountName == "" {
		return nil, errors.New("verified upstream settlement sender is required")
	}
	return &QPay{BaseURL: strings.TrimRight(base, "/"), ApplicationID: app, Key: key, Secret: secret, Environment: environment, Sender: sender, Client: &http.Client{Timeout: 55 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects disabled") }}, Now: time.Now}, nil
}
func (q *QPay) request(ctx context.Context, method, path, key string, body any, target any) error {
	var raw []byte
	var e error
	if body != nil {
		raw, e = json.Marshal(body)
		if e != nil {
			return e
		}
	}
	req, e := http.NewRequestWithContext(ctx, method, q.BaseURL+path, bytes.NewReader(raw))
	if e != nil {
		return e
	}
	req.Header.Set("X-Application-ID", q.ApplicationID)
	req.Header.Set("X-API-Key", q.Key)
	req.Header.Set("X-API-Secret", q.Secret)
	req.Header.Set("X-QPay-Environment", q.Environment)
	req.Header.Set("Accept", "application/json")
	if method != http.MethodGet {
		if key == "" {
			return errors.New("write idempotency key required")
		}
		timestamp := strconv.FormatInt(q.Now().Unix(), 10)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Idempotency-Key", key)
		req.Header.Set("X-Timestamp", timestamp)
		req.Header.Set("X-Signature", security.MAC([]byte(q.Secret), q.ApplicationID+"."+key+"."+timestamp+"."+security.Digest(string(raw))))
	}
	res, e := q.Client.Do(req)
	if e != nil {
		return errors.New("QPay transport outcome unknown")
	}
	defer res.Body.Close()
	data, e := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if e != nil || len(data) > 2<<20 {
		return errors.New("invalid QPay response size")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("QPay HTTP %d; preserve original operation", res.StatusCode)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if e = decoder.Decode(target); e != nil {
		return fmt.Errorf("invalid QPay JSON: %w", e)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("trailing QPay JSON")
	}
	return nil
}

func (q *QPay) object(ctx context.Context, method, path, key string, body any, target any) error {
	var raw json.RawMessage
	if e := q.request(ctx, method, path, key, body, &raw); e != nil {
		return e
	}
	if strings.HasPrefix(path, "/v1/bills/") {
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if e := json.Unmarshal(raw, &envelope); e != nil || len(envelope.Data) == 0 {
			return errors.New("bill response has no data object")
		}
		raw = envelope.Data
	}
	return json.Unmarshal(raw, target)
}

func rawMoney(raw json.RawMessage) (service.Money, error) {
	var text string
	if len(raw) == 0 {
		return 0, errors.New("missing upstream amount")
	}
	if raw[0] == '"' {
		if e := json.Unmarshal(raw, &text); e != nil {
			return 0, e
		}
	} else {
		text = string(raw)
	}
	var m service.Money
	e := json.Unmarshal([]byte(strconv.Quote(text)), &m)
	return m, e
}
func (q *QPay) Banks(ctx context.Context) ([]service.Bank, error) {
	var data struct {
		Banks []struct {
			Code   string `json:"bank_code"`
			Name   string `json:"bank_name"`
			Status string `json:"status"`
		} `json:"banks"`
	}
	if e := q.request(ctx, "GET", "/v1/banks", "", nil, &data); e != nil {
		return nil, e
	}
	out := []service.Bank{}
	for _, b := range data.Banks {
		if b.Status == "active" && b.Code != "" && b.Name != "" {
			out = append(out, service.Bank{Code: b.Code, Name: b.Name})
		}
	}
	return out, nil
}

// Products returns a bounded normalised catalogue. An unexpected upstream shape
// is an integration error, never a fabricated product or synthetic fallback.
func (q *QPay) Products(ctx context.Context) ([]service.Product, error) {
	var billers struct {
		Data []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			CategoryID string `json:"category_id"`
		} `json:"data"`
	}
	if e := q.request(ctx, "GET", "/v1/bills/billers", "", nil, &billers); e != nil {
		return nil, e
	}
	if len(billers.Data) > 100 {
		return nil, errors.New("catalogue exceeds qualified bound; add pagination before enabling this catalogue")
	}
	out := []service.Product{}
	for _, b := range billers.Data {
		if b.ID == "" {
			return nil, errors.New("missing biller identifier")
		}
		var items struct {
			Data []struct {
				ID       string          `json:"id"`
				Name     string          `json:"name"`
				Amount   json.RawMessage `json:"amount"`
				Variable bool            `json:"is_variable_amount"`
			} `json:"data"`
		}
		if e := q.request(ctx, "GET", "/v1/bills/billers/"+url.PathEscape(b.ID)+"/items", "", nil, &items); e != nil {
			return nil, e
		}
		for _, item := range items.Data {
			if item.ID == "" || item.Name == "" {
				return nil, errors.New("unqualified bill item shape")
			}
			amount := service.Money(0)
			if len(item.Amount) > 0 && string(item.Amount) != "null" {
				var e error
				amount, e = rawMoney(item.Amount)
				if e != nil {
					return nil, e
				}
			}
			ref, _ := json.Marshal([]string{b.CategoryID, b.ID, item.ID})
			out = append(out, service.Product{ID: base64.RawURLEncoding.EncodeToString(ref), Name: item.Name, Category: b.CategoryID, Amount: amount, Variable: item.Variable || amount == 0})
			if len(out) > 1000 {
				return nil, errors.New("catalogue exceeds qualified bound")
			}
		}
	}
	return out, nil
}
func (q *QPay) Enquire(ctx context.Context, d service.Destination) (service.Enquiry, error) {
	var result struct {
		Exists bool   `json:"account_exists"`
		Name   string `json:"account_name"`
	}
	body := map[string]any{"account_number": d.AccountNumber, "bank_code": d.BankCode, "currency": "NGN"}
	if e := q.request(ctx, "POST", "/v1/transfers/account/name-enquiry", security.Random("enquiry_", 18), body, &result); e != nil {
		return service.Enquiry{}, e
	}
	if !result.Exists || strings.TrimSpace(result.Name) == "" {
		return service.Enquiry{}, service.Invalid("account could not be verified")
	}
	return service.Enquiry{Name: result.Name, Reference: security.Digest(d.BankCode + ":" + d.AccountNumber)}, nil
}
func billPayload(d service.Destination, a service.Money) (map[string]any, error) {
	raw, e := base64.RawURLEncoding.DecodeString(d.ProductID)
	if e != nil {
		return nil, service.Invalid("invalid product identifier")
	}
	var ids []string
	if json.Unmarshal(raw, &ids) != nil || len(ids) != 3 || ids[1] == "" || ids[2] == "" {
		return nil, service.Invalid("invalid product identifier")
	}
	return map[string]any{"category_id": ids[0], "biller_id": ids[1], "item_id": ids[2], "customer_reference": d.CustomerID, "amount": strconv.FormatInt(int64(a), 10), "currency": "NGN"}, nil
}
func (q *QPay) ValidateBill(ctx context.Context, d service.Destination, a service.Money) (service.Enquiry, error) {
	body, e := billPayload(d, a)
	if e != nil {
		return service.Enquiry{}, e
	}
	var result struct {
		Status              string `json:"status"`
		CustomerName        string `json:"customer_name"`
		ValidationReference string `json:"reference"`
	}
	if e = q.object(ctx, "POST", "/v1/bills/validate-customer", security.Random("validate_", 18), body, &result); e != nil {
		return service.Enquiry{}, e
	}
	if result.Status != "VALIDATED" || result.CustomerName == "" {
		return service.Enquiry{}, service.Invalid("bill customer validation did not return a qualified result")
	}
	return service.Enquiry{Name: result.CustomerName, Reference: result.ValidationReference}, nil
}
func (q *QPay) Quote(ctx context.Context, kind string, d service.Destination, a service.Money) (service.Money, error) {
	var result struct {
		Amount   json.RawMessage `json:"amount"`
		Total    json.RawMessage `json:"total_debit_amount"`
		Currency string          `json:"currency"`
	}
	var e error
	if kind == "bank" {
		e = q.request(ctx, "GET", "/v1/quotes?currency=NGN&transfer_type=nip&amount="+strconv.FormatInt(int64(a), 10), "", nil, &result)
	} else {
		body, err := billPayload(d, a)
		if err != nil {
			return 0, err
		}
		e = q.object(ctx, "POST", "/v1/bills/quote", security.Random("quote_", 18), body, &result)
	}
	if e != nil {
		return 0, e
	}
	amount, e := rawMoney(result.Amount)
	if e != nil {
		return 0, e
	}
	total, e := rawMoney(result.Total)
	if e != nil {
		return 0, e
	}
	if amount != a || total < a || result.Currency != "NGN" {
		return 0, errors.New("upstream quote does not match requested amount/currency")
	}
	return total - a, nil
}
func (q *QPay) Submit(ctx context.Context, p service.ProviderRequest) (service.ProviderResult, error) {
	path := "/v1/transfers"
	var body map[string]any
	var e error
	if p.Kind == "bank" {
		party := func(d service.Destination) map[string]string {
			return map[string]string{"account_number": d.AccountNumber, "account_name": d.AccountName, "bank_code": d.BankCode}
		}
		body = map[string]any{"amount": strconv.FormatInt(int64(p.Amount), 10), "currency": p.Currency, "transfer_type": "nip", "sender": party(q.Sender), "receiver": party(p.Destination), "narration": p.Narration, "trace_id": p.Reference}
	} else {
		path = "/v1/bills/payments"
		body, e = billPayload(p.Destination, p.Amount)
		if e != nil {
			return service.ProviderResult{}, e
		}
		body["metadata"] = map[string]string{"client_reference": p.Reference}
	}
	var accepted struct {
		Reference string `json:"reference"`
	}
	if e = q.object(ctx, "POST", path, p.Reference, body, &accepted); e != nil {
		return service.ProviderResult{}, e
	}
	if accepted.Reference == "" {
		return service.ProviderResult{}, errors.New("upstream accepted response has no reference; investigate original key")
	}
	return service.ProviderResult{Reference: p.Reference, UpstreamID: accepted.Reference, Status: "pending", Amount: p.Amount, Currency: p.Currency}, nil
}
func (q *QPay) Query(ctx context.Context, p service.ProviderRequest) (service.ProviderResult, error) {
	path := "/v1/transfers"
	if p.Kind == "bill" {
		path = "/v1/bills/payments"
	}
	reference := p.UpstreamID
	if reference == "" {
		var list struct {
			Data []struct {
				Reference string                     `json:"reference"`
				Trace     string                     `json:"client_trace_id"`
				Key       string                     `json:"idempotency_key"`
				Metadata  map[string]json.RawMessage `json:"metadata"`
			} `json:"data"`
		}
		if e := q.request(ctx, "GET", path, "", nil, &list); e != nil {
			return service.ProviderResult{}, e
		}
		for _, v := range list.Data {
			var trace string
			_ = json.Unmarshal(v.Metadata["client_reference"], &trace)
			if v.Trace == p.Reference || v.Key == p.Reference || trace == p.Reference {
				if reference != "" && reference != v.Reference {
					return service.ProviderResult{}, errors.New("multiple original-reference candidates; investigate")
				}
				reference = v.Reference
			}
		}
		if reference == "" {
			return service.ProviderResult{}, errors.New("original operation is not discoverable; no replacement will be submitted")
		}
	}
	var raw struct {
		Reference string                     `json:"reference"`
		Status    string                     `json:"status"`
		Amount    json.RawMessage            `json:"amount"`
		Currency  string                     `json:"currency"`
		Result    map[string]json.RawMessage `json:"result"`
	}
	method, suffix, body := "GET", "", any(nil)
	if p.Kind == "bill" {
		method = "POST"
		suffix = "/requery"
		body = map[string]any{}
	}
	if e := q.object(ctx, method, path+"/"+url.PathEscape(reference)+suffix, "query_"+p.Reference, body, &raw); e != nil {
		return service.ProviderResult{}, e
	}
	amount, e := rawMoney(raw.Amount)
	if e != nil {
		return service.ProviderResult{}, e
	}
	if raw.Reference != reference {
		return service.ProviderResult{}, errors.New("upstream reference mismatch")
	}
	result := service.ProviderResult{Reference: p.Reference, UpstreamID: reference, Amount: amount, Currency: raw.Currency, Status: "pending"}
	switch raw.Status {
	case "SETTLED", "SUCCESSFUL":
		result.Status = "succeeded"
	case "FAILED", "REJECTED", "DECLINED", "CANCELLED":
		result.Status = "failed"
	case "VALIDATED", "QUEUED", "PROVIDER_PENDING", "APPROVAL_PENDING", "PENDING":
	default:
		return result, errors.New("unqualified upstream state; retain funds and investigate")
	}
	if p.Kind == "bill" && result.Status == "succeeded" {
		var token string
		_ = json.Unmarshal(raw.Result["token"], &token)
		if token != "" {
			result.Fulfilled = true
			result.Fulfilment = token
		} else {
			var fulfilled bool
			_ = json.Unmarshal(raw.Result["fulfilled"], &fulfilled)
			if fulfilled {
				result.Fulfilled = true
				result.Fulfilment = "Provider confirmed fulfilment"
			}
		}
	}
	return result, nil
}
