package backup

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func instant(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
func schedule() ScheduleSpec {
	return ScheduleSpec{Plan: Ref{"plan_test", 1}, Operation: "backup", Timezone: "Africa/Lagos", Frequency: "daily", At: "02:00", StartsAt: instant("2026-01-01T00:00:00Z"), Missed: "catch_up_once"}
}
func TestScheduleWallClockAndDST(t *testing.T) {
	cases := []struct{ name, zone, at, after, want string }{{"Lagos", "Africa/Lagos", "02:00", "2026-09-24T00:59:59Z", "2026-09-24T01:00:00Z"}, {"spring gap", "Europe/Helsinki", "03:30", "2026-03-28T23:00:00Z", "2026-03-30T00:30:00Z"}, {"fall first", "Europe/Helsinki", "03:30", "2026-10-24T23:00:00Z", "2026-10-25T00:30:00Z"}, {"fall never twice", "Europe/Helsinki", "03:30", "2026-10-25T00:30:00Z", "2026-10-26T01:30:00Z"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := schedule()
			s.Timezone = c.zone
			s.At = c.at
			n, err := Next(s, instant(c.after), 1)
			if err != nil || len(n) != 1 || !n[0].Equal(instant(c.want)) {
				t.Fatalf("%v %v", n, err)
			}
		})
	}
}
func TestScheduleMonthlyIntervalAndCatchup(t *testing.T) {
	s := schedule()
	s.Frequency = "monthly"
	s.MonthDay = -1
	n, err := Next(s, instant("2026-02-01T00:00:00Z"), 2)
	if err != nil || !n[0].Equal(instant("2026-02-28T01:00:00Z")) {
		t.Fatal(n, err)
	}
	s.Frequency = "interval"
	s.IntervalMinutes = 15
	s.StartsAt = instant("2026-09-24T00:00:00Z")
	now := instant("2026-09-24T04:01:00Z")
	due, ok, err := Due(s, s.StartsAt, now)
	if err != nil || !ok || !due.Equal(instant("2026-09-24T04:00:00Z")) {
		t.Fatal(due, ok, err)
	}
	s.Missed = "skip"
	due, ok, err = Due(s, s.StartsAt, now)
	if err != nil || !ok {
		t.Fatal("skip should run current timely occurrence", due, err)
	}
	_, ok, _ = Due(s, s.StartsAt, now.Add(6*time.Minute))
	if ok {
		t.Fatal("late skip executed")
	}
	_, _, err = Due(s, now.Add(time.Hour), now)
	if err == nil {
		t.Fatal("clock rollback accepted")
	}
}
func TestConfigurationRejectsUnsafeAndUnboundedFields(t *testing.T) {
	for _, raw := range []string{`{"profile":"approved","type":"files","classification":"restricted","rpo_seconds":60,"command":"curl attacker"}`, `{"profile":"../../etc","type":"files","classification":"restricted","rpo_seconds":60}`, `{"profile":"approved","type":"raw_database","classification":"restricted","rpo_seconds":60}`} {
		if _, err := ValidateSpec("sources", []byte(raw)); err == nil {
			t.Fatal(raw)
		}
	}
	s := schedule()
	s.Timezone = "arbitrary"
	if ValidateSchedule(s) == nil {
		t.Fatal("invalid timezone")
	}
	s = schedule()
	s.Frequency = "interval"
	s.IntervalMinutes = 0
	if ValidateSchedule(s) == nil {
		t.Fatal("zero interval")
	}
	if _, err := ValidateSpec("retention", Canonical(RetentionSpec{KeepLast: 1, MinimumDays: 1})); err == nil {
		t.Fatal("last good backup unprotected")
	}
}
func TestSignaturesBindPolicyAndHTTPRequest(t *testing.T) {
	pub, private, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now()
	p := Policy{Version: 1, Serial: 4, AgentID: "agent_one", Environment: "local", IssuedAt: now, ExpiresAt: now.Add(time.Hour)}
	s, _ := Sign(p, private)
	if _, err := VerifyPolicy(s, pub, p.AgentID, p.Environment, now, 4); err != nil {
		t.Fatal(err)
	}
	for _, x := range []struct {
		id, env string
		serial  int64
		now     time.Time
	}{{"agent_two", "local", 4, now}, {p.AgentID, "production", 4, now}, {p.AgentID, "local", 5, now}, {p.AgentID, "local", 4, now.Add(2 * time.Hour)}} {
		if _, err := VerifyPolicy(s, pub, x.id, x.env, x.now, x.serial); err == nil {
			t.Fatal("unbound policy")
		}
	}
	auth := Authentication{p.AgentID, "POST", "/v1/backup-agent/results", now, "nonce_123", Digest([]byte("one"))}
	h, _ := EncodeAuth(auth, private)
	if _, err := DecodeAuth(h, map[string]ed25519.PublicKey{p.AgentID: pub}, auth.Method, auth.Path, []byte("one"), now); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeAuth(h, map[string]ed25519.PublicKey{p.AgentID: pub}, auth.Method, auth.Path, []byte("two"), now); err == nil {
		t.Fatal("modified body")
	}
	s.Payload[1] ^= 1
	if Verify(s, pub, &p) == nil {
		t.Fatal("corrupt signature")
	}
}
func TestJournalEncryptionLockAndCrashRecovery(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	key := make([]byte, 32)
	rand.Read(key)
	j, err := OpenJournal(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenJournal(dir, key); err == nil {
		t.Fatal("second writer acquired lock")
	}
	_, signer, _ := ed25519.GenerateKey(rand.Reader)
	entry := JournalRun{Result: Result{RunID: "br_synthetic", StartedAt: time.Now(), State: "unknown", Files: []File{{Path: "private-customer-filename.txt"}}}}
	if err = j.Write(entry.Result.RunID, entry); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "br_synthetic.enc"))
	if strings.Contains(string(raw), "private-customer") {
		t.Fatal("plaintext journal leak")
	}
	if err = j.RecoverInterrupted(signer, time.Now()); err != nil {
		t.Fatal(err)
	}
	var got JournalRun
	if err = j.Read(entry.Result.RunID, &got); err != nil || got.Result.State != "unknown" || !got.Complete {
		t.Fatal(got, err)
	}
	if err = Verify(got.Signed, signer.Public().(ed25519.PublicKey), &Result{}); err != nil {
		t.Fatal(err)
	}
	j.Close()
	key[0] ^= 1
	j, err = OpenJournal(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if j.Read("br_synthetic", &got) == nil {
		t.Fatal("wrong key accepted")
	}
}
func TestFileScopeSymlinkAndExclusions(t *testing.T) {
	src := t.TempDir()
	target := t.TempDir()
	os.WriteFile(filepath.Join(src, "note.txt"), []byte("data"), 0600)
	os.WriteFile(filepath.Join(src, ".env.local"), []byte("secret"), 0600)
	budget := int64(1000)
	files, err := copyFiles(src, target, "source_test", &budget)
	if err != nil || len(files) != 1 || files[0].SHA256 != Digest([]byte("data")) {
		t.Fatal(files, err)
	}
	os.Symlink("/etc/passwd", filepath.Join(src, "escape"))
	if _, err = copyFiles(src, t.TempDir(), "source_test", &budget); err == nil {
		t.Fatal("symlink accepted")
	}
	os.Remove(filepath.Join(src, "escape"))
	os.WriteFile(filepath.Join(src, "PG_VERSION"), []byte("16"), 0600)
	if _, err = copyFiles(src, t.TempDir(), "source_test", &budget); err == nil {
		t.Fatal("raw database accepted")
	}
}
func TestAgentOriginAndEnvironmentInjection(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	for _, url := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com/path", "https://example.com?x=1"} {
		if _, err := NewClient(AgentConfig{APIOrigin: url, Environment: "production"}, key); err == nil {
			t.Fatal(url)
		}
	}
	p := filepath.Join(t.TempDir(), "env.json")
	os.WriteFile(p, []byte(`{"LD_PRELOAD":"/tmp/malware"}`), 0600)
	if _, err := ReadEnvironment(p); err == nil {
		t.Fatal("process injection")
	}
	os.WriteFile(p, []byte(`{"PGUSER":"qpf"}`), 0600)
	os.Chmod(p, 0644)
	if _, err := ReadEnvironment(p); err == nil {
		t.Fatal("readable credentials")
	}
}
func TestCommandResultCannotInventCopyOrApproval(t *testing.T) {
	p := testPolicy()
	now := time.Now()
	cmd := Command{ID: "command_one", Operation: "backup", Resource: Ref{"plan_test", 1}, Approved: true, ExpiresAt: now.Add(time.Hour)}
	p.Commands = []Command{cmd}
	r := NewCommandResult(p, Digest(Canonical(p)), cmd, now)
	r.FinishedAt = now
	r.State = "failed"
	if err := AuthorizeResult(p, r); err != nil {
		t.Fatal(err)
	}
	r.CommandID = "unapproved"
	if AuthorizeResult(p, r) == nil {
		t.Fatal("unapproved command")
	}
	r.CommandID = cmd.ID
	r.Copies = []Copy{{ID: "cp_test", Source: Ref{"source_wrong", 1}, Destination: Ref{"destination_test", 1}}}
	if AuthorizeResult(p, r) == nil {
		t.Fatal("unrelated source")
	}
	if RequiredComplete(p, Result{Plan: r.Plan}) {
		t.Fatal("empty copies reported protected")
	}
}
func testPolicy() Policy {
	now := time.Now()
	resource := func(id, kind string, spec any) Resource {
		return Resource{ID: id, Version: 1, Kind: kind, State: "approved", Environment: "local", AgentID: "agent_test", Spec: Canonical(spec)}
	}
	return Policy{Version: 1, Serial: 1, AgentID: "agent_test", Environment: "local", IssuedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Hour), Resources: []Resource{resource("source_test", "sources", SourceSpec{Profile: "files_test", Type: "files", Classification: "restricted", RPOSeconds: 3600}), resource("destination_test", "destinations", DestinationSpec{Profile: "repo_test", Provider: "local", FailureDomain: "test", Region: "local"}), resource("retention_test", "retention", RetentionSpec{KeepLast: 2, MinimumDays: 1}), resource("plan_test", "plans", PlanSpec{Sources: []Ref{{"source_test", 1}}, Destinations: []CopyBinding{{Ref{"destination_test", 1}, true}}, Retention: Ref{"retention_test", 1}, Verify: "read_data", MinFailureDomains: 1, TimeoutSeconds: 60})}}
}
func TestDecodeRejectsTrailing(t *testing.T) {
	var v SourceSpec
	if Decode([]byte(`{} {}`), &v) == nil {
		t.Fatal("multiple objects")
	}
	if json.Valid(Canonical(v)) != true {
		t.Fatal("invalid output")
	}
}
func TestRunnerRejectsUnprovisionedExecutable(t *testing.T) {
	if _, err := (OSRunner{}).Run(context.Background(), Process{Tool: "sh", Args: []string{"-c", "true"}}); err == nil {
		t.Fatal("shell enabled")
	}
}
