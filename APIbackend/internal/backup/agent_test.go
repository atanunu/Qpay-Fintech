package backup

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRealAgentContinuesApprovedScheduleDuringAPIOutageWithoutDuplicateRuns(t *testing.T) {
	e, p := engineFixture(t)
	now := time.Now().UTC().Truncate(time.Minute)
	clock := func() time.Time { return now }
	policyPub, policyKey, _ := ed25519.GenerateKey(rand.Reader)
	agentPub, agentKey, _ := ed25519.GenerateKey(rand.Reader)
	p.IssuedAt = now.Add(-time.Minute)
	p.ExpiresAt = now.Add(time.Hour)
	p.Serial = 1
	s := schedule()
	s.Frequency = "interval"
	s.IntervalMinutes = 15
	s.StartsAt = now.Add(-30 * time.Minute)
	p.Resources = append(p.Resources, Resource{ID: "schedule_offline", Version: 1, Kind: "schedules", State: "approved", Environment: "local", AgentID: p.AgentID, Spec: Canonical(s)})
	signed, _ := Sign(p, policyKey)
	var offline atomic.Bool
	var mu sync.Mutex
	received := map[string]bool{}
	nonces := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if offline.Load() {
			w.WriteHeader(503)
			return
		}
		body, _ := io.ReadAll(r.Body)
		auth, err := DecodeAuth(r.Header.Get("X-Backup-Agent"), map[string]ed25519.PublicKey{p.AgentID: agentPub}, r.Method, r.URL.Path, body, clock())
		if err != nil {
			t.Error("agent authentication", err)
			w.WriteHeader(401)
			return
		}
		mu.Lock()
		if nonces[auth.Nonce] {
			mu.Unlock()
			t.Error("nonce replay")
			w.WriteHeader(401)
			return
		}
		nonces[auth.Nonce] = true
		mu.Unlock()
		var data any = map[string]any{"status": "accepted"}
		if r.URL.Path == "/v1/backup-agent/policy" {
			data = signed
		}
		if r.URL.Path == "/v1/backup-agent/results" {
			var signed Signed
			var result Result
			if err := json.Unmarshal(body, &signed); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if err := Verify(signed, agentPub, &result); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if err := AuthorizeResult(p, result); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			mu.Lock()
			received[result.RunID] = true
			mu.Unlock()
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()
	c := e.Config
	c.APIOrigin = server.URL
	keyDir := t.TempDir()
	writeKey := func(name string, key []byte) string {
		file := filepath.Join(keyDir, name)
		if err := os.WriteFile(file, []byte(base64.StdEncoding.EncodeToString(key)), 0600); err != nil {
			t.Fatal(err)
		}
		return file
	}
	c.PrivateKeyFile = writeKey("agent.key", agentKey.Seed())
	c.PolicyPublicKeyFile = writeKey("policy.pub", policyPub)
	c.StateKeyFile = writeKey("journal.key", make([]byte, 32))
	open := func() *Agent {
		a, err := OpenAgent(c)
		if err != nil {
			t.Fatal(err)
		}
		a.Now = clock
		a.Client.Now = clock
		a.Engine.Now = clock
		return a
	}
	a := open()
	if err := a.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	runs, err := a.Journal.Runs()
	if err != nil || len(runs) != 1 || runs[0].Result.State != "succeeded" {
		t.Fatal(runs, err)
	}
	offline.Store(true)
	now = now.Add(15 * time.Minute)
	if err := a.Tick(context.Background()); err != nil {
		t.Fatal("offline lease", err)
	}
	runs, _ = a.Journal.Runs()
	if len(runs) != 2 {
		t.Fatal("offline schedule did not capture", len(runs))
	}
	a.Close()
	a = open()
	defer a.Close()
	if err := a.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	runs, _ = a.Journal.Runs()
	if len(runs) != 2 {
		t.Fatal("restart duplicated a capture")
	}
	offline.Store(false)
	if err := a.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	n := len(received)
	mu.Unlock()
	if n != 2 {
		t.Fatal("durable outbox failed", n)
	}
	now = p.ExpiresAt.Add(time.Second)
	offline.Store(true)
	if err := a.Tick(context.Background()); err == nil {
		t.Fatal("expired offline lease continued")
	}
	runs, _ = a.Journal.Runs()
	if len(runs) != 2 {
		t.Fatal("capture after lease expiry")
	}
}
