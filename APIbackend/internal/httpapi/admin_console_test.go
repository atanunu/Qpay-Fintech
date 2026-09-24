package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

func adminHTTPLogin(t *testing.T, h *Handler) (map[string]string, []*http.Cookie, string) {
	t.Helper()
	ctx := context.Background()
	h.Config.StaffOrigins = []string{"http://localhost:5174"}
	uid, e := h.Service.BootstrapStaff(ctx, "admin-http@example.invalid", "Synthetic Operator", "Admin-http-password-2026!")
	if e != nil {
		t.Fatal(e)
	}
	input := map[string]string{"email": "admin-http@example.invalid", "password": "Admin-http-password-2026!", "client": "web", "device": "HTTP staff fixture"}
	denied := call(h, "POST", "/v1/admin/auth/login", input, map[string]string{"Origin": "http://localhost:5173"}, nil)
	if denied.Code != 403 {
		t.Fatal("customer origin accepted for staff login", denied.Code)
	}
	login := call(h, "POST", "/v1/admin/auth/login", input, map[string]string{"Origin": "http://localhost:5174"}, nil)
	if login.Code != 200 {
		t.Fatal(login.Body.String())
	}
	session := data(t, login)
	if session["access_token"] != nil || session["refresh_token"] != nil {
		t.Fatal("bearer token disclosed to browser")
	}
	cookies := login.Result().Cookies()
	headers := map[string]string{"Origin": "http://localhost:5174", "X-CSRF-Token": session["csrf_token"].(string)}
	for _, c := range cookies {
		if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || !strings.HasPrefix(c.Name, "qpf_staff_") {
			t.Fatal("unsafe staff cookie")
		}
	}
	blocked := call(h, "GET", "/v1/admin/console/metrics", nil, headers, cookies)
	if blocked.Code != 403 {
		t.Fatal("unenrolled staff reached data", blocked.Code)
	}
	start := call(h, "POST", "/v1/admin/auth/mfa/enrol", map[string]string{"password": input["password"]}, headers, cookies)
	if start.Code != 200 {
		t.Fatal(start.Code, start.Body.String())
	}
	secret := data(t, start)["secret"].(string)
	code, e := security.TOTP(secret, time.Now().Unix()/30)
	if e != nil {
		t.Fatal(e)
	}
	confirm := call(h, "POST", "/v1/admin/auth/mfa/confirm", map[string]string{"code": code}, headers, cookies)
	if confirm.Code != 200 {
		t.Fatal(confirm.Code, confirm.Body.String())
	}
	return headers, cookies, uid
}
func TestAdminHTTPCookieOriginCSRFAndMFAIsolation(t *testing.T) {
	h, _, _ := httpFixture(t)
	headers, cookies, _ := adminHTTPLogin(t, h)
	for _, path := range []string{"/v1/admin/auth/me", "/v1/admin/console/bootstrap", "/v1/admin/console/metrics", "/v1/admin/console/records/customers?limit=1"} {
		w := call(h, "GET", path, nil, headers, cookies)
		if w.Code != 200 {
			t.Fatal(path, w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("private record cached")
		}
	}
	input := map[string]any{"action": "work_create", "resource": "incident", "title": "Synthetic isolation investigation", "reason": "Synthetic staff permission evidence", "severity": "low"}
	if w := call(h, "POST", "/v1/admin/console/actions", input, map[string]string{"Origin": "http://localhost:5174"}, cookies); w.Code != 403 {
		t.Fatal("missing CSRF accepted", w.Code)
	}
	if w := call(h, "POST", "/v1/admin/console/actions", input, map[string]string{"Origin": "http://localhost:5173", "X-CSRF-Token": headers["X-CSRF-Token"]}, cookies); w.Code != 403 {
		t.Fatal("customer origin accepted", w.Code)
	}
	if w := call(h, "GET", "/v1/wallet", nil, nil, cookies); w.Code != 401 {
		t.Fatal("staff cookie reached customer wallet", w.Code)
	}
	customer, _ := httpLogin(t, h, "mobile")
	w := call(h, "GET", "/v1/admin/console/metrics", nil, map[string]string{"Authorization": "Bearer " + customer["access_token"].(string)}, nil)
	if w.Code != 403 {
		t.Fatal("customer token reached console", w.Code)
	}
	w = call(h, "POST", "/v1/admin/console/actions", input, headers, cookies)
	if w.Code != 200 {
		t.Fatal("reasoned operation", w.Code, w.Body.String())
	}
}
func TestAdminHTTPTransactionalRoleAndRecentProof(t *testing.T) {
	h, s, _ := httpFixture(t)
	headers, cookies, uid := adminHTTPLogin(t, h)
	if _, e := s.DB.Exec(`UPDATE sessions SET created_at=now()-interval '11 minutes' WHERE user_id=$1`, uid); e != nil {
		t.Fatal(e)
	}
	w := call(h, "POST", "/v1/admin/console/exports", map[string]string{"resource": "customers", "reason": "Approved synthetic audit report"}, headers, cookies)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "step_up_required") {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, e := s.DB.Exec(`UPDATE users SET role='support',version=version+1 WHERE id=$1`, uid); e != nil {
		t.Fatal(e)
	}
	w = call(h, "GET", "/v1/admin/console/records/staff", nil, headers, cookies)
	if w.Code != 403 {
		t.Fatal("changed role not enforced", w.Code, w.Body.String())
	}
	if _, e := s.DB.Exec(`UPDATE sessions SET revoked=true WHERE user_id=$1`, uid); e != nil {
		t.Fatal(e)
	}
	if w = call(h, "GET", "/v1/admin/console/metrics", nil, headers, cookies); w.Code != 401 {
		t.Fatal("revoked staff read", w.Code)
	}
}
func TestAdminHTTPNonLocalMissingStaffOriginFailsClosed(t *testing.T) {
	h, _, _ := httpFixture(t)
	h.Config.Environment = "production"
	h.Config.StaffOrigins = nil
	w := call(h, "POST", "/v1/admin/auth/login", map[string]string{"email": "missing@example.invalid", "password": "not-an-account", "client": "web"}, map[string]string{"Origin": "http://localhost:5173"}, nil)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
