package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// The companion Backups/tests/physical_recovery.py creates an isolated cluster,
// archives WAL to two encrypted repositories, and starts the restored copy only
// after this engine test has materialized it. No financial process is started.
func TestRealPhysicalCaptureVerifyAndPITRMaterialization(t *testing.T) {
	config := os.Getenv("BACKUP_PHYSICAL_CONFIG")
	if config == "" {
		if os.Getenv("BACKUP_REQUIRE_PHYSICAL") == "true" {
			t.Fatal("physical fixture required")
		}
		t.Skip("run the isolated physical recovery harness")
	}
	e, p := engineFixture(t)
	ctx := context.Background()
	bin, err := exec.LookPath("pgbackrest")
	if err != nil {
		t.Fatal(err)
	}
	e.Config.Executables["pgbackrest"] = bin
	e.Runner = OSRunner{e.Config.Executables}
	source := SourceProfile{Type: "postgres_physical", PgBackRestConfig: config, Stanza: "synthetic"}
	e.Config.Sources["files_test"] = source
	p.Resources[0].Spec = Canonical(SourceSpec{Profile: "files_test", Type: "postgres_physical", Classification: "restricted", RPOSeconds: 300})
	d := DestinationProfile{Provider: "pgbackrest", PgRepository: 1, PgBackRestConfig: config, Stanza: "synthetic", FailureDomain: "test", Region: "local"}
	e.Config.Destinations["repo_test"] = d
	p.Resources[1].Spec = Canonical(DestinationSpec{Profile: "repo_test", Provider: "pgbackrest", FailureDomain: "test", Region: "local"})
	d2 := d
	d2.PgRepository = 2
	d2.FailureDomain = "independent"
	e.Config.Destinations["repo_second"] = d2
	second := p.Resources[1]
	second.ID = "destination_second"
	second.Spec = Canonical(DestinationSpec{Profile: "repo_second", Provider: "pgbackrest", FailureDomain: "independent", Region: "local"})
	p.Resources = append(p.Resources, second)
	plan := Parse[PlanSpec](p.Resources[3])
	plan.Verify = "metadata"
	plan.MinFailureDomains = 2
	plan.Destinations = append(plan.Destinations, CopyBinding{second.Ref(), true})
	p.Resources[3].Spec = Canonical(plan)
	db, err := sql.Open("pgx", os.Getenv("BACKUP_PHYSICAL_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE physical_ledger_test(id int PRIMARY KEY,value bigint);INSERT INTO physical_ledger_test VALUES(1,400),(2,-400)`); err != nil {
		t.Fatal(err)
	}
	if err = e.probe(ctx, p.Resources[0], "br_physical_probe"); err != nil {
		t.Fatal("source probe", err)
	}
	var full Result
	for i, mode := range []string{"full", "diff", "incr"} {
		cmd := Command{ID: "command_physical_" + mode, Operation: mode, Resource: Ref{"plan_test", 1}, Approved: true, ExpiresAt: time.Now().Add(time.Hour)}
		p.Commands = append(p.Commands, cmd)
		out := e.Execute(ctx, p, NewCommandResult(p, Digest(Canonical(p)), cmd, time.Now()), &cmd, nil)
		if out.State != "succeeded" || len(out.Copies) != 2 {
			t.Fatal(mode, out)
		}
		if i == 0 {
			full = out
		}
		if err = AuthorizeResult(p, out); err != nil {
			t.Fatal(err)
		}
	}
	var target time.Time
	if err = db.QueryRow(`SELECT clock_timestamp()`).Scan(&target); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, err = db.Exec(`INSERT INTO physical_ledger_test VALUES(3,999);SELECT pg_switch_wal()`); err != nil {
		t.Fatal(err)
	}
	if err = e.probe(ctx, p.Resources[0], "br_physical_archive"); err != nil {
		t.Fatal(err)
	}
	for _, c := range full.Copies {
		cmd := Command{ID: "command_verify_physical_" + strconv.Itoa(len(c.ID)), Operation: "verify", Resource: full.Plan, PointID: full.RunID, CopyID: c.ID, Approved: true}
		if err = e.verifyPoint(ctx, p, cmd, full); err != nil {
			t.Fatal("physical verify", err)
		}
	}
	e.Config.RestoreRoots["isolated_test"] = os.Getenv("BACKUP_PHYSICAL_RESTORE")
	cmd := Command{ID: "command_pitr_isolated", Operation: "restore", Resource: full.Plan, PointID: full.RunID, CopyID: full.Copies[0].ID, Target: "isolated_test", RecoveryTime: &target, Approved: true, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	p.Commands = append(p.Commands, cmd)
	out := e.Execute(ctx, p, NewCommandResult(p, Digest(Canonical(p)), cmd, time.Now()), &cmd, &full)
	if out.State != "materialized" {
		t.Fatal("physical materialization", out)
	}
	if err = AuthorizeResult(p, out); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(e.Config.RestoreRoots[cmd.Target], cmd.ID)
	raw, err := os.ReadFile(filepath.Join(targetDir, "postgresql.auto.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("missing recovery configuration")
	}
	receipt := map[string]any{"target": targetDir, "recovery_time": target, "copies": 2, "engines": []string{"full", "diff", "incr", "verify", "restore"}}
	b, _ := json.MarshalIndent(receipt, "", "  ")
	if err = os.WriteFile(os.Getenv("BACKUP_PHYSICAL_RECEIPT"), b, 0600); err != nil {
		t.Fatal(err)
	}
}
