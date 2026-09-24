package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var physicalLabel = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}F(_[0-9]{8}-[0-9]{6}[DI])?$`)

type physicalStanza struct {
	Cipher string           `json:"cipher"`
	Name   string           `json:"name"`
	Backup []physicalBackup `json:"backup"`
}
type physicalBackup struct {
	Reference []string `json:"reference"`
	Archive   struct {
		Start string `json:"start"`
		Stop  string `json:"stop"`
	} `json:"archive"`
	Label      string            `json:"label"`
	Annotation map[string]string `json:"annotation"`
	Timestamp  struct {
		Start int64 `json:"start"`
		Stop  int64 `json:"stop"`
	} `json:"timestamp"`
	Info struct {
		Size int64 `json:"size"`
	} `json:"info"`
	Error bool `json:"error"`
}

func (e *Engine) physicalInfo(ctx context.Context, p SourceProfile, d DestinationProfile) ([]physicalStanza, error) {
	if p.PgBackRestConfig == "" || !ID(p.Stanza) || d.PgRepository < 1 || d.PgRepository > 256 {
		return nil, errors.New("physical repository profile missing")
	}
	raw, err := e.Runner.Run(ctx, Process{Tool: "pgbackrest", Args: []string{"--config=" + p.PgBackRestConfig, "--stanza=" + p.Stanza, fmt.Sprintf("--repo=%d", d.PgRepository), "--output=json", "info"}})
	if err != nil {
		return nil, err
	}
	var v []physicalStanza
	if json.Unmarshal(raw, &v) != nil {
		return nil, errors.New("invalid physical inventory")
	}
	for _, stanza := range v {
		if stanza.Name == p.Stanza && stanza.Cipher != "aes-256-cbc" {
			return nil, errors.New("physical repository encryption is not qualified")
		}
	}
	return v, nil
}
func physicalExists(v []physicalStanza, label string) bool {
	for _, s := range v {
		for _, b := range s.Backup {
			if b.Label == label && !b.Error {
				return true
			}
		}
	}
	return false
}
func (e *Engine) physicalCapture(ctx context.Context, r Resource, d DestinationProfile, mode, run string, c Copy) (Copy, error) {
	p, err := e.source(r)
	if err != nil {
		return c, err
	}
	if d.Provider != "pgbackrest" || d.PgRepository < 1 || d.PgBackRestConfig != p.PgBackRestConfig || d.Stanza != p.Stanza {
		return c, errors.New("physical repository required")
	}
	if _, err = e.physicalInfo(ctx, p, d); err != nil {
		return c, err
	}
	c.CaptureStarted = e.Now()
	args := []string{"--config=" + p.PgBackRestConfig, "--stanza=" + p.Stanza, fmt.Sprintf("--repo=%d", d.PgRepository), "--type=" + mode, "--annotation=qpf-run=" + run, "--no-expire-auto", "backup"}
	if _, err = e.Runner.Run(ctx, Process{Tool: "pgbackrest", Args: args}); err != nil {
		return c, err
	}
	info, err := e.physicalInfo(ctx, p, d)
	if err != nil {
		return c, err
	}
	for _, s := range info {
		if s.Name != p.Stanza {
			continue
		}
		for _, b := range s.Backup {
			if b.Annotation["qpf-run"] == run && physicalLabel.MatchString(b.Label) && !b.Error && b.Timestamp.Stop >= b.Timestamp.Start {
				c.Snapshot = b.Label
				c.Dependencies = b.Reference
				c.WALStart = b.Archive.Start
				c.WALStop = b.Archive.Stop
				c.Bytes = strconv.FormatInt(b.Info.Size, 10)
				c.CaptureFinished = e.Now()
				c.Verified = "metadata"
				return c, nil
			}
		}
	}
	return c, errors.New("finalized physical backup could not be correlated")
}
func probePostgres(ctx context.Context, p SourceProfile) error {
	env, err := ReadEnvironment(p.EnvironmentFile)
	if err != nil {
		return err
	}
	dsn, err := postgresDSN(env)
	if err != nil {
		return err
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return db.PingContext(ctx)
}

func redisArguments(p SourceProfile, env map[string]string, tail []string) []string {
	a := []string{"-h", env["REDIS_HOST"], "-p", env["REDIS_PORT"]}
	if p.RedisTLS {
		a = append(a, "--tls")
		if p.RedisCACert != "" {
			a = append(a, "--cacert", p.RedisCACert)
		}
	}
	return append(a, tail...)
}
