package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRealPostgresDatabaseAndImmutableObjectsRestoredAfterLoss(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("QPF_REQUIRE_POSTGRES") == "true" {
			t.Fatal("PostgreSQL required")
		}
		t.Skip("PostgreSQL integration requires TEST_DATABASE_URL")
	}
	e, p := engineFixture(t)
	for _, name := range []string{"pg_dump", "pg_dumpall", "pg_restore"} {
		path, err := exec.LookPath(name)
		if err != nil {
			t.Fatal("PostgreSQL tool required", name)
		}
		e.Config.Executables[name] = path
	}
	e.Runner = OSRunner{e.Config.Executables}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("backup_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`)
		admin.Exec(`DROP DATABASE IF EXISTS ` + name + `_restored WITH (FORCE)`)
	})
	u.Path = "/" + name
	query := u.Query()
	query.Del("search_path")
	u.RawQuery = query.Encode()
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE ledger_test(id integer PRIMARY KEY,value bigint NOT NULL);INSERT INTO ledger_test VALUES(1,32100),(2,-32100);CREATE TABLE private_uploads(object_key text PRIMARY KEY,state text);INSERT INTO private_uploads VALUES('upload_immutable_test','clean')`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	env := map[string]string{"PGHOST": u.Hostname(), "PGPORT": u.Port(), "PGUSER": u.User.Username(), "PGDATABASE": name, "PGSSLMODE": "disable"}
	if env["PGPORT"] == "" {
		env["PGPORT"] = "5432"
	}
	dir := t.TempDir()
	if password, ok := u.User.Password(); ok {
		passfile := filepath.Join(dir, "pgpass")
		os.WriteFile(passfile, []byte(fmt.Sprintf("%s:%s:*:%s:%s\n", env["PGHOST"], env["PGPORT"], env["PGUSER"], password)), 0600)
		env["PGPASSFILE"] = passfile
	}
	envfile := filepath.Join(dir, "postgres.json")
	os.WriteFile(envfile, Canonical(env), 0600)
	objects := filepath.Join(dir, "objects")
	os.Mkdir(objects, 0700)
	os.WriteFile(filepath.Join(objects, "upload_immutable_test"), []byte("synthetic-application-ciphertext"), 0600)
	e.Config.Sources["files_test"] = SourceProfile{Type: "qpf_postgres", EnvironmentFile: envfile, ObjectRoot: objects}
	p.Resources[0].Spec = Canonical(SourceSpec{Profile: "files_test", Type: "qpf_postgres", Classification: "restricted", RPOSeconds: 3600})
	cmd := Command{ID: "command_database", Operation: "backup", Resource: Ref{"plan_test", 1}, Approved: true, ExpiresAt: time.Now().Add(time.Hour)}
	p.Commands = []Command{cmd}
	r := e.Execute(context.Background(), p, NewCommandResult(p, Digest(Canonical(p)), cmd, time.Now()), &cmd, nil)
	if r.State != "succeeded" {
		t.Fatal("database capture", r)
	}
	if _, err = admin.Exec(`DROP DATABASE ` + name); err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(objects)
	restore := Command{ID: "command_database_restore", Operation: "restore", Resource: r.Plan, Approved: true, PointID: r.RunID, CopyID: r.Copies[0].ID, Target: "isolated_test", ExpiresAt: time.Now().Add(time.Hour)}
	p.Commands = append(p.Commands, restore)
	out := e.Execute(context.Background(), p, NewCommandResult(p, Digest(Canonical(p)), restore, time.Now()), &restore, &r)
	if out.State != "materialized" {
		t.Fatal(out)
	}
	target := filepath.Join(e.Config.RestoreRoots[restore.Target], restore.ID)
	object, err := os.ReadFile(filepath.Join(target, "private-objects", "upload_immutable_test"))
	if err != nil || string(object) != "synthetic-application-ciphertext" {
		t.Fatal("missing exact object", err)
	}
	manifest, err := os.ReadFile(filepath.Join(target, "recovery-manifest.json"))
	if err != nil || !json.Valid(manifest) {
		t.Fatal(err)
	}
	if _, err = admin.Exec(`CREATE DATABASE ` + name + `_restored`); err != nil {
		t.Fatal(err)
	}
	env["PGDATABASE"] = name + "_restored"
	if _, err = e.Runner.Run(context.Background(), Process{Tool: "pg_restore", Env: env, Args: []string{"--no-owner", "--no-acl", "--exit-on-error", "--dbname=" + env["PGDATABASE"], filepath.Join(target, "database.dump")}}); err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + env["PGDATABASE"]
	recovered, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	var count, sum int
	if err = recovered.QueryRow(`SELECT count(*),sum(value) FROM ledger_test`).Scan(&count, &sum); err != nil || count != 2 || sum != 0 {
		t.Fatal("recovered ledger differs", count, sum, err)
	}
}
