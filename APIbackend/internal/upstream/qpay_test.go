package upstream

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/service"
)

func testQPay(t *testing.T, handler http.HandlerFunc) *QPay {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	q, e := NewQPay(server.URL, "test_app", "test_key", strings.Repeat("s", 32), "sandbox", service.Destination{AccountNumber: "1234567890", AccountName: "SYNTHETIC SENDER", BankCode: "999"})
	if e != nil {
		t.Fatal(e)
	}
	q.Client = server.Client()
	q.Client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	q.Now = func() time.Time { return time.Unix(2000000000, 0) }
	return q
}
func TestQPaySigningAndTransferAcceptance(t *testing.T) {
	q := testQPay(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/v1/transfers" || r.Header.Get("X-Application-ID") != "test_app" || r.Header.Get("X-QPay-Environment") != "sandbox" || r.Header.Get("X-API-Key") != "test_key" {
			t.Error("wrong upstream scope")
		}
		expected := security.MAC([]byte(strings.Repeat("s", 32)), "test_app.pay_original.2000000000."+security.Digest(string(body)))
		if r.Header.Get("X-Signature") != expected || r.Header.Get("X-Idempotency-Key") != "pay_original" {
			t.Error("unbound signature")
		}
		var payload map[string]any
		if e := json.Unmarshal(body, &payload); e != nil {
			t.Error(e)
		}
		if payload["amount"] != "10000" || payload["trace_id"] != "pay_original" {
			t.Error("lossy or uncorrelated request")
		}
		io.WriteString(w, `{"reference":"TRF_ORIGINAL","status":"VALIDATED"}`)
	})
	out, e := q.Submit(context.Background(), service.ProviderRequest{Reference: "pay_original", Kind: "bank", Amount: 10000, Currency: "NGN", Narration: "Synthetic"})
	if e != nil || out.Status != "pending" || out.UpstreamID != "TRF_ORIGINAL" {
		t.Fatal(out, e)
	}
}
func TestQPayAmbiguousSubmissionUsesOriginalListAndQueryOnly(t *testing.T) {
	calls := 0
	q := testQPay(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" {
			t.Error("unsafe replacement submission")
		}
		if r.URL.Path == "/v1/transfers" {
			io.WriteString(w, `{"data":[{"reference":"TRF_1","idempotency_key":"pay_original"}]}`)
			return
		}
		if r.URL.Path != "/v1/transfers/TRF_1" {
			t.Error(r.URL.Path)
		}
		io.WriteString(w, `{"reference":"TRF_1","status":"SETTLED","amount":"10000","currency":"NGN"}`)
	})
	out, e := q.Query(context.Background(), service.ProviderRequest{Reference: "pay_original", Kind: "bank", Amount: 10000, Currency: "NGN"})
	if e != nil || out.Status != "succeeded" || calls != 2 {
		t.Fatal(out, e, calls)
	}
}
func TestQPayAbsentOriginalDoesNotResubmit(t *testing.T) {
	calls := 0
	q := testQPay(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" {
			t.Error("unsafe write")
		}
		io.WriteString(w, `{"data":[]}`)
	})
	if _, e := q.Query(context.Background(), service.ProviderRequest{Reference: "pay_original", Kind: "bank"}); e == nil || calls != 1 {
		t.Fatal("missing original must stay unresolved")
	}
}
func TestQPayBillWrapperAndReadOnlyRequery(t *testing.T) {
	q := testQPay(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/bills/payments/BILL_1/requery" {
			t.Error("must query original bill, not vend again")
		}
		io.WriteString(w, `{"data":{"reference":"BILL_1","status":"SUCCESSFUL","amount":10000,"currency":"NGN","result":{"token":"SYNTHETIC TOKEN"}}}`)
	})
	out, e := q.Query(context.Background(), service.ProviderRequest{Reference: "pay_original", UpstreamID: "BILL_1", Kind: "bill", Amount: 10000, Currency: "NGN"})
	if e != nil || !out.Fulfilled || out.Fulfilment != "SYNTHETIC TOKEN" {
		t.Fatal(out, e)
	}
}
func TestQPayQuoteValidatesAmountsAndBillEnvelope(t *testing.T) {
	raw, _ := json.Marshal([]string{"electricity", "test-biller", "test-item"})
	d := service.Destination{ProductID: base64.RawURLEncoding.EncodeToString(raw), CustomerID: "synthetic"}
	for _, v := range []struct {
		body string
		want bool
	}{
		{`{"data":{"amount":"10000","total_debit_amount":"10100","currency":"NGN"}}`, true},
		{`{"data":{"amount":"9999","total_debit_amount":"10100","currency":"NGN"}}`, false},
		{`{"data":{"amount":"10000","total_debit_amount":"9900","currency":"NGN"}}`, false},
		{`{"data":{"amount":"10000","total_debit_amount":"10100","currency":"USD"}}`, false},
		{`{"amount":"10000","total_debit_amount":"10100","currency":"NGN"}`, false},
	} {
		t.Run(v.body, func(t *testing.T) {
			q := testQPay(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, v.body) })
			fee, e := q.Quote(context.Background(), "bill", d, 10000)
			if (e == nil) != v.want || (v.want && fee != 100) {
				t.Fatal(fee, e)
			}
		})
	}
}
func TestQPayRefusesRedirectAndMalformedResponse(t *testing.T) {
	for _, status := range []int{302, 401, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			q := testQPay(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", "https://other.invalid")
				w.WriteHeader(status)
			})
			if _, e := q.Banks(context.Background()); e == nil {
				t.Fatal("non-success accepted")
			}
		})
	}
	q := testQPay(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"banks":[]} {"second":true}`) })
	if _, e := q.Banks(context.Background()); e == nil {
		t.Fatal("trailing JSON accepted")
	}
}
func TestQPayRejectsUnknownFinancialStateAndMismatchedReference(t *testing.T) {
	for _, body := range []string{`{"reference":"TRF_1","status":"REVERSED","amount":10000,"currency":"NGN"}`, `{"reference":"OTHER","status":"SETTLED","amount":10000,"currency":"NGN"}`} {
		q := testQPay(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) })
		if _, e := q.Query(context.Background(), service.ProviderRequest{Reference: "pay_original", UpstreamID: "TRF_1", Kind: "bank"}); e == nil {
			t.Fatal("unqualified result accepted")
		}
	}
}
func TestQPayConfigurationRejectsInsecureOrigin(t *testing.T) {
	for _, base := range []string{"http://example.invalid", "https://user:pass@example.invalid", "https://example.invalid/path", "https://example.invalid?secret=x"} {
		if _, e := NewQPay(base, "app", "key", strings.Repeat("s", 32), "sandbox", service.Destination{}); e == nil {
			t.Fatal(base)
		}
	}
}
