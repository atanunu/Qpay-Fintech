package backup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func engineFixture(t *testing.T) (*Engine, Policy) {
	t.Helper()
	tool, err := exec.LookPath("restic")
	if err != nil {
		if os.Getenv("BACKUP_REQUIRE_ENGINES") == "true" {
			t.Fatal("restic required")
		}
		t.Skip("restic engine not available; set BACKUP_REQUIRE_ENGINES=true in acceptance")
	}
	dir := t.TempDir()
	password := filepath.Join(dir, "repo.key")
	os.WriteFile(password, []byte("synthetic-repository-test-password-only"), 0600)
	source := filepath.Join(dir, "source")
	os.Mkdir(source, 0700)
	os.WriteFile(filepath.Join(source, "record.txt"), []byte("Synthetic recoverable evidence"), 0600)
	scratch := filepath.Join(dir, "scratch")
	os.Mkdir(scratch, 0700)
	restore := filepath.Join(dir, "restore")
	os.Mkdir(restore, 0700)
	c := AgentConfig{ID: "agent_test", InstanceID: "instance_test", Environment: "local", TimeoutSeconds: 60, MaxStageBytes: 10 << 20, StateDir: filepath.Join(dir, "state"), ScratchDir: scratch, Stage: DestinationProfile{Provider: "local", Repository: filepath.Join(dir, "stage"), PasswordFile: password}, Sources: map[string]SourceProfile{"files_test": {Type: "files", Root: source}}, Destinations: map[string]DestinationProfile{"repo_test": {Provider: "local", Repository: filepath.Join(dir, "offsite"), PasswordFile: password, FailureDomain: "test", Region: "local"}}, RestoreRoots: map[string]string{"isolated_test": restore}, Executables: map[string]string{"restic": tool}}
	e, err := NewEngine(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"stage", "repo_test"} {
		if err = e.Initialize(context.Background(), p); err != nil {
			t.Fatal("repository init", err)
		}
	}
	return e, testPolicy()
}
func TestRealEncryptedSnapshotCopyVerifyAndRestore(t *testing.T) {
	e, p := engineFixture(t)
	ctx := context.Background()
	cmd := Command{ID: "command_capture", Operation: "backup", Resource: Ref{"plan_test", 1}, Approved: true, ExpiresAt: time.Now().Add(time.Hour)}
	p.Commands = []Command{cmd}
	r := e.Execute(ctx, p, NewCommandResult(p, Digest(Canonical(p)), cmd, time.Now()), &cmd, nil)
	if r.State != "succeeded" {
		t.Fatalf("capture: %+v", r)
	}
	if err := ValidateResult(r); err != nil {
		t.Fatal(err)
	}
	if err := AuthorizeResult(p, r); err != nil {
		t.Fatal(err)
	}
	if !RequiredComplete(p, r) || len(r.Files) != 1 || r.Files[0].Path != "files/record.txt" {
		t.Fatal("inventory/completion", r)
	}
	os.RemoveAll(e.Config.Sources["files_test"].Root) // Prove recovery does not read the original source.
	restore := Command{ID: "command_restore", Operation: "restore", Resource: r.Plan, Approved: true, PointID: r.RunID, CopyID: r.Copies[0].ID, Target: "isolated_test", ExpiresAt: time.Now().Add(time.Hour)}
	p.Commands = append(p.Commands, restore)
	restored := e.Execute(ctx, p, NewCommandResult(p, Digest(Canonical(p)), restore, time.Now()), &restore, &r)
	if restored.State != "materialized" {
		t.Fatal("restore", restored)
	}
	data, err := os.ReadFile(filepath.Join(e.Config.RestoreRoots[restore.Target], restore.ID, "files", "record.txt"))
	if err != nil || string(data) != "Synthetic recoverable evidence" {
		t.Fatal(string(data), err)
	}
	if _, err = e.materialize(ctx, p, restore, r); err == nil {
		t.Fatal("overwrote restore target")
	}
	verify := restore
	verify.ID = "command_verify"
	verify.Operation = "verify"
	verified := e.Execute(ctx, p, NewCommandResult(p, Digest(Canonical(p)), verify, time.Now()), &verify, &r)
	if verified.State != "succeeded" {
		t.Fatal(verified)
	}
	export := restore
	export.ID = "command_export"
	export.Operation = "export"
	result := e.Execute(ctx, p, NewCommandResult(p, Digest(Canonical(p)), export, time.Now()), &export, &r)
	if result.State != "materialized" {
		t.Fatal("encrypted export", result)
	}
	leaked := false
	filepath.WalkDir(e.Config.Destinations["repo_test"].Repository, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			b, _ := os.ReadFile(path)
			leaked = leaked || strings.Contains(string(b), "Synthetic recoverable evidence")
		}
		return nil
	})
	if leaked {
		t.Fatal("repository plaintext leak")
	}
}
func TestRealDestinationProbeAndMissingRequiredCopy(t *testing.T) {
	e, p := engineFixture(t)
	ctx := context.Background()
	d, _ := Find(p, Ref{"destination_test", 1})
	if err := e.probe(ctx, d, "br_probe_test"); err != nil {
		t.Fatal("readback probe", err)
	}
	bad := e.Config.Destinations["repo_test"]
	bad.Repository = filepath.Join(t.TempDir(), "not-initialized")
	e.Config.Destinations["repo_test"] = bad
	cmd := Command{ID: "command_capture", Operation: "backup", Resource: Ref{"plan_test", 1}, Approved: true, ExpiresAt: time.Now().Add(time.Hour)}
	r := e.Execute(ctx, p, NewCommandResult(p, Digest(Canonical(p)), cmd, time.Now()), &cmd, nil)
	if r.State == "succeeded" || RequiredComplete(p, r) {
		t.Fatal("missing destination became successful")
	}
	if _, err := os.Stat(bad.Repository); err == nil {
		t.Fatal("initialized destination on failure")
	}
}
