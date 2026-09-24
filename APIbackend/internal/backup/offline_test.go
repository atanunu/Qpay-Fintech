package backup

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRealOfflineRecoveryWithoutControlAPI(t *testing.T) {
	e, p := engineFixture(t)
	ctx := context.Background()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	agentPub, agentKey, _ := ed25519.GenerateKey(rand.Reader)
	cmd := Command{ID: "command_offline_capture", Operation: "backup", Resource: Ref{"plan_test", 1}, Approved: true, ExpiresAt: time.Now().Add(time.Hour)}
	p.Commands = []Command{cmd}
	policy, _ := Sign(p, priv)
	result := e.Execute(ctx, p, NewCommandResult(p, Digest(policy.Payload), cmd, time.Now()), &cmd, nil)
	if result.State != "succeeded" {
		t.Fatal(result)
	}
	signed, _ := Sign(result, agentKey)
	entry := JournalRun{Result: result, Signed: signed, Policy: policy, Complete: true}
	b, err := NewOfflineBundle(entry, "offline_restore_test", result.Copies[0].ID, "isolated_test", "restore", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c1, k1, _ := ed25519.GenerateKey(rand.Reader)
	c2, k2, _ := ed25519.GenerateKey(rand.Reader)
	keys := map[string]ed25519.PublicKey{"custodian_one": c1, "custodian_two": c2}
	b, _ = ApproveOffline(b, "custodian_one", k1)
	if _, err = e.OfflineRecover(ctx, b, pub, agentPub, keys); err == nil {
		t.Fatal("single custodian authorized recovery")
	}
	b, _ = ApproveOffline(b, "custodian_two", k2)
	changed := b
	changed.Request.Target = "production"
	if _, err = e.OfflineRecover(ctx, changed, pub, agentPub, keys); err == nil {
		t.Fatal("changed target accepted")
	}
	os.RemoveAll(e.Config.Sources["files_test"].Root)
	e.Config.APIOrigin = "https://unavailable.example.invalid"
	if _, err = e.OfflineRecover(ctx, b, pub, agentPub, keys); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(e.Config.RestoreRoots["isolated_test"], b.Request.ID, "files/record.txt"))
	if err != nil || string(data) != "Synthetic recoverable evidence" {
		t.Fatal("offline restored data", err)
	}
	if _, err = e.OfflineRecover(ctx, b, pub, agentPub, keys); err == nil {
		t.Fatal("overwrote previous recovery target")
	}
	e.Now = func() time.Time { return b.Request.ExpiresAt.Add(time.Second) }
	if _, err = e.OfflineRecover(ctx, b, pub, agentPub, keys); err == nil {
		t.Fatal("expired approval accepted")
	}
}
func TestIndependentHealthMonitorRejectsStaleOrForgedEvidence(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	h := Health{Version: 1, AgentID: "agent_test", ObservedAt: now, PolicyExpiresAt: now.Add(time.Hour), LastSuccessfulCapture: now, Stage: "idle"}
	if err := WriteHealth(dir, h, key); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "health.json")
	if _, err := CheckHealth(file, pub, now, time.Minute, time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckHealth(file, pub, now.Add(2*time.Minute), time.Minute, time.Hour); err == nil {
		t.Fatal("stale heartbeat accepted")
	}
	bad, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := CheckHealth(file, bad, now, time.Minute, time.Hour); err == nil {
		t.Fatal("forged health accepted")
	}
	h.LastSuccessfulCapture = now.Add(-48 * time.Hour)
	WriteHealth(dir, h, key)
	if _, err := CheckHealth(file, pub, now, time.Minute, time.Hour); err == nil {
		t.Fatal("stale capture accepted")
	}
}
