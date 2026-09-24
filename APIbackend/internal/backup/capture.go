package backup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

type capture struct {
	Snapshot          string
	Files             []File
	Started, Finished time.Time
	Bytes             int64
}
type budgetWriter struct {
	Writer    io.Writer
	Remaining *int64
}

func (w budgetWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > *w.Remaining {
		return 0, errors.New("staging budget exhausted")
	}
	n, e := w.Writer.Write(p)
	*w.Remaining -= int64(n)
	return n, e
}
func saveStream(path string, reader io.Reader, budget *int64) (int64, string, error) {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return 0, "", e
	}
	h := sha256.New()
	n, e := io.Copy(budgetWriter{io.MultiWriter(f, h), budget}, reader)
	syncErr := f.Sync()
	closeErr := f.Close()
	if e == nil {
		e = syncErr
	}
	if e == nil {
		e = closeErr
	}
	return n, hex.EncodeToString(h.Sum(nil)), e
}
func excluded(name string) bool {
	return slices.Contains([]string{".git", "node_modules", "vendor", ".cache", "tmp", ".DS_Store", "pg_wal", "pg_xlog", "postmaster.pid"}, name) || strings.HasPrefix(name, ".env") || strings.HasSuffix(name, ".sock") || strings.HasSuffix(name, ".tmp")
}
func copyFiles(root, target, source string, budget *int64) ([]File, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) == "/" {
		return nil, errors.New("dedicated source root required")
	}
	if _, e := os.Stat(filepath.Join(root, "PG_VERSION")); e == nil {
		return nil, errors.New("raw PostgreSQL directories are not filesystem backup sources")
	}
	opened, e := os.OpenRoot(root)
	if e != nil {
		return nil, e
	}
	defer opened.Close()
	out := []File{}
	e = fs.WalkDir(opened.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name != "." && excluded(d.Name()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("source contains a symlink; use a reviewed read-only snapshot without escape paths")
		}
		dest := filepath.Join(target, filepath.FromSlash(name))
		if d.IsDir() {
			return os.MkdirAll(dest, 0700)
		}
		if !d.Type().IsRegular() {
			return errors.New("special source files are not supported")
		}
		if len(out) >= 100000 {
			return errors.New("source file count exceeds configured implementation bound")
		}
		f, e := opened.Open(name)
		if e != nil {
			return e
		}
		before, e := f.Stat()
		if e != nil {
			f.Close()
			return e
		}
		size, digest, e := saveStream(dest, f, budget)
		after, statErr := f.Stat()
		f.Close()
		if e != nil {
			return e
		}
		if statErr != nil || before.Size() != size || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			return errors.New("source changed during capture; no complete snapshot is claimed")
		}
		out = append(out, File{Path: filepath.ToSlash(name), Size: strconv.FormatInt(size, 10), SHA256: digest, SourceID: source})
		return nil
	})
	return out, e
}
func (e *Engine) captureSource(ctx context.Context, r Resource, run, plan string) (capture, error) {
	out := capture{Started: e.Now()}
	spec := Parse[SourceSpec](r)
	profile, ok := e.Config.Sources[spec.Profile]
	if !ok || profile.Type != spec.Type {
		return out, errors.New("source profile mismatch")
	}
	dir, err := os.MkdirTemp(e.Config.ScratchDir, "capture-")
	if err != nil {
		return out, err
	}
	defer os.RemoveAll(dir)
	if err = os.Chmod(dir, 0700); err != nil {
		return out, err
	}
	budget := e.Config.MaxStageBytes
	switch spec.Type {
	case "files":
		out.Files, err = copyFiles(profile.Root, filepath.Join(dir, "files"), r.ID, &budget)
		for i := range out.Files {
			out.Files[i].Path = "files/" + out.Files[i].Path
		}
	case "postgres_logical", "qpf_postgres":
		out.Files, err = e.capturePostgres(ctx, profile, r.ID, dir, &budget, spec.Type == "qpf_postgres")
	case "mongodb":
		if !profile.MongoReplicaSet || profile.MongoConfig == "" {
			return out, errors.New("MongoDB capture requires a qualified non-sharded replica-set profile")
		}
		f, ferr := os.OpenFile(filepath.Join(dir, "mongodb.archive.gz"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if ferr != nil {
			return out, ferr
		}
		_, err = e.Runner.Run(ctx, Process{Tool: "mongodump", Args: []string{"--config=" + profile.MongoConfig, "--archive", "--gzip", "--oplog"}, Stdout: budgetWriter{f, &budget}})
		f.Sync()
		f.Close()
		if err == nil {
			out.Files, err = inventory(dir, r.ID)
		}
	case "redis":
		env, ferr := ReadEnvironment(profile.EnvironmentFile)
		if ferr != nil {
			return out, ferr
		}
		host, port := env["REDIS_HOST"], env["REDIS_PORT"]
		if host == "" || port == "" {
			return out, errors.New("Redis profile must define a host and port")
		}
		_, err = e.Runner.Run(ctx, Process{Tool: "redis-cli", Args: redisArguments(profile, env, []string{"--rdb", filepath.Join(dir, "redis.rdb")}), Env: env})
		if err == nil {
			out.Files, err = inventory(dir, r.ID)
			if err == nil {
				for _, f := range out.Files {
					n, _ := strconv.ParseInt(f.Size, 10, 64)
					if n > budget {
						err = errors.New("Redis output exceeded the staging budget")
					}
				}
			}
		}
	default:
		return out, errors.New("unsupported capture source")
	}
	if err != nil {
		return out, err
	}
	meta := map[string]any{"version": 1, "source": r.Ref(), "type": spec.Type, "run_id": run, "plan_id": plan, "started_at": out.Started, "files": out.Files, "financial_execution_allowed": false, "instructions": "Restore into an isolated environment; never replay financial or notification queues. Keys are independently escrowed."}
	if err = os.WriteFile(filepath.Join(dir, "recovery-manifest.json"), Canonical(meta), 0600); err != nil {
		return out, err
	}
	p, err := resticProcess(e.Config.Stage, "backup", "--host", e.Config.ID, "--tag", run, "--tag", r.Ref().Key(), "--tag", plan, "--", ".")
	if err != nil {
		return out, err
	}
	p.Dir = dir
	raw, err := e.Runner.Run(ctx, p)
	if err != nil {
		return out, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		var v struct {
			Message    string `json:"message_type"`
			ID         string `json:"snapshot_id"`
			TotalBytes int64  `json:"total_bytes_processed"`
		}
		if json.Unmarshal([]byte(line), &v) == nil && v.Message == "summary" && digestPattern.MatchString(v.ID) {
			out.Snapshot = v.ID
			out.Bytes = v.TotalBytes
		}
	}
	if out.Snapshot == "" {
		return out, errors.New("backup completed without a finalized snapshot identifier")
	}
	out.Finished = e.Now()
	return out, nil
}
func inventory(dir, source string) ([]File, error) {
	out := []File{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return errors.New("unexpected special staging file")
		}
		f, e := os.Open(path)
		if e != nil {
			return e
		}
		h := sha256.New()
		n, e := io.Copy(h, f)
		f.Close()
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(dir, path)
		if e != nil {
			return e
		}
		out = append(out, File{Path: filepath.ToSlash(rel), SourceID: source, Size: strconv.FormatInt(n, 10), SHA256: hex.EncodeToString(h.Sum(nil))})
		return nil
	})
	return out, err
}
func (e *Engine) capturePostgres(ctx context.Context, p SourceProfile, source, dir string, budget *int64, objects bool) ([]File, error) {
	env, err := ReadEnvironment(p.EnvironmentFile)
	if err != nil {
		return nil, err
	}
	if e.Config.Environment != "local" && env["PGSSLMODE"] != "verify-full" {
		return nil, errors.New("non-local PostgreSQL capture requires verify-full TLS")
	}
	if env["PGDATABASE"] == "" || env["PGUSER"] == "" {
		return nil, errors.New("PostgreSQL environment requires explicit database/user")
	}
	args := []string{"--format=custom", "--no-owner", "--no-acl", "--lock-wait-timeout=30000"}
	versions := map[string]string{}
	var tx *sql.Tx
	var db *sql.DB
	if objects {
		// pgx parses explicit libpq variables from a constructed DSN, not process-global
		// environment. The same exported MVCC snapshot feeds object inventory and pg_dump.
		config, er := postgresDSN(env)
		if er != nil {
			return nil, er
		}
		db, er = sql.Open("pgx", config)
		if er != nil {
			return nil, er
		}
		defer db.Close()
		tx, err = db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()
		var snapshot string
		if err = tx.QueryRowContext(ctx, `SELECT pg_export_snapshot()`).Scan(&snapshot); err != nil {
			return nil, err
		}
		args = append(args, "--snapshot="+snapshot)
		// Existing private uploads use immutable unique object keys. Missing/deleted
		// content at this boundary aborts the entire required source capture.
		rows, er := tx.QueryContext(ctx, `SELECT object_key FROM private_uploads WHERE state<>'deleted' ORDER BY object_key`)
		if er != nil {
			return nil, er
		}
		keys := []string{}
		for rows.Next() {
			var key string
			if er = rows.Scan(&key); er != nil {
				rows.Close()
				return nil, er
			}
			if !ID(key) || len(keys) >= 100000 {
				rows.Close()
				return nil, errors.New("invalid private-object inventory")
			}
			keys = append(keys, key)
		}
		er = rows.Err()
		rows.Close()
		if er != nil {
			return nil, er
		}
		if len(keys) > 0 && p.ObjectRoot == "" && p.ObjectProfile == "" {
			return nil, errors.New("private-object root must be configured; missing objects cannot be skipped")
		}
		if len(keys) > 0 && p.ObjectProfile != "" {
			var er error
			versions, er = e.captureObjectVersions(ctx, p.ObjectProfile, keys, dir, budget)
			if er != nil {
				return nil, er
			}
		} else if len(keys) > 0 {
			root, er := os.OpenRoot(p.ObjectRoot)
			if er != nil {
				return nil, er
			}
			defer root.Close()
			if er = os.Mkdir(filepath.Join(dir, "private-objects"), 0700); er != nil {
				return nil, er
			}
			for _, key := range keys {
				fi, er := root.Lstat(key)
				if er != nil || !fi.Mode().IsRegular() {
					return nil, errors.New("required private object is absent or unsafe")
				}
				f, er := root.Open(key)
				if er != nil {
					return nil, er
				}
				_, _, er = saveStream(filepath.Join(dir, "private-objects", key), f, budget)
				f.Close()
				if er != nil {
					return nil, er
				}
			}
		}
	}
	f, err := os.OpenFile(filepath.Join(dir, "database.dump"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	_, err = e.Runner.Run(ctx, Process{Tool: "pg_dump", Args: args, Env: env, Stdout: budgetWriter{f, budget}})
	syncErr := f.Sync()
	f.Close()
	if err != nil {
		return nil, err
	}
	if syncErr != nil {
		return nil, syncErr
	}
	// Globals are a separate cluster-level portability asset; not falsely described
	// as part of the same database MVCC snapshot. Password hashes are excluded.
	globals, err := os.OpenFile(filepath.Join(dir, "globals.sql"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	_, err = e.Runner.Run(ctx, Process{Tool: "pg_dumpall", Args: []string{"--globals-only", "--no-role-passwords"}, Env: env, Stdout: budgetWriter{globals, budget}})
	globals.Sync()
	globals.Close()
	if err != nil {
		return nil, err
	}
	_, err = e.Runner.Run(ctx, Process{Tool: "pg_restore", Args: []string{"--list", filepath.Join(dir, "database.dump")}})
	if err != nil {
		return nil, err
	}
	files, err := inventory(dir, source)
	if err != nil {
		return nil, err
	}
	for i := range files {
		files[i].ObjectVersion = versions[files[i].Path]
	}
	return files, nil
}
func postgresDSN(env map[string]string) (string, error) {
	keys := map[string]string{"PGHOST": "host", "PGPORT": "port", "PGDATABASE": "dbname", "PGUSER": "user", "PGPASSFILE": "passfile", "PGSSLMODE": "sslmode", "PGSSLROOTCERT": "sslrootcert", "PGSSLCERT": "sslcert", "PGSSLKEY": "sslkey", "PGCONNECT_TIMEOUT": "connect_timeout"}
	parts := []string{}
	for k, v := range env {
		if dbkey, ok := keys[k]; ok {
			if strings.ContainsAny(v, "\n\r\x00") {
				return "", errors.New("invalid database profile")
			}
			v = strings.ReplaceAll(strings.ReplaceAll(v, "\\", "\\\\"), "'", "\\'")
			parts = append(parts, fmt.Sprintf("%s='%s'", dbkey, v))
		}
	}
	return strings.Join(parts, " "), nil
}
