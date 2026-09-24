package httpapi

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/backup"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBackupStaffRoutesRequireSeparateOriginCSRFAndFreshProof(t *testing.T) {
	h, _, _ := httpFixture(t)
	headers, cookies, _ := adminHTTPLogin(t, h)
	for _, path := range []string{"/v1/admin/backups/bootstrap", "/v1/admin/backups/overview", "/v1/admin/backups/resources/sources?limit=5"} {
		resp := call(h, "GET", path, nil, headers, cookies)
		if resp.Code != 200 {
			t.Fatal(path, resp.Code, resp.Body.String())
		}
	}
	in := map[string]any{"name": "Synthetic retention", "environment": "local", "agent_id": "", "base_version": 0, "spec": map[string]int{"keep_last": 2, "minimum_days": 1}, "reason": "Define synthetic recovery retention", "submit": false}
	denied := call(h, "POST", "/v1/admin/backups/resources/retention", in, map[string]string{"Origin": "http://localhost:5174"}, cookies)
	if denied.Code != 403 {
		t.Fatal("CSRF missing accepted", denied.Code)
	}
	made := call(h, "POST", "/v1/admin/backups/resources/retention", in, headers, cookies)
	if made.Code != 201 {
		t.Fatal(made.Code, made.Body.String())
	}
}
func TestBackupAgentCannotUseBrowserIdentityAndRejectsSignedReplay(t *testing.T) {
	h, _, _ := httpFixture(t)
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	_, policykey, _ := ed25519.GenerateKey(rand.Reader)
	h.Service.Config.Backups = &backup.ControlConfig{LeaseSeconds: 3600, SigningKey: policykey, Agents: []backup.RegisteredAgent{{ID: "agent_http", Environment: "local", PublicKey: pub}}}
	path := "/v1/backup-agent/policy"
	header, _ := backup.EncodeAuth(backup.Authentication{AgentID: "agent_http", Method: "GET", Path: path, Timestamp: time.Now(), Nonce: "nonce_http_one", BodyDigest: backup.Digest(nil)}, key)
	makeReq := func(origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, strings.NewReader(""))
		r.Header.Set("X-Backup-Agent", header)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := makeReq("http://localhost:5173"); w.Code != 403 {
		t.Fatal("agent browser accepted", w.Code)
	}
	w := makeReq("")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var body struct {Data backup.Signed `json:"data"`}
 if json.Unmarshal(w.Body.Bytes(),&body)!=nil{t.Fatal("bad response")}
 envelope:=body.Data
 if len(envelope.Payload)==0 {
		t.Fatal("bad signed policy")
	}
	if _, err := backup.VerifyPolicy(envelope, policykey.Public().(ed25519.PublicKey), "agent_http", "local", time.Now(), 0); err != nil {
		t.Fatal(err)
	}
	if w := makeReq(""); w.Code != 401 {
		t.Fatal("replay accepted", w.Code)
	}
}
