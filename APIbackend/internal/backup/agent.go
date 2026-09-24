package backup

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type Agent struct {
	Config    AgentConfig
	Engine    *Engine
	Journal   *Journal
	Client    *Client
	Private   ed25519.PrivateKey
	PolicyKey ed25519.PublicKey
	Now       func() time.Time
}

func OpenAgent(c AgentConfig) (*Agent, error) {
	e, err := NewEngine(c)
	if err != nil {
		return nil, err
	}
	seed, err := ReadKey(c.PrivateKeyFile, true)
	if err != nil || len(seed) != 32 {
		return nil, errors.New("protected agent signing seed required")
	}
	key := ed25519.NewKeyFromSeed(seed)
	pub, err := ReadKey(c.PolicyPublicKeyFile, false)
	if err != nil || len(pub) != 32 {
		return nil, errors.New("trusted control-plane public key required")
	}
	stateKey, err := ReadKey(c.StateKeyFile, true)
	if err != nil {
		return nil, err
	}
	j, err := OpenJournal(c.StateDir, stateKey)
	if err != nil {
		return nil, err
	}
	client, err := NewClient(c, key)
	if err != nil {
		j.Close()
		return nil, err
	}
	if err = os.MkdirAll(c.ScratchDir, 0700); err != nil {
		j.Close()
		return nil, err
	}
	a := &Agent{c, e, j, client, key, ed25519.PublicKey(pub), time.Now}
	a.Engine.History = j.Runs
	if err = j.RecoverInterrupted(key, time.Now()); err != nil {
		j.Close()
		return nil, err
	}
	return a, nil
}
func (a *Agent) Close() error { return a.Journal.Close() }
func (a *Agent) Synchronize(ctx context.Context) (Policy, Signed, error) {
	state, err := a.Journal.State()
	if err != nil {
		return Policy{}, Signed{}, err
	}
	signed := Signed{}
	online := a.Client.Request(ctx, "GET", "/v1/backup-agent/policy", nil, &signed) == nil
	if !online {
		signed = state.Policy
	}
	p, err := VerifyPolicy(signed, a.PolicyKey, a.Config.ID, a.Config.Environment, a.Now(), state.Serial)
	if err != nil {
		return p, signed, err
	}
	if online {
		state.Policy = signed
		state.Serial = p.Serial
		state.LastSync = a.Now()
		if err = a.Journal.Write("state", state); err != nil {
			return p, signed, err
		}
	}
	return p, signed, nil
}
func (a *Agent) Flush(ctx context.Context) error {
	runs, err := a.Journal.Runs()
	if err != nil {
		return err
	}
	for _, entry := range runs {
		if !entry.Complete || entry.Reported {
			continue
		}
		if err = a.Client.Request(ctx, "POST", "/v1/backup-agent/results", entry.Signed, nil); err != nil {
			return err
		}
		entry.Reported = true
		if err = a.Journal.Write(entry.Result.RunID, entry); err != nil {
			return err
		}
	}
	return nil
}
func (a *Agent) Tick(ctx context.Context) error {
	// A lost API response leaves the same signed result in the durable outbox.
	// It can be uploaded repeatedly but can never rewrite the original occurrence.
	_ = a.Flush(ctx)
	p, signed, err := a.Synchronize(ctx)
	if err != nil {
		return err
	}
	state, err := a.Journal.State()
	if err != nil {
		return err
	}
	runs, err := a.Journal.Runs()
	if err != nil {
		return err
	}
	known := map[string]bool{}
	pending := 0
	for _, r := range runs {
		known[r.Result.RunID] = true
		if !r.Reported {
			pending++
		}
	}
	if pending >= 512 {
		return errors.New("result outbox is full; no new captures until evidence can be reported")
	}
	health := Health{Version: 1, AgentID: a.Config.ID, ObservedAt: a.Now(), PolicyExpiresAt: p.ExpiresAt, PendingResults: pending, Stage: "idle"}
	for _, run := range runs {
		if run.Complete && run.Result.State == "succeeded" && slices.Contains([]string{"backup", "full", "diff", "incr"}, run.Result.Operation) && run.Result.FinishedAt.After(health.LastSuccessfulCapture) {
			health.LastSuccessfulCapture = run.Result.FinishedAt
		}
	}
	var healthMu sync.Mutex
	publish := func() {
		healthMu.Lock()
		defer healthMu.Unlock()
		health.ObservedAt = a.Now()
		_ = WriteHealth(a.Config.StateDir, health, a.Private)
	}
	healthCtx, stopHealth := context.WithCancel(ctx)
	defer stopHealth()
	doneHealth := make(chan struct{})
	defer func() { stopHealth(); <-doneHealth }()
	go func() {
		defer close(doneHealth)
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-healthCtx.Done():
				return
			case <-t.C:
				publish()
			}
		}
	}()
	a.Engine.Progress = func(run, stage string) {
		healthMu.Lock()
		health.RunID = run
		health.Stage = stage
		healthMu.Unlock()
		publish()
	}
	defer func() { a.Engine.Progress = nil }()
	publish()
	status := AgentStatus{ID: a.Config.ID, InstanceID: a.Config.InstanceID, Environment: a.Config.Environment, ObservedAt: a.Now(), Versions: map[string]string{}, PolicyExpiresAt: p.ExpiresAt, PendingResults: pending, FreeBytes: "0"}
	var st syscall.Statfs_t
	if syscall.Statfs(a.Config.ScratchDir, &st) == nil {
		status.FreeBytes = strconv.FormatUint(st.Bavail*uint64(st.Bsize), 10)
	}
	for tool := range a.Config.Executables {
		args := []string{"--version"}
		if tool == "restic" || tool == "rclone" || tool == "pgbackrest" {
			args = []string{"version"}
		}
		out, err := a.Engine.Runner.Run(ctx, Process{Tool: tool, Args: args})
		if err == nil {
			line := string(out)
			for i, c := range line {
				if c == '\n' {
					line = line[:i]
					break
				}
			}
			if Text(line, 200) {
				status.Versions[tool] = line
			}
		}
	}
	status.Stage = "idle"
	_ = a.Client.Request(ctx, "POST", "/v1/backup-agent/heartbeat", status, nil)
	heartbeatDone := make(chan struct{})
	defer func() { stopHealth(); <-heartbeatDone }()
	go func() {
		defer close(heartbeatDone)
		timer := time.NewTicker(30 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-healthCtx.Done():
				return
			case <-timer.C:
				copy := status
				healthMu.Lock()
				copy.ObservedAt = a.Now()
				copy.Stage = health.Stage
				copy.RunID = health.RunID
				healthMu.Unlock()
				_ = a.Client.Request(healthCtx, "POST", "/v1/backup-agent/heartbeat", copy, nil)
			}
		}
	}()

	// Commands are one-shot. Restores/exports additionally require a fresh online
	// claim, so an old signed lease cannot materialize secrets during an API outage.
	for _, cmd := range p.Commands {
		runID := CommandRunID(cmd.ID)
		if known[runID] || !cmd.Approved || !cmd.ExpiresAt.After(a.Now()) {
			continue
		}
		var point *Result
		if cmd.PointID != "" {
			point = &Result{}
			if err = a.Client.Request(ctx, "POST", "/v1/backup-agent/commands/"+cmd.ID+"/point", struct{}{}, point); err != nil {
				continue
			}
		}
		if slices.Contains([]string{"restore", "export", "forget"}, cmd.Operation) {
			if err = a.Client.Request(ctx, "POST", "/v1/backup-agent/commands/"+cmd.ID+"/claim", struct{}{}, nil); err != nil {
				continue
			}
		}
		r := NewCommandResult(p, Digest(signed.Payload), cmd, a.Now())
		if err = a.execute(ctx, p, signed, r, &cmd, point); err != nil {
			return err
		}
		known[runID] = true
	}
	for _, resource := range p.Resources {
		if resource.Kind != "schedules" || resource.State != "approved" || resource.Disabled {
			continue
		}
		s := Parse[ScheduleSpec](resource)
		last := state.Cursors[resource.Ref().Key()]
		occurrence, due, err := Due(s, last, a.Now())
		if err != nil {
			return err
		}
		if due && !known[OccurrenceID(resource.Ref(), occurrence)] {
			r := Result{Version: 1, AgentID: a.Config.ID, RunID: OccurrenceID(resource.Ref(), occurrence), PolicyDigest: Digest(signed.Payload), Schedule: resource.Ref(), Plan: s.Plan, Occurrence: occurrence, Operation: s.Operation, StartedAt: a.Now(), State: "unknown"}
			// Verification schedules choose their latest actual matching point, never a
			// synthetic placeholder. No point means a visible missing-evidence failure.
			var point *Result
			var cmd *Command
			if s.Operation == "verify" {
				slices.SortFunc(runs, func(a, b JournalRun) int { return a.Result.FinishedAt.Compare(b.Result.FinishedAt) })
				for i := len(runs) - 1; i >= 0; i-- {
					x := runs[i].Result
					if x.Plan == s.Plan && x.State == "succeeded" && slices.Contains([]string{"backup", "full", "diff", "incr"}, x.Operation) && len(x.Copies) > 0 {
						y := x
						point = &y
						for _, copy := range x.Copies {
							if copy.State == "stored" {
								cmd = &Command{Resource: s.Plan, PointID: x.RunID, CopyID: copy.ID, Operation: "verify"}
								break
							}
						}
						break
					}
				}
			}
			if err = a.execute(ctx, p, signed, r, cmd, point); err != nil {
				return err
			}
		}
		// Persist after durable run reservation/completion. A crash here finds the
		// existing run marker and cannot recapture the same logical occurrence.
		state, err = a.Journal.State()
		if err != nil {
			return err
		}
		state.Cursors[resource.Ref().Key()] = a.Now()
		if err = a.Journal.Write("state", state); err != nil {
			return err
		}
	}
	_ = a.Flush(ctx)
	healthMu.Lock()
	health.Stage = "idle"
	health.RunID = ""
	if latest, err := a.Journal.Runs(); err == nil {
		for _, run := range latest {
			if run.Complete && run.Result.State == "succeeded" && slices.Contains([]string{"backup", "full", "diff", "incr"}, run.Result.Operation) && run.Result.FinishedAt.After(health.LastSuccessfulCapture) {
				health.LastSuccessfulCapture = run.Result.FinishedAt
			}
		}
	}
	healthMu.Unlock()
	publish()
	return nil
}
func (a *Agent) execute(ctx context.Context, p Policy, signed Signed, r Result, cmd *Command, point *Result) error {
	entry := JournalRun{Result: r, Policy: signed}
	if err := a.Journal.Write(r.RunID, entry); err != nil {
		return err
	}
	r = a.Engine.Execute(ctx, p, r, cmd, point)
	if err := ValidateResult(r); err != nil {
		return fmt.Errorf("agent result validation failed: %w", err)
	}
	if err := AuthorizeResult(p, r); err != nil {
		return err
	}
	s, err := Sign(r, a.Private)
	if err != nil {
		return err
	}
	entry.Result = r
	entry.Signed = s
	entry.Complete = true
	if err = a.Journal.Write(r.RunID, entry); err != nil {
		return err
	}
	state, err := a.Journal.State()
	if err != nil {
		return err
	}
	state.LastResult = a.Now()
	return a.Journal.Write("state", state)
}
func (a *Agent) Watch(ctx context.Context, interval time.Duration, report func(error)) error {
	if interval < 15*time.Second {
		return errors.New("poll interval must be at least 15 seconds")
	}
	for {
		if err := a.Tick(ctx); err != nil && report != nil {
			report(err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
