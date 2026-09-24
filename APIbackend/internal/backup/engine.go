package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Engine is a single-writer, deployment-owned process. It accepts only signed,
// validated resource references; it cannot modify the application's live state.
type Engine struct {
	Config   AgentConfig
	Runner   Runner
	Now      func() time.Time
	Progress func(string, string)
	History  func() ([]JournalRun, error)
}

func NewEngine(c AgentConfig) (*Engine, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &Engine{Config: c, Runner: OSRunner{c.Executables}, Now: time.Now}, nil
}
func (e *Engine) step(run, stage string) {
	if e.Progress != nil {
		e.Progress(run, stage)
	}
}
func (e *Engine) Initialize(ctx context.Context, profile string) error {
	// Explicit CLI-only repository initialization. Never initialize on an auth error.
	d := e.Config.Stage
	if profile != "stage" {
		var ok bool
		d, ok = e.Config.Destinations[profile]
		if !ok {
			return errors.New("unknown destination profile")
		}
	}
	if d.Provider == "pgbackrest" {
		return errors.New("initialize physical repositories with the reviewed stanza-create runbook")
	}
	if err := protectedPassword(d.PasswordFile); err != nil {
		return err
	}
	p, err := resticProcess(d, "init")
	if err != nil {
		return err
	}
	_, err = e.Runner.Run(ctx, p)
	return err
}
func protectedPassword(path string) error {
	f, err := os.Lstat(path)
	if err != nil || !f.Mode().IsRegular() || f.Mode().Perm()&0077 != 0 || f.Size() < 16 || f.Size() > 4096 {
		return errors.New("repository password requires an owner-only regular file of 16–4096 bytes")
	}
	return nil
}
func (e *Engine) source(r Resource) (SourceProfile, error) {
	s := Parse[SourceSpec](r)
	p, ok := e.Config.Sources[s.Profile]
	if !ok || p.Type != s.Type {
		return p, errors.New("source profile is not provisioned")
	}
	return p, nil
}
func (e *Engine) destination(r Resource) (DestinationProfile, error) {
	s := Parse[DestinationSpec](r)
	p, ok := e.Config.Destinations[s.Profile]
	if !ok || p.Provider != s.Provider || p.FailureDomain != s.FailureDomain || p.OffHost != s.OffHost || p.Region != s.Region {
		return p, errors.New("destination profile differs from approved destination")
	}
	if p.Provider != "pgbackrest" {
		if err := protectedPassword(p.PasswordFile); err != nil {
			return p, err
		}
	}
	return p, nil
}
func (e *Engine) compatible(p Policy, ref Ref) error {
	r, ok := Find(p, ref)
	if !ok || r.Disabled {
		return errors.New("resource unavailable")
	}
	if r.Kind == "sources" {
		_, err := e.source(r)
		return err
	}
	if r.Kind == "destinations" {
		_, err := e.destination(r)
		return err
	}
	if r.Kind != "plans" || r.State != "approved" {
		return errors.New("approved plan required")
	}
	v := Parse[PlanSpec](r)
	if _, err := ValidateSpec("plans", r.Spec); err != nil {
		return err
	}
	for _, ref := range v.Sources {
		src, ok := Find(p, ref)
		if !ok || src.Disabled || src.State != "approved" {
			return errors.New("source no longer permitted")
		}
		if _, err := e.source(src); err != nil {
			return err
		}
	}
	for _, b := range v.Destinations {
		dst, ok := Find(p, b.Destination)
		if !ok || dst.Disabled || dst.State != "approved" {
			return errors.New("destination no longer permitted")
		}
		if _, err := e.destination(dst); err != nil {
			return err
		}
	}
	return nil
}
func NewCommandResult(p Policy, digest string, c Command, now time.Time) Result {
	return Result{Version: 1, AgentID: p.AgentID, RunID: CommandRunID(c.ID), PolicyDigest: digest, CommandID: c.ID, Plan: c.Resource, Operation: c.Operation, StartedAt: now, Copies: []Copy{}, Files: []File{}, State: "failed", PointID: c.PointID}
}
func (e *Engine) Execute(ctx context.Context, p Policy, r Result, command *Command, point *Result) Result {
	r.StartedAt = e.Now()
	r.State = "failed"
	r.ErrorCode = "profile_unavailable"
	r.Copies = []Copy{}
	r.Files = []File{}
	finish := func() Result {
		r.FinishedAt = e.Now()
		if r.FinishedAt.Before(r.StartedAt) {
			r.FinishedAt = r.StartedAt
		}
		if len(r.Files) > 5000 {
			r.Files = r.Files[:5000]
			r.FilesTruncated = true
		}
		return r
	}
	if !p.ExpiresAt.After(r.StartedAt) || p.AgentID != e.Config.ID || p.Environment != e.Config.Environment {
		r.ErrorCode = "policy_expired"
		return finish()
	}
	if ReadOperation(r.Operation) {
		if historical, ok := Find(p, r.Plan); !ok || historical.Kind != "plans" || historical.State != "approved" {
			return finish()
		}
	} else if err := e.compatible(p, r.Plan); err != nil {
		return finish()
	}
	resource, _ := Find(p, r.Plan)
	timeout := e.Config.TimeoutSeconds
	if resource.Kind == "plans" {
		timeout = Parse[PlanSpec](resource).TimeoutSeconds
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	if r.Operation == "test" {
		e.step(r.RunID, "testing")
		if err := e.probe(ctx, resource, r.RunID); err == nil {
			r.State = "succeeded"
			r.ErrorCode = ""
		} else {
			r.ErrorCode = "repository_unavailable"
		}
		return finish()
	}
	if !slices.Contains([]string{"backup", "full", "diff", "incr"}, r.Operation) {
		if command == nil || point == nil || point.Plan != r.Plan {
			r.ErrorCode = "invalid_inventory"
			return finish()
		}
		e.step(r.RunID, r.Operation)
		r.ErrorCode = "verification_failed"
		if r.Operation == "verify" {
			if e.verifyPoint(ctx, p, *command, *point) == nil {
				r.State = "succeeded"
				r.ErrorCode = ""
			}
			return finish()
		}
		if r.Operation == "restore" || r.Operation == "export" {
			r.ErrorCode = "restore_failed"
			if target, err := e.materialize(ctx, p, *command, *point); err == nil {
				r.State = "materialized"
				r.ErrorCode = ""
				r.MaterializedTarget = target
			}
			return finish()
		}
		r.ErrorCode = "retention_blocked"
		if r.Operation == "retention_preview" || r.Operation == "forget" {
			if preview, err := e.retention(ctx, p, *command, *point); err == nil {
				r.Retention = &preview
				r.State = "succeeded"
				r.ErrorCode = ""
				if r.Operation == "forget" {
					r.ExpiredCopyID = command.CopyID
				}
			}
		}
		return finish()
	}
	v := Parse[PlanSpec](resource)
	src0, _ := Find(p, v.Sources[0])
	physical := Parse[SourceSpec](src0).Type == "postgres_physical"
	if (r.Operation == "backup") == physical {
		r.ErrorCode = "command_unavailable"
		return finish()
	}
	stored := 0
	for si, ref := range v.Sources {
		src, _ := Find(p, ref)
		e.step(r.RunID, "capturing")
		var captured capture
		var capErr error
		if !physical {
			captured, capErr = e.captureSource(ctx, src, r.RunID, r.Plan.Key())
			if capErr == nil {
				r.Files = append(r.Files, captured.Files...)
			}
		}
		for di, b := range v.Destinations {
			c := Copy{ID: CopyID(r.RunID, si, di), Source: ref, Destination: b.Destination, State: "failed", Bytes: "0", Engine: "restic", Verified: "none", ErrorCode: "copy_failed"}
			dst, _ := Find(p, b.Destination)
			profile, err := e.destination(dst)
			if physical {
				c.Engine = "pgbackrest"
				c, err = e.physicalCapture(ctx, src, profile, r.Operation, r.RunID, c)
			} else if capErr != nil {
				c.ErrorCode = "capture_failed"
				err = capErr
			} else if err == nil {
				c.CaptureStarted = captured.Started
				c.CaptureFinished = captured.Finished
				c.Bytes = strconv.FormatInt(captured.Bytes, 10)
				e.step(r.RunID, "copying")
				c.Snapshot, err = e.copySnapshot(ctx, profile, e.Config.Stage, captured.Snapshot, r.RunID, src.Ref().Key())
				if err == nil {
					e.step(r.RunID, "verifying")
					err = e.check(ctx, profile, v.Verify)
					if err != nil {
						c.ErrorCode = "verification_failed"
					} else {
						c.Verified = v.Verify
					}
				}
			}
			if err == nil {
				c.State = "stored"
				c.ErrorCode = ""
				stored++
			}
			r.Copies = append(r.Copies, c)
		}
	}
	r.FinishedAt = e.Now()
	if RequiredComplete(p, r) {
		r.State = "succeeded"
		r.ErrorCode = ""
	} else if stored > 0 {
		r.State = "partial"
		r.ErrorCode = "copy_failed"
	} else {
		r.ErrorCode = "capture_failed"
	}
	e.step(r.RunID, "finalizing")
	return finish()
}
func (e *Engine) check(ctx context.Context, d DestinationProfile, level string) error {
	args := []string{}
	if level == "read_data" {
		args = append(args, "--read-data")
	}
	p, err := resticProcess(d, "check", args...)
	if err != nil {
		return err
	}
	_, err = e.Runner.Run(ctx, p)
	return err
}

type snapshotInfo struct {
	ID       string    `json:"id"`
	Original string    `json:"original"`
	Paths    []string  `json:"paths"`
	Tags     []string  `json:"tags"`
	Time     time.Time `json:"time"`
	Hostname string    `json:"hostname"`
}

func (e *Engine) snapshots(ctx context.Context, d DestinationProfile, args ...string) ([]snapshotInfo, error) {
	p, err := resticProcess(d, "snapshots", args...)
	if err != nil {
		return nil, err
	}
	raw, err := e.Runner.Run(ctx, p)
	if err != nil {
		return nil, err
	}
	var out []snapshotInfo
	if json.Unmarshal(raw, &out) != nil {
		return nil, errors.New("invalid snapshot inventory")
	}
	for _, s := range out {
		if !digestPattern.MatchString(s.ID) {
			return nil, errors.New("invalid snapshot identifier")
		}
	}
	return out, nil
}
func (e *Engine) copySnapshot(ctx context.Context, d, from DestinationProfile, snapshot, run, source string) (string, error) {
	if !digestPattern.MatchString(snapshot) {
		return "", errors.New("exact snapshot hash required")
	}
	p, err := resticProcess(d, "copy", "--from-repo", from.Repository, "--from-password-file", from.PasswordFile, snapshot)
	if err != nil {
		return "", err
	}
	// Stage is always local, so it needs no competing remote credential environment.
	if _, err = e.Runner.Run(ctx, p); err != nil {
		return "", err
	}
	list, err := e.snapshots(ctx, d, "--tag", run+","+source)
	if err != nil {
		return "", err
	}
	for _, s := range list {
		if slices.Contains(s.Tags, run) && slices.Contains(s.Tags, source) && (s.Original == snapshot || s.ID == snapshot) {
			return s.ID, nil
		}
	}
	return "", errors.New("copied snapshot could not be correlated to original")
}
func (e *Engine) probe(ctx context.Context, r Resource, run string) error {
	if r.Kind == "sources" {
		p, err := e.source(r)
		if err != nil {
			return err
		}
		switch p.Type {
		case "files":
			root, err := os.OpenRoot(p.Root)
			if err != nil {
				return err
			}
			defer root.Close()
			if _, err = root.Stat("PG_VERSION"); err == nil {
				return errors.New("raw database folder rejected")
			}
			f, err := root.Open(".")
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = f.ReadDir(1)
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			return nil
		case "postgres_logical", "qpf_postgres":
			return probePostgres(ctx, p)
		case "postgres_physical":
			_, err = e.Runner.Run(ctx, Process{Tool: "pgbackrest", Args: []string{"--config=" + p.PgBackRestConfig, "--stanza=" + p.Stanza, "check"}})
			return err
		case "mongodb":
			if !p.MongoReplicaSet || p.MongoConfig == "" {
				return errors.New("qualified replica-set required")
			}
			_, err = e.Runner.Run(ctx, Process{Tool: "mongodump", Args: []string{"--version"}})
			return err
		case "redis":
			env, err := ReadEnvironment(p.EnvironmentFile)
			if err != nil {
				return err
			}
			_, err = e.Runner.Run(ctx, Process{Tool: "redis-cli", Args: redisArguments(p, env, []string{"PING"}), Env: env})
			return err
		}
		return errors.New("unsupported source")
	}
	d, err := e.destination(r)
	if err != nil {
		return err
	}
	if d.Provider == "pgbackrest" {
		if d.PgBackRestConfig == "" || !ID(d.Stanza) || d.PgRepository < 1 {
			return errors.New("provision the physical destination stanza/config/repository")
		}
		_, err := e.Runner.Run(ctx, Process{Tool: "pgbackrest", Args: []string{"--config=" + d.PgBackRestConfig, "--stanza=" + d.Stanza, fmt.Sprintf("--repo=%d", d.PgRepository), "check"}})
		return err
	}
	if _, err = e.snapshots(ctx, d); err != nil {
		return err
	}
	data := []byte("Qpay synthetic encrypted connectivity probe: " + run)
	p, err := resticProcess(d, "backup", "--stdin", "--stdin-filename", "qpf-connection-probe.txt", "--host", e.Config.ID, "--tag", "qpf-probe", "--tag", run)
	if err != nil {
		return err
	}
	p.Stdin = bytes.NewReader(data)
	raw, err := e.Runner.Run(ctx, p)
	if err != nil {
		return err
	}
	snapshot := ""
	for _, line := range strings.Split(string(raw), "\n") {
		var s struct {
			Type string `json:"message_type"`
			ID   string `json:"snapshot_id"`
		}
		if json.Unmarshal([]byte(line), &s) == nil && s.Type == "summary" && digestPattern.MatchString(s.ID) {
			snapshot = s.ID
		}
	}
	if snapshot == "" {
		return errors.New("missing probe snapshot")
	}
	p, err = resticProcess(d, "dump", snapshot, "qpf-connection-probe.txt")
	if err != nil {
		return err
	}
	raw, err = e.Runner.Run(ctx, p)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, data) {
		return errors.New("probe readback mismatch")
	}
	return nil
}
func selectedCopy(point Result, id string) (Copy, error) {
	for _, c := range point.Copies {
		if c.ID == id && c.State == "stored" {
			return c, nil
		}
	}
	return Copy{}, errors.New("copy is not available")
}
func (e *Engine) verifyPoint(ctx context.Context, p Policy, cmd Command, point Result) error {
	c, err := selectedCopy(point, cmd.CopyID)
	if err != nil {
		return err
	}
	r, ok := Find(p, c.Destination)
	if !ok {
		return errors.New("missing historical destination")
	}
	d, err := e.destination(r)
	if err != nil {
		return err
	}
	if c.Engine == "pgbackrest" {
		src, ok := Find(p, c.Source)
		if !ok {
			return errors.New("missing source")
		}
		profile, err := e.source(src)
		if err != nil {
			return err
		}
		info, err := e.physicalInfo(ctx, profile, d)
		if err != nil {
			return err
		}
		if !physicalExists(info, c.Snapshot) {
			return errors.New("physical recovery chain absent")
		}
		// pgBackRest verify checks repository backup and WAL checksums, not database recovery.
		_, err = e.Runner.Run(ctx, Process{Tool: "pgbackrest", Args: []string{"--config=" + profile.PgBackRestConfig, "--stanza=" + profile.Stanza, fmt.Sprintf("--repo=%d", d.PgRepository), "--set=" + c.Snapshot, "verify"}})
		return err
	}
	list, err := e.snapshots(ctx, d, c.Snapshot)
	if err != nil || len(list) != 1 || list[0].ID != c.Snapshot {
		return errors.New("exact snapshot is unavailable")
	}
	return e.check(ctx, d, "read_data")
}
func (e *Engine) materialize(ctx context.Context, p Policy, cmd Command, point Result) (string, error) {
	root, ok := e.Config.RestoreRoots[cmd.Target]
	if !ok {
		return "", errors.New("target is not provisioned")
	}
	fi, err := os.Lstat(root)
	if err != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm()&0077 != 0 {
		return "", errors.New("isolated recovery root must be an owner-only nonsymlink directory")
	}
	c, err := selectedCopy(point, cmd.CopyID)
	if err != nil {
		return "", err
	}
	r, ok := Find(p, c.Destination)
	if !ok {
		return "", errors.New("historical destination absent")
	}
	d, err := e.destination(r)
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, cmd.ID)
	if err = os.Mkdir(target, 0700); err != nil {
		return "", errors.New("target already exists or is unavailable; never overwrite a restore")
	}
	// Failed materialization is retained for protected investigation. It is never
	// announced as usable, recursively deleted, or placed into a production path.
	if c.Engine == "pgbackrest" {
		if cmd.Operation == "export" {
			return "", errors.New("physical export requires base and WAL dependency packaging")
		}
		src, ok := Find(p, c.Source)
		if !ok {
			return "", errors.New("missing physical profile")
		}
		sp, err := e.source(src)
		if err != nil {
			return "", err
		}
		if !physicalLabel.MatchString(c.Snapshot) {
			return "", errors.New("invalid physical backup label")
		}
		args := []string{"--config=" + sp.PgBackRestConfig, "--stanza=" + sp.Stanza, fmt.Sprintf("--repo=%d", d.PgRepository), "--reset-pg1-host", "--pg1-path=" + target, "--set=" + c.Snapshot, "--target-action=pause"}
		if cmd.RecoveryTime != nil {
			if cmd.RecoveryTime.Before(c.CaptureFinished) || cmd.RecoveryTime.After(e.Now()) {
				return "", errors.New("physical recovery target is outside the permitted time bounds")
			}
			args = append(args, "--type=time", "--target="+cmd.RecoveryTime.UTC().Format("2006-01-02 15:04:05.999999-07:00"))
		} else {
			args = append(args, "--type=immediate")
		}
		args = append(args, "restore")
		if _, err = e.Runner.Run(ctx, Process{Tool: "pgbackrest", Args: args}); err != nil {
			return "", err
		}
	} else {
		if !digestPattern.MatchString(c.Snapshot) {
			return "", errors.New("exact snapshot required")
		}
		if cmd.Operation == "export" {
			// An independent encrypted repository, not arbitrary chunks or plaintext ZIP.
			// The existing independently escrowed destination key opens the export. The
			// key itself is never written into the bundle.
			output := d
			output.Provider = "local"
			output.Repository = filepath.Join(target, "repository")
			output.EnvironmentFile = ""
			init, err := resticProcess(output, "init")
			if err != nil {
				return "", err
			}
			if _, err = e.Runner.Run(ctx, init); err != nil {
				return "", err
			}
			copy, err := resticProcess(d, "copy")
			if err != nil {
				return "", err
			}
			// Destination is local; source environment is needed by the remote read side.
			copy.Env["RESTIC_REPOSITORY"] = output.Repository
			copy.Env["RESTIC_PASSWORD_FILE"] = output.PasswordFile
			copy.Args = []string{"--json", "copy", "--from-repo", d.Repository, "--from-password-file", d.PasswordFile, c.Snapshot}
			if _, err = e.Runner.Run(ctx, copy); err != nil {
				return "", err
			}
			if err = e.check(ctx, output, "read_data"); err != nil {
				return "", err
			}
		} else {
			pr, err := resticProcess(d, "restore", c.Snapshot, "--target", target, "--verify")
			if err != nil {
				return "", err
			}
			if _, err = e.Runner.Run(ctx, pr); err != nil {
				return "", err
			}
		}
	}
	manifest := map[string]any{"version": 1, "operation": cmd.Operation, "point_id": point.RunID, "copy_id": c.ID, "snapshot": c.Snapshot, "plan": point.Plan, "target_profile": cmd.Target, "created_at": e.Now(), "database_started": false, "financial_execution_allowed": false, "resume_requires": "external reconciliation and independent approval"}
	if err = os.WriteFile(filepath.Join(target, "RECOVERY-STATUS.json"), Canonical(manifest), 0600); err != nil {
		return "", err
	}
	return cmd.Target + ":" + cmd.ID, nil
}
