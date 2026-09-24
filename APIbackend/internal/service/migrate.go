package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
)

//go:embed schema.sql
var Schema string

//go:embed parity_v2.sql
var ParitySchema string

//go:embed admin_v3.sql
var AdminSchema string

// Migrate serialises schema changes with an advisory transaction lock and checks drift.
func Migrate(ctx context.Context, db *sql.DB) error {
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(818426001)`); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version integer PRIMARY KEY,checksum text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now())`); e != nil {
		return e
	}
	for index, source := range []string{Schema, ParitySchema, AdminSchema} {
		version := index + 1
		hash := sha256.Sum256([]byte(source))
		checksum := hex.EncodeToString(hash[:])
		var prior string
		e = tx.QueryRowContext(ctx, `SELECT checksum FROM schema_migrations WHERE version=$1`, version).Scan(&prior)
		if e == nil {
			if prior != checksum {
				return fmt.Errorf("schema version %d checksum differs from applied migration", version)
			}
			continue
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if _, e = tx.ExecContext(ctx, source); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)`, version, checksum); e != nil {
			return e
		}
	}
	return tx.Commit()
}
