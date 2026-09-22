package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/service"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/upstream"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func httpFixture(t *testing.T) (*Handler, *service.Service, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("QPF_REQUIRE_POSTGRES") == "true" {
			t.Fatal("real PostgreSQL required")
		}
		t.Skip("TEST_DATABASE_URL not set")
	}
	admin, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	schema := "http_" + security.Digest(security.Random("", 16))[:20]
	if _, e = admin.Exec(`CREATE SCHEMA ` + schema); e != nil {
		t.Fatal(e)
	}
	u, e := url.Parse(dsn)
	if e != nil {
		t.Fatal(e)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, e := sql.Open("pgx", u.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close(); admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`); admin.Close() })
	if e = service.Migrate(context.Background(), db); e != nil {
		t.Fatal(e)
	}
	key := bytes.Repeat([]byte{7}, 32)
	s := service.New(db, service.Config{Environment: "local", Pepper: key, Box: security.Box{Active: "test", Keys: map[string][]byte{"test": key}}, NotificationMode: "local", Gateway: &upstream.Local{DB: db}, ExternalEnabled: true})
	id, e := s.SeedLocal(context.Background(), "http@example.invalid", "Synthetic HTTP", "test-password-123", "123456")
	if e != nil {
		t.Fatal(e)
	}
	h := New(s, Config{Environment: "local", Origins: []string{"http://localhost:5173"}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), PolicyKey: key})
	return h, s, id
}
func call(h http.Handler, method, path string, body any, headers map[string]string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	raw := []byte(nil)
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func data(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out struct {
		Data map[string]any `json:"data"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e, w.Body.String())
	}
	return out.Data
}
func httpLogin(t *testing.T, h *Handler, client string) (map[string]any, []*http.Cookie) {
	t.Helper()
	w := call(h, "POST", "/v1/auth/login", map[string]string{"email": "http@example.invalid", "password": "test-password-123", "client": client, "device": "test"}, map[string]string{"Origin": "http://localhost:5173"}, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	return data(t, w), w.Result().Cookies()
}
func TestMobileHTTPJourneyAndOwnership(t *testing.T) {
	h, s, _ := httpFixture(t)
	login, _ := httpLogin(t, h, "mobile")
	headers := map[string]string{"Authorization": "Bearer " + login["access_token"].(string)}
	wallet := call(h, "GET", "/v1/wallet", nil, headers, nil)
	if wallet.Code != 200 || data(t, wallet)["balance_minor"] != "10000000" {
		t.Fatal(wallet.Body.String())
	}
	recipient, e := s.SeedLocal(context.Background(), "recipient@example.invalid", "Recipient", "test-password-123", "123456")
	if e != nil {
		t.Fatal(e)
	}
	quote := call(h, "POST", "/v1/quotes", map[string]string{"kind": "internal", "amount_minor": "10000", "currency": "NGN", "recipient_id": recipient, "narration": "Synthetic HTTP transfer"}, headers, nil)
	if quote.Code != 201 {
		t.Fatal(quote.Code, quote.Body.String())
	}
	qid := data(t, quote)["id"].(string)
	auth := call(h, "POST", "/v1/quotes/"+qid+"/authorisations", map[string]string{"pin": "123456"}, headers, nil)
	if auth.Code != 201 {
		t.Fatal(auth.Body.String())
	}
	input := map[string]string{"quote_id": qid, "authorisation_token": data(t, auth)["authorisation_token"].(string)}
	headers["Idempotency-Key"] = "synthetic-http-idem-key"
	paid := call(h, "POST", "/v1/payments", input, headers, nil)
	if paid.Code != 202 || data(t, paid)["status"] != "succeeded" {
		t.Fatal(paid.Code, paid.Body.String())
	}
	replay := call(h, "POST", "/v1/payments", input, headers, nil)
	if replay.Code != 202 || data(t, replay)["id"] != data(t, paid)["id"] {
		t.Fatal("unsafe replay", replay.Body.String())
	}
	if call(h, "GET", "/v1/admin/overview", nil, headers, nil).Code != 403 {
		t.Fatal("customer crossed staff boundary")
	}
	unknown := call(h, "GET", "/v1/payments/pay_missing", nil, headers, nil)
	if unknown.Code != 404 {
		t.Fatal(unknown.Code)
	}
	if wallet.Header().Get("Cache-Control") != "no-store" || wallet.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing privacy/trace headers")
	}
}
func TestBrowserHTTPUsesCookiesAndCSRF(t *testing.T) {
	h, _, _ := httpFixture(t)
	login, cookies := httpLogin(t, h, "web")
	if login["access_token"] != nil || login["refresh_token"] != nil || len(cookies) != 2 {
		t.Fatal("web exposed bearer credentials")
	}
	for _, c := range cookies {
		if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
			t.Fatal("unsafe cookie")
		}
	}
	proof := map[string]string{"Origin": "http://localhost:5173", "X-CSRF-Token": login["csrf_token"].(string)}
	if call(h, "PATCH", "/v1/me", map[string]string{"name": "Changed"}, map[string]string{"Origin": "http://localhost:5173"}, cookies).Code != 403 {
		t.Fatal("missing CSRF accepted")
	}
	changed := call(h, "PATCH", "/v1/me", map[string]string{"name": "Changed"}, proof, cookies)
	if changed.Code != 200 {
		t.Fatal(changed.Code, changed.Body.String())
	}
	restored := call(h, "GET", "/v1/auth/csrf", nil, proof, cookies)
	if restored.Code != 200 || data(t, restored)["csrf_token"] != login["csrf_token"] {
		t.Fatal("reload recovery failed")
	}
	if call(h, "GET", "/v1/wallet", nil, map[string]string{"Authorization": "Bearer " + cookies[0].Value}, nil).Code != 403 {
		t.Fatal("web token accepted as mobile bearer")
	}
	refreshed := call(h, "POST", "/v1/auth/refresh", map[string]string{"client": "web"}, proof, cookies)
	if refreshed.Code != 200 {
		t.Fatal(refreshed.Body.String())
	}
	reused := call(h, "POST", "/v1/auth/refresh", map[string]string{"client": "web"}, proof, cookies)
	if reused.Code != 401 {
		t.Fatal("old refresh accepted")
	}
	if call(h, "GET", "/v1/wallet", nil, proof, refreshed.Result().Cookies()).Code != 401 {
		t.Fatal("replayed family still active")
	}
}
func TestBodyAndContractValidation(t *testing.T) {
	for _, body := range []string{`{"email":"one","email":"two"}`, `{"email":"one"} {}`, `{"unknown":true}`, strings.Repeat("[", 30) + strings.Repeat("]", 30)} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		var in RegisterInput
		if e := bind(r, &in); e == nil {
			t.Fatal("unsafe body accepted", body)
		}
	}
	h := New(&service.Service{}, Config{})
	spec := h.OpenAPI()
	if spec["openapi"] != "3.1.0" {
		t.Fatal("missing OpenAPI")
	}
	ids := map[string]bool{}
	for _, r := range h.Routes {
		key := r.Method + " " + r.Path
		if ids[key] {
			t.Fatal("duplicate route", key)
		}
		ids[key] = true
	}
	if len(ids) < 60 {
		t.Fatal("incomplete route inventory")
	}
	if got := call(h, "GET", "/health/live", nil, map[string]string{"Origin": "https://untrusted.invalid"}, nil); got.Code != 403 {
		t.Fatal("untrusted origin accepted")
	}
}
func TestStaffEnrolmentDoesNotGrantOperations(t *testing.T) {
	h, s, _ := httpFixture(t)
	_, e := s.BootstrapStaff(context.Background(), "admin@example.invalid", "Admin", "test-password-123")
	if e != nil {
		t.Fatal(e)
	}
	w := call(h, "POST", "/v1/admin/auth/login", map[string]string{"email": "admin@example.invalid", "password": "test-password-123", "client": "web", "device": "test"}, map[string]string{"Origin": "http://localhost:5173"}, nil)
	if w.Code != 200 || data(t, w)["mfa_enrolment_required"] != true {
		t.Fatal(w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if got := call(h, "GET", "/v1/admin/overview", nil, nil, cookies); got.Code != 403 {
		t.Fatal("un-enrolled staff gained operations", got.Code)
	}
	if got := call(h, "GET", "/v1/admin/auth/me", nil, nil, cookies); got.Code != 200 {
		t.Fatal("enrolment session cannot read itself", got.Code)
	}
}
