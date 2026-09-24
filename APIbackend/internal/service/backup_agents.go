package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/backup"
)

func (s *Service) BackupAgentAuthenticate(ctx context.Context, header, method, path string, body []byte) (string, error) {
	if s.Config.Backups == nil {
		return "", unavailable()
	}
	a, e := backup.DecodeAuth(header, s.Config.Backups.Keys(), method, path, body, s.Now())
	if e != nil {
		return "", unauthorized()
	}
	result, e := s.DB.ExecContext(ctx, `INSERT INTO backup_agent_nonces(agent_id,nonce,expires_at) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, a.AgentID, a.Nonce, s.Now().Add(10*time.Minute))
	if e != nil {
		return "", e
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return "", unauthorized()
	}
	return a.AgentID, nil
}
func (s *Service) BackupAgentHeartbeat(ctx context.Context, id string, status backup.AgentStatus) error {
	reg, ok := s.Config.Backups.Agent(id)
	if !ok || status.ID != id || status.Environment != reg.Environment || status.InstanceID != reg.InstanceID || status.ObservedAt.Before(s.Now().Add(-5*time.Minute)) || status.ObservedAt.After(s.Now().Add(time.Minute)) || status.PendingResults < 0 || status.PendingResults > 100000 || len(status.Versions) > 20 || (status.RunID != "" && !backup.ID(status.RunID)) || (status.Stage != "" && !backup.Text(status.Stage, 60)) {
		return Invalid("agent heartbeat is stale or mismatched")
	}
	// Only provisioned non-secret descriptors are returned to the UI. The reporting
	// process cannot add a new source/egress destination via its heartbeat.
	status.Profiles = reg.Profiles
	for name, version := range status.Versions {
		if !backup.ID(name) || !backup.Text(version, 200) {
			return Invalid("invalid tool version observation")
		}
	}
	_, e := s.DB.ExecContext(ctx, `INSERT INTO backup_agent_status(id,body,observed_at,received_at) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO UPDATE SET body=excluded.body,observed_at=excluded.observed_at,received_at=excluded.received_at WHERE backup_agent_status.observed_at<=excluded.observed_at`, id, string(backup.Canonical(status)), status.ObservedAt, s.Now())
	return e
}
func (s *Service) BackupAgentPolicy(ctx context.Context, id string) (backup.Signed, error) {
	var signed backup.Signed
	c := s.Config.Backups
	a, ok := c.Agent(id)
	if !ok {
		return signed, denied()
	}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		if e := exec(tx, ctx, `SELECT pg_advisory_xact_lock(818426004)`); e != nil {
			return e
		}
		p := backup.Policy{Version: 1, AgentID: id, Environment: a.Environment, IssuedAt: s.Now(), ExpiresAt: s.Now().Add(time.Duration(c.LeaseSeconds) * time.Second), Resources: []backup.Resource{}, Commands: []backup.Command{}}
		if e := tx.QueryRowContext(ctx, `SELECT nextval('backup_policy_serial')`).Scan(&p.Serial); e != nil {
			return e
		}
		rows, e := tx.QueryContext(ctx, `SELECT `+backupColumns+` FROM backup_resources r JOIN backup_entities e ON e.id=r.id WHERE (r.agent_id=$1 OR r.kind='retention') AND r.environment=$2 AND (r.state='approved' OR r.kind IN ('sources','destinations')) AND (r.kind<>'schedules' OR r.version=(SELECT max(r2.version) FROM backup_resources r2 WHERE r2.id=r.id AND r2.state='approved')) ORDER BY r.id,r.version LIMIT 2001`, id, a.Environment)
		if e != nil {
			return e
		}
		for rows.Next() {
			r, e := scanBackup(rows)
			if e != nil {
				rows.Close()
				return e
			}
			p.Resources = append(p.Resources, r)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(p.Resources) > 2000 {
			return conflict("agent policy exceeds supported resource limit; split agent scope")
		}
		jobs, e := tx.QueryContext(ctx, `SELECT body FROM backup_commands WHERE agent_id=$1 AND state='queued' AND expires_at>$2 ORDER BY created_at LIMIT 100`, id, s.Now())
		if e != nil {
			return e
		}
		for jobs.Next() {
			var raw []byte
			if e = jobs.Scan(&raw); e != nil {
				jobs.Close()
				return e
			}
			var job backup.Command
			if e = json.Unmarshal(raw, &job); e != nil {
				jobs.Close()
				return e
			}
			p.Commands = append(p.Commands, job)
		}
		e = jobs.Err()
		jobs.Close()
		if e != nil {
			return e
		}
		holdRows, err := tx.QueryContext(ctx, `SELECT point_id FROM backup_holds WHERE point_id IN (SELECT id FROM backup_results WHERE agent_id=$1) ORDER BY point_id LIMIT 10001`, id)
		if err != nil {
			return err
		}
		p.Holds = []string{}
		for holdRows.Next() {
			var point string
			if err = holdRows.Scan(&point); err != nil {
				holdRows.Close()
				return err
			}
			p.Holds = append(p.Holds, point)
		}
		err = holdRows.Err()
		holdRows.Close()
		if err != nil {
			return err
		}
		if len(p.Holds) > 10000 {
			return conflict("hold inventory exceeds agent bound")
		}
		signed, e = backup.Sign(p, c.SigningKey)
		if e != nil {
			return e
		}
		return exec(tx, ctx, `INSERT INTO backup_permits(digest,agent_id,serial,payload,issued_at,expires_at) VALUES($1,$2,$3,$4,$5,$6)`, backup.Digest(signed.Payload), id, p.Serial, string(signed.Payload), p.IssuedAt, p.ExpiresAt)
	})
	return signed, e
}

// Command claims are only required for materializing sensitive recovery data.
// A stale policy alone cannot begin a restore/export after cancellation or outage.
func (s *Service) BackupAgentClaim(ctx context.Context, agent, id string) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		if e := exec(tx, ctx, `SELECT pg_advisory_xact_lock(818426004)`); e != nil {
			return e
		}
		var state, operation, author string
		var expires time.Time
		var approved sql.NullString
		var raw []byte
		if e := tx.QueryRowContext(ctx, `SELECT state,operation,expires_at,approved_by,requested_by,body FROM backup_commands WHERE id=$1 AND agent_id=$2 FOR UPDATE`, id, agent).Scan(&state, &operation, &expires, &approved, &author, &raw); e != nil {
			return isMissing(e)
		}
		if state != "queued" || !expires.After(s.Now()) {
			return conflict("job is no longer eligible to start")
		}
		maker, e := s.user(ctx, tx, author, true)
		if e != nil {
			return e
		}
		if maker.Status != "active" || !maker.MFA || !slices.Contains(backupManageRoles, maker.Role) {
			return denied()
		}
		if slices.Contains([]string{"restore", "export", "forget"}, operation) {
			if !approved.Valid || approved.String == author {
				return denied()
			}
			checker, e := s.user(ctx, tx, approved.String, true)
			if e != nil {
				return e
			}
			if checker.Status != "active" || !checker.MFA || checker.Role != "admin" {
				return denied()
			}
		}
		var c backup.Command
		if e = json.Unmarshal(raw, &c); e != nil {
			return e
		}
		r, e := getBackup(ctx, tx, c.Resource)
		if e != nil {
			return e
		}
		if r.Disabled && !backup.ReadOperation(operation) || r.State != "approved" {
			return conflict("plan is no longer enabled")
		}
		if !backup.ReadOperation(operation) {
			if e = s.backupDependencies(ctx, tx, r, false); e != nil {
				return e
			}
		}
		if operation == "forget" {
			if !s.Config.Backups.MaintenanceAllowed {
				return denied()
			}
			var held, expired bool
			if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM backup_holds WHERE point_id=$1),EXISTS(SELECT 1 FROM backup_copy_expirations WHERE point_id=$1 AND copy_id=$2)`, c.PointID, c.CopyID).Scan(&held, &expired); e != nil {
				return e
			}
			if held || expired {
				return conflict("recovery point is held or this copy was already removed")
			}
		}
		return exec(tx, ctx, `UPDATE backup_commands SET state='unknown' WHERE id=$1`, id)
	})
}
func (s *Service) BackupAgentPoint(ctx context.Context, agent, command string) (backup.Result, error) {
	var out backup.Result
	e := s.transact(ctx, func(tx *sql.Tx) error {
		var id, state string
		var expires time.Time
		if e := tx.QueryRowContext(ctx, `SELECT point_id,state,expires_at FROM backup_commands WHERE id=$1 AND agent_id=$2`, command, agent).Scan(&id, &state, &expires); e != nil {
			return isMissing(e)
		}
		if !slices.Contains([]string{"queued", "unknown"}, state) || !expires.After(s.Now()) {
			return denied()
		}
		var e error
		out, e = s.readBackupResult(ctx, tx, id)
		if e == nil {
			e = overlayBackupExpirations(ctx, tx, &out)
		}
		return e
	})
	return out, e
}
func (s *Service) BackupAgentResult(ctx context.Context, agent string, signed backup.Signed) error {
	reg, ok := s.Config.Backups.Agent(agent)
	if !ok {
		return denied()
	}
	var r backup.Result
	if e := backup.Verify(signed, reg.PublicKey, &r); e != nil {
		return Invalid("invalid signed backup result")
	}
	if e := backup.ValidateResult(r); e != nil {
		return Invalid(e.Error())
	}
	if r.AgentID != agent || r.FinishedAt.After(s.Now().Add(time.Minute)) {
		return Invalid("agent result identity/time mismatch")
	}
	raw := string(backup.Canonical(r))
	signedDigest := backup.Digest(backup.Canonical(signed))
	return s.transact(ctx, func(tx *sql.Tx) error {
		if e := exec(tx, ctx, `SELECT pg_advisory_xact_lock(818426004)`); e != nil {
			return e
		}
		var prior string
		e := tx.QueryRowContext(ctx, `SELECT signed_digest FROM backup_results WHERE id=$1`, r.RunID).Scan(&prior)
		if e == nil {
			if prior == signedDigest {
				return nil
			}
			return conflict("the original backup occurrence already has different evidence")
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		var payload []byte
		if e = tx.QueryRowContext(ctx, `SELECT payload FROM backup_permits WHERE digest=$1 AND agent_id=$2`, r.PolicyDigest, agent).Scan(&payload); e != nil {
			return isMissing(e)
		}
		var p backup.Policy
		if e = json.Unmarshal(payload, &p); e != nil {
			return e
		}
		if r.StartedAt.Before(p.IssuedAt.Add(-time.Minute)) || r.StartedAt.After(p.ExpiresAt) || r.FinishedAt.After(p.ExpiresAt.Add(24*time.Hour)) {
			return Invalid("capture did not begin within its signed policy lease")
		}
		if e = backup.AuthorizeResult(p, r); e != nil {
			return Invalid(e.Error())
		}
		if r.Operation == "restore" || r.Operation == "export" || r.Operation == "forget" {
			var state string
			if err := tx.QueryRowContext(ctx, `SELECT state FROM backup_commands WHERE id=$1 AND agent_id=$2`, r.CommandID, agent).Scan(&state); err != nil {
				return isMissing(err)
			}
			if state != "unknown" {
				return conflict("sensitive execution requires a separately approved live claim")
			}
		}
		complete := false
		if slices.Contains([]string{"backup", "full", "diff", "incr"}, r.Operation) {
			complete = backup.RequiredComplete(p, r)
			if (r.State == "succeeded") != complete {
				return Invalid("result state contradicts mandatory component/copy evidence")
			}
		}
		sealed, e := s.Config.Box.Seal(raw, "backup-result:"+r.RunID)
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO backup_results(id,agent_id,policy_digest,plan_id,plan_version,command_id,schedule_id,schedule_version,operation,state,started_at,finished_at,copy_count,required_complete,error_code,body_enc,signed_digest,received_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, r.RunID, agent, r.PolicyDigest, r.Plan.ID, r.Plan.Version, r.CommandID, r.Schedule.ID, r.Schedule.Version, r.Operation, r.State, r.StartedAt, r.FinishedAt, len(r.Copies), complete, r.ErrorCode, sealed, signedDigest, s.Now()); e != nil {
			return e
		}
		if r.Operation == "forget" && r.State == "succeeded" {
			if e = exec(tx, ctx, `INSERT INTO backup_copy_expirations(point_id,copy_id,result_id) VALUES($1,$2,$3)`, r.PointID, r.ExpiredCopyID, r.RunID); e != nil {
				return e
			}
		}
		if r.CommandID != "" {
			if e = exec(tx, ctx, `UPDATE backup_commands SET state=$3 WHERE id=$1 AND agent_id=$2 AND state IN ('queued','unknown','cancelled')`, r.CommandID, agent, r.State); e != nil {
				return e
			}
			if r.Operation == "test" {
				for _, cmd := range p.Commands {
					if cmd.ID == r.CommandID {
						if e = exec(tx, ctx, `INSERT INTO backup_checks(resource_id,resource_version,run_id,state,checked_at) VALUES($1,$2,$3,$4,$5)`, cmd.Resource.ID, cmd.Resource.Version, r.RunID, r.State, r.FinishedAt); e != nil {
							return e
						}
					}
				}
			}
		}
		if e = s.audit(ctx, tx, "backup-agent:"+agent, "backup.result.recorded", r.RunID, map[string]any{"operation": r.Operation, "state": r.State, "required_complete": complete, "digest": signedDigest}); e != nil {
			return e
		}
		return s.backupNotify(ctx, tx, r)
	})
}
func (s *Service) backupNotify(ctx context.Context, tx *sql.Tx, r backup.Result) error {
	// Do not send a capture-completion claim for a probe, export, preview or verification.
	if r.State == "succeeded" && !slices.Contains([]string{"backup", "full", "diff", "incr"}, r.Operation) {
		return nil
	}
	if r.State == "materialized" && r.Operation != "restore" {
		return nil
	}
	workflow := "backup-run-completed"
	if r.State == "failed" || r.State == "unknown" {
		workflow = "backup-run-failed"
	} else if r.State == "partial" {
		workflow = "backup-run-partial"
	} else if r.Operation == "restore" {
		workflow = "backup-restore-materialized"
	}
	if _, ok := NotificationMetadata[workflow]; !ok {
		return nil
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM users WHERE role IN ('admin','platform') AND status='active' AND verified AND mfa_enabled ORDER BY id LIMIT 100`)
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if e = s.notify(ctx, tx, id, workflow, r.RunID, map[string]any{}, false); e != nil {
			return e
		}
	}
	return nil
}
