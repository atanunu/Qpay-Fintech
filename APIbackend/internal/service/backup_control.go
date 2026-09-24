package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/backup"
)

type BackupResourceInput struct {
	Name        string          `json:"name"`
	Environment string          `json:"environment"`
	AgentID     string          `json:"agent_id"`
	BaseVersion int64           `json:"base_version"`
	Spec        json.RawMessage `json:"spec"`
	Reason      string          `json:"reason"`
	Submit      bool            `json:"submit"`
}
type BackupDecision struct {
	Version  int64  `json:"version"`
	Digest   string `json:"digest"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}
type BackupCommandInput struct {
	Operation     string     `json:"operation"`
	Resource      backup.Ref `json:"resource"`
	PointID       string     `json:"point_id"`
	CopyID        string     `json:"copy_id"`
	Target        string     `json:"target"`
	Reason        string     `json:"reason"`
	RecoveryTime  *time.Time `json:"recovery_time,omitempty"`
	PreviewRunID  string     `json:"preview_run_id"`
	PreviewDigest string     `json:"preview_digest"`
}

var backupViewRoles = []string{"admin", "platform", "auditor"}
var backupManageRoles = []string{"admin", "platform"}

const backupColumns = `r.id,r.kind,r.name,r.environment,r.agent_id,r.version,r.state,e.disabled,r.digest,r.spec,r.created_by,coalesce(r.approved_by,''),r.created_at,r.decided_at`

func scanBackup(row interface{ Scan(...any) error }) (backup.Resource, error) {
	var r backup.Resource
	var decided sql.NullTime
	e := row.Scan(&r.ID, &r.Kind, &r.Name, &r.Environment, &r.AgentID, &r.Version, &r.State, &r.Disabled, &r.Digest, &r.Spec, &r.CreatedBy, &r.ApprovedBy, &r.CreatedAt, &decided)
	if decided.Valid {
		r.DecidedAt = &decided.Time
	}
	return r, isMissing(e)
}
func getBackup(ctx context.Context, tx *sql.Tx, ref backup.Ref) (backup.Resource, error) {
	return scanBackup(tx.QueryRowContext(ctx, `SELECT `+backupColumns+` FROM backup_resources r JOIN backup_entities e ON e.id=r.id WHERE r.id=$1 AND r.version=$2`, ref.ID, ref.Version))
}
func backupReason(reason string) error {
	if !safeText(reason, 1000) || len(strings.TrimSpace(reason)) < 8 {
		return Invalid("explain the purpose in 8 to 1000 characters; do not include secrets or customer details")
	}
	return nil
}
func (s *Service) BackupBootstrap(ctx context.Context, p Principal) (map[string]any, error) {
	out := map[string]any{}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = s.consolePrincipal(ctx, tx, p, backupViewRoles...)
		if err != nil {
			return err
		}
		agents := []map[string]any{}
		lease := 0
		if c := s.Config.Backups; c != nil {
			lease = c.LeaseSeconds
			for _, a := range c.Agents {
				agents = append(agents, map[string]any{"id": a.ID, "environment": a.Environment, "instance_id": a.InstanceID, "profiles": a.Profiles})
			}
		}
		out = map[string]any{"enabled": s.Config.Backups != nil, "agents": agents, "providers": backup.Providers, "can_manage": slices.Contains(backupManageRoles, p.User.Role), "can_approve": p.User.Role == "admin", "can_inventory": p.User.Role == "admin" || p.User.Role == "platform", "maintenance_enabled": s.Config.Backups != nil && s.Config.Backups.MaintenanceAllowed, "lease_seconds": lease, "server_time": s.Now(), "production_activated": false, "instructions": "Backups are executed by separately provisioned agents. Drafts and tests do not enable protection. A second administrator approves exact revisions. Keys stay outside this database."}
		return s.audit(ctx, tx, p.User.ID, "backup.bootstrap", "backups", map[string]any{})
	})
	return out, e
}
func (s *Service) BackupList(ctx context.Context, p Principal, kind string, q AdminQuery) (AdminPage, error) {
	out := AdminPage{Items: []map[string]any{}, GeneratedAt: s.Now()}
	if q.Limit < 1 || q.Limit > 100 || len(q.Search) > 100 || len(q.Before) > 150 {
		return out, Invalid("invalid backup page bounds")
	}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = s.consolePrincipal(ctx, tx, p, backupViewRoles...)
		if err != nil {
			return err
		}
		var query string
		switch kind {
		case "sources", "destinations", "plans", "schedules", "retention":
			rows, err := tx.QueryContext(ctx, `SELECT `+backupColumns+` FROM backup_entities e JOIN backup_resources r ON r.id=e.id AND r.version=e.latest_version WHERE e.kind=$1 AND ($2='' OR r.id<$2) AND ($3='' OR position(lower($3) in lower(r.name||' '||r.environment||' '||r.state||' '||r.agent_id))>0) ORDER BY r.id DESC LIMIT $4`, kind, q.Before, q.Search, q.Limit+1)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				r, e := scanBackup(rows)
				if e != nil {
					return e
				}
				b := backup.Canonical(r)
				var row map[string]any
				json.Unmarshal(b, &row)
				out.Items = append(out.Items, row)
			}
			if e := rows.Err(); e != nil {
				return e
			}
		case "runs", "points":
			query = `SELECT id,agent_id,plan_id,plan_version,operation,state,started_at,finished_at,copy_count,required_complete,error_code,received_at FROM backup_results`
			if kind == "points" {
				query += ` WHERE operation IN ('backup','full','diff','incr')`
			}
		case "jobs", "restores":
			query = `SELECT id,agent_id,operation,resource_id,resource_version,point_id,copy_id,target,digest,requested_by,approved_by,state,created_at,expires_at FROM backup_commands`
			if kind == "restores" {
				query += ` WHERE operation IN ('restore','export')`
			}
		case "agents":
			query = `SELECT id,body,observed_at,received_at FROM backup_agent_status`
		default:
			return missing()
		}
		if query != "" {
			out.Items, err = readAdminRows(ctx, tx, `SELECT row_to_json(t) FROM (`+query+`) t WHERE ($1='' OR id<$1) AND ($2='' OR position(lower($2) in lower(to_jsonb(t)::text))>0) ORDER BY id DESC LIMIT $3`, q.Before, q.Search, q.Limit+1)
			if err != nil {
				return err
			}
		}
		out.HasMore = len(out.Items) > q.Limit
		if out.HasMore {
			out.Items = out.Items[:q.Limit]
			out.NextBefore = out.Items[len(out.Items)-1]["id"].(string)
		}
		return s.audit(ctx, tx, p.User.ID, "backup.list", kind, map[string]any{"count": len(out.Items)})
	})
	return out, e
}
func (s *Service) BackupVersions(ctx context.Context, p Principal, kind, id string) ([]backup.Resource, error) {
	out := []backup.Resource{}
	if !backup.Kind(kind) || !backup.ID(id) {
		return out, missing()
	}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = s.consolePrincipal(ctx, tx, p, backupViewRoles...)
		if err != nil {
			return err
		}
		rows, e := tx.QueryContext(ctx, `SELECT `+backupColumns+` FROM backup_resources r JOIN backup_entities e ON e.id=r.id WHERE r.id=$1 AND r.kind=$2 ORDER BY r.version DESC LIMIT 100`, id, kind)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			r, e := scanBackup(rows)
			if e != nil {
				return e
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, e
}
func (s *Service) BackupSave(ctx context.Context, p Principal, kind, id string, in BackupResourceInput) (backup.Resource, error) {
	var out backup.Resource
	if !backup.Kind(kind) || id != "" && !backup.ID(id) {
		return out, missing()
	}
	if e := backupReason(in.Reason); e != nil {
		return out, e
	}
	if !safeText(in.Name, 140) || !slices.Contains([]string{"local", "staging", "production"}, in.Environment) || kind != "retention" && !backup.ID(in.AgentID) || kind == "retention" && in.AgentID != "" {
		return out, Invalid("name, environment and the correct agent assignment are required")
	}
	spec, e := backup.ValidateSpec(kind, in.Spec)
	if e != nil {
		return out, Invalid(e.Error())
	}
	if e = s.RequireStaffElevation(ctx, p); e != nil {
		return out, e
	}
	creating := id == ""
	if creating {
		id = stringID("bak_")
	}
	e = s.transact(ctx, func(tx *sql.Tx) error {
		var err error
		p, err = s.consolePrincipal(ctx, tx, p, backupManageRoles...)
		if err != nil {
			return err
		}
		if err = exec(tx, ctx, `SELECT pg_advisory_xact_lock(818426004)`); err != nil {
			return err
		}
		version := int64(1)
		if creating {
			if in.BaseVersion != 0 {
				return conflict("new resources have no previous version")
			}
			if err = exec(tx, ctx, `INSERT INTO backup_entities(id,kind,latest_version) VALUES($1,$2,1)`, id, kind); err != nil {
				return err
			}
		} else {
			var prior int64
			var oldKind string
			if err = tx.QueryRowContext(ctx, `SELECT kind,latest_version FROM backup_entities WHERE id=$1 FOR UPDATE`, id).Scan(&oldKind, &prior); err != nil {
				return isMissing(err)
			}
			if oldKind != kind || prior != in.BaseVersion {
				return conflict("configuration changed; reload before editing")
			}
			version = prior + 1
			if err = exec(tx, ctx, `UPDATE backup_entities SET latest_version=$2 WHERE id=$1`, id, version); err != nil {
				return err
			}
		}
		state := "draft"
		if in.Submit {
			state = "pending"
		}
		out = backup.Resource{ID: id, Kind: kind, Name: in.Name, Environment: in.Environment, AgentID: in.AgentID, Version: version, State: state, Spec: spec, CreatedBy: p.User.ID, CreatedAt: s.Now()}
		out.Digest = backup.Digest(backup.Canonical(struct {
			ID, Kind, Name, Environment, Agent string
			Version                            int64
			Spec                               json.RawMessage
		}{id, kind, in.Name, in.Environment, in.AgentID, version, spec}))
		if in.Submit {
			if err = s.backupDependencies(ctx, tx, out, false); err != nil {
				return err
			}
		}
		if err = exec(tx, ctx, `INSERT INTO backup_resources(id,version,kind,name,environment,agent_id,spec,digest,state,created_by,reason,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, id, version, kind, in.Name, in.Environment, in.AgentID, string(spec), out.Digest, state, p.User.ID, in.Reason, s.Now()); err != nil {
			return err
		}
		return s.audit(ctx, tx, p.User.ID, "backup.revision.created", id, map[string]any{"kind": kind, "version": version, "digest": out.Digest, "state": state})
	})
	return out, e
}
func (s *Service) backupDependencies(ctx context.Context, tx *sql.Tx, r backup.Resource, requireTest bool) error {
	if r.Kind != "retention" {
		a, ok := s.Config.Backups.Agent(r.AgentID)
		if !ok || a.Environment != r.Environment {
			return Invalid("provision an agent in this environment before submitting")
		}
	}
	profile := func(kind, id, typ, provider string) (backup.Profile, error) {
		p, ok := s.Config.Backups.Profile(r.AgentID, kind, id)
		if !ok || typ != "" && p.Type != typ || provider != "" && p.Provider != provider {
			return p, Invalid("profile is not provisioned for this agent and source/provider type")
		}
		return p, nil
	}
	ref := func(b backup.Ref, kind string) (backup.Resource, error) {
		d, e := getBackup(ctx, tx, b)
		if e != nil {
			return d, e
		}
		if d.Kind != kind || d.State != "approved" || d.Disabled || d.Environment != r.Environment || kind != "retention" && d.AgentID != r.AgentID {
			return d, Invalid("dependency must be an enabled approved exact revision in the same environment and agent")
		}
		return d, nil
	}
	switch r.Kind {
	case "sources":
		v := backup.Parse[backup.SourceSpec](r)
		if _, e := profile("source", v.Profile, v.Type, ""); e != nil {
			return e
		}
	case "destinations":
		v := backup.Parse[backup.DestinationSpec](r)
		p, e := profile("destination", v.Profile, "", v.Provider)
		if e != nil {
			return e
		}
		if p.FailureDomain != v.FailureDomain || p.OffHost != v.OffHost || p.Region != v.Region {
			return Invalid("failure domain, off-host status and region must match the provisioned profile")
		}
	case "plans":
		v := backup.Parse[backup.PlanSpec](r)
		if _, e := ref(v.Retention, "retention"); e != nil {
			return e
		}
		physical := false
		for _, b := range v.Sources {
			d, e := ref(b, "sources")
			if e != nil {
				return e
			}
			if e = s.backupDependencies(ctx, tx, d, false); e != nil {
				return e
			}
			physical = physical || backup.Parse[backup.SourceSpec](d).Type == "postgres_physical"
		}
		if physical && v.Verify != "metadata" {
			return Invalid("physical capture reports metadata verification; request a separate full repository check and isolated restore drill")
		}
		if physical && len(v.Sources) != 1 {
			return Invalid("physical PostgreSQL plans must have one cluster source; each repository has its own recovery boundary")
		}
		domains := map[string]bool{}
		offhost := false
		for _, b := range v.Destinations {
			d, e := ref(b.Destination, "destinations")
			if e != nil {
				return e
			}
			if e = s.backupDependencies(ctx, tx, d, false); e != nil {
				return e
			}
			spec := backup.Parse[backup.DestinationSpec](d)
			if (spec.Provider == "pgbackrest") != physical {
				return Invalid("destination engine is incompatible with this source plan")
			}
			if b.Required {
				domains[spec.FailureDomain] = true
				offhost = offhost || spec.OffHost
			}
		}
		if len(domains) < v.MinFailureDomains {
			return Invalid("required copies must cover the configured number of independent failure domains")
		}
		if r.Environment == "production" && (!offhost || v.MinFailureDomains < 2) {
			return Invalid("production plans require at least two required failure domains including off-host protection")
		}
	case "schedules":
		v := backup.Parse[backup.ScheduleSpec](r)
		plan, e := ref(v.Plan, "plans")
		if e != nil {
			return e
		}
		if e = s.backupDependencies(ctx, tx, plan, false); e != nil {
			return e
		}
		ps := backup.Parse[backup.PlanSpec](plan)
		source, e := ref(ps.Sources[0], "sources")
		if e != nil {
			return e
		}
		physical := backup.Parse[backup.SourceSpec](source).Type == "postgres_physical"
		if v.Operation != "verify" && (physical && !slices.Contains([]string{"full", "diff", "incr"}, v.Operation) || !physical && v.Operation != "backup") {
			return Invalid("schedule operation is incompatible with its plan engine")
		}
	}
	if requireTest && (r.Kind == "sources" || r.Kind == "destinations") {
		var ok bool
		e := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM backup_checks WHERE resource_id=$1 AND resource_version=$2 AND state='succeeded' AND checked_at>$3)`, r.ID, r.Version, s.Now().Add(-24*time.Hour)).Scan(&ok)
		if e != nil {
			return e
		}
		if !ok {
			return conflict("run and pass a connection test for this exact revision before approval")
		}
	}
	return nil
}
func (s *Service) BackupDecide(ctx context.Context, p Principal, kind, id string, in BackupDecision) (backup.Resource, error) {
	var out backup.Resource
	if !backup.Kind(kind) || !backup.ID(id) {
		return out, missing()
	}
	if e := backupReason(in.Reason); e != nil {
		return out, e
	}
	if e := s.RequireStaffElevation(ctx, p); e != nil {
		return out, e
	}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		roles := backupManageRoles
		if in.Decision != "submit" {
			roles = []string{"admin"}
		}
		var e error
		p, e = s.consolePrincipal(ctx, tx, p, roles...)
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `SELECT pg_advisory_xact_lock(818426004)`); e != nil {
			return e
		}
		out, e = getBackup(ctx, tx, backup.Ref{ID: id, Version: in.Version})
		if e != nil {
			return e
		}
		if out.Kind != kind || out.Digest != in.Digest {
			return conflict("exact configuration digest and version required")
		}
		var latest int64
		if e = tx.QueryRowContext(ctx, `SELECT latest_version FROM backup_entities WHERE id=$1 FOR UPDATE`, id).Scan(&latest); e != nil {
			return e
		}
		if latest != in.Version {
			return conflict("newer revision exists; reload")
		}
		if in.Decision == "submit" {
			if out.State != "draft" || out.CreatedBy != p.User.ID {
				return denied()
			}
			if e = s.backupDependencies(ctx, tx, out, false); e != nil {
				return e
			}
			out.State = "pending"
			if e = exec(tx, ctx, `UPDATE backup_resources SET state='pending' WHERE id=$1 AND version=$2`, id, in.Version); e != nil {
				return e
			}
		} else {
			if out.State != "pending" || p.User.ID == out.CreatedBy {
				return conflict("an independent administrator must decide this pending revision")
			}
			maker, e := s.user(ctx, tx, out.CreatedBy, true)
			if e != nil {
				return e
			}
			if !maker.MFA || maker.Status != "active" || !slices.Contains(backupManageRoles, maker.Role) {
				return denied()
			}
			if in.Decision != "approve" && in.Decision != "reject" {
				return Invalid("choose submit, approve or reject")
			}
			out.State = "rejected"
			if in.Decision == "approve" {
				if e = s.backupDependencies(ctx, tx, out, true); e != nil {
					return e
				}
				out.State = "approved"
			}
			if e = exec(tx, ctx, `UPDATE backup_resources SET state=$3,approved_by=$4,decision_reason=$5,decided_at=$6 WHERE id=$1 AND version=$2`, id, in.Version, out.State, p.User.ID, in.Reason, s.Now()); e != nil {
				return e
			}
			if out.State == "approved" {
				if e = exec(tx, ctx, `UPDATE backup_entities SET disabled=false WHERE id=$1`, id); e != nil {
					return e
				}
			}
			out.ApprovedBy = p.User.ID
			now := s.Now()
			out.DecidedAt = &now
		}
		return s.audit(ctx, tx, p.User.ID, "backup.revision."+in.Decision, id, map[string]any{"version": in.Version, "digest": in.Digest})
	})
	return out, e
}
func (s *Service) BackupPause(ctx context.Context, p Principal, id string, in BackupDecision) error {
	if e := backupReason(in.Reason); e != nil {
		return e
	}
	if e := s.RequireStaffElevation(ctx, p); e != nil {
		return e
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var e error
		p, e = s.consolePrincipal(ctx, tx, p, backupManageRoles...)
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `SELECT pg_advisory_xact_lock(818426004)`); e != nil {
			return e
		}
		r, e := getBackup(ctx, tx, backup.Ref{ID: id, Version: in.Version})
		if e != nil {
			return e
		}
		if r.Digest != in.Digest {
			return conflict("configuration changed")
		}
		res, e := tx.ExecContext(ctx, `UPDATE backup_entities SET disabled=true WHERE id=$1 AND latest_version=$2`, id, in.Version)
		if e != nil {
			return e
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			return conflict("reload the current revision")
		}
		return s.audit(ctx, tx, p.User.ID, "backup.resource.paused", id, map[string]any{"version": in.Version, "reason": in.Reason, "lease_notice": "offline agents may continue until the current signed policy lease expires"})
	})
}
func (s *Service) BackupOverview(ctx context.Context, p Principal) (map[string]any, error) {
	out := map[string]any{}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		var e error
		p, e = s.consolePrincipal(ctx, tx, p, backupViewRoles...)
		if e != nil {
			return e
		}
		counts, e := readAdminRows(ctx, tx, `SELECT row_to_json(t) FROM (SELECT e.kind,count(*) AS total,count(*) FILTER(WHERE r.state='approved' AND NOT e.disabled) AS enabled FROM backup_entities e JOIN backup_resources r ON r.id=e.id AND r.version=e.latest_version GROUP BY e.kind) t`)
		if e != nil {
			return e
		}
		points, e := readAdminRows(ctx, tx, `SELECT row_to_json(t) FROM (SELECT DISTINCT ON(plan_id) plan_id,plan_version,finished_at,state,copy_count,required_complete FROM backup_results WHERE operation IN ('backup','full','diff','incr') ORDER BY plan_id,finished_at DESC) t`)
		if e != nil {
			return e
		}
		var failures, pending int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM backup_results WHERE state IN ('failed','partial','unknown') AND finished_at>$1`, s.Now().Add(-24*time.Hour)).Scan(&failures); e != nil {
			return e
		}
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM backup_commands WHERE state='pending' AND expires_at>$1`, s.Now()).Scan(&pending); e != nil {
			return e
		}
		out = map[string]any{"resources": counts, "latest_points": points, "failures_24h": failures, "pending_approvals": pending, "generated_at": s.Now(), "rpo_claim": "No continuous recovery coverage is inferred from upload completion. Consult measured WAL coverage and isolated recovery validation."}
		return nil
	})
	return out, e
}
func (s *Service) BackupPoint(ctx context.Context, p Principal, id, reason string) (backup.Result, error) {
	var out backup.Result
	if e := backupReason(reason); e != nil {
		return out, e
	}
	if e := s.RequireStaffElevation(ctx, p); e != nil {
		return out, e
	}
	e := s.transact(ctx, func(tx *sql.Tx) error {
		var e error
		p, e = s.consolePrincipal(ctx, tx, p, backupManageRoles...)
		if e != nil {
			return e
		}
		var ciphertext string
		if e = tx.QueryRowContext(ctx, `SELECT body_enc FROM backup_results WHERE id=$1`, id).Scan(&ciphertext); e != nil {
			return isMissing(e)
		}
		raw, e := s.Config.Box.Open(ciphertext, "backup-result:"+id)
		if e != nil {
			return e
		}
		if e = json.Unmarshal([]byte(raw), &out); e != nil {
			return e
		}
		if e = overlayBackupExpirations(ctx, tx, &out); e != nil {
			return e
		}
		return s.audit(ctx, tx, p.User.ID, "backup.inventory.read", id, map[string]any{"reason": reason, "count": len(out.Files)})
	})
	return out, e
}
func (s *Service) BackupHold(ctx context.Context, p Principal, id, reason string) error {
	if e := backupReason(reason); e != nil {
		return e
	}
	if e := s.RequireStaffElevation(ctx, p); e != nil {
		return e
	}
	return s.transact(ctx, func(tx *sql.Tx) error {
		var e error
		p, e = s.consolePrincipal(ctx, tx, p, "admin")
		if e != nil {
			return e
		}
		if e = exec(tx, ctx, `SELECT pg_advisory_xact_lock(818426004)`); e != nil {
			return e
		}
		var inFlight bool
		if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM backup_commands WHERE point_id=$1 AND operation='forget' AND state='unknown')`, id).Scan(&inFlight); e != nil {
			return e
		}
		if inFlight {
			return conflict("a removal already started; investigate it before recording protection")
		}
		if e = exec(tx, ctx, `UPDATE backup_commands SET state='cancelled' WHERE point_id=$1 AND operation='forget' AND state IN ('queued','pending')`, id); e != nil {
			return e
		}
		if e = exec(tx, ctx, `INSERT INTO backup_holds(point_id,reason,actor_id) VALUES($1,$2,$3) ON CONFLICT(point_id) DO NOTHING`, id, reason, p.User.ID); e != nil {
			return e
		}
		return s.audit(ctx, tx, p.User.ID, "backup.hold.created", id, map[string]any{"reason": reason})
	})
}

// Helpers used by the agent gateway keep all errors safe for staff and agents.
func backupFault(e error) error {
	if e == nil {
		return nil
	}
	var f *Fault
	if errors.As(e, &f) {
		return e
	}
	return e
}
