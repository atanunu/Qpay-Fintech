package backup

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
)

func Find(p Policy, ref Ref) (Resource, bool) {
	for _, r := range p.Resources {
		if r.ID == ref.ID && r.Version == ref.Version {
			return r, true
		}
	}
	return Resource{}, false
}
func CommandRunID(id string) string { return "br_" + Digest([]byte("command/" + id))[:40] }
func AllowedError(s string) bool {
	return slices.Contains([]string{"", "capture_failed", "copy_failed", "verification_failed", "restore_failed", "profile_unavailable", "policy_expired", "interrupted", "quota_exceeded", "command_unavailable", "repository_unavailable", "retention_blocked", "invalid_inventory"}, s)
}
func ValidateResult(r Result) error {
	if r.Version != 1 || !ID(r.AgentID) || !ID(r.RunID) || !digestPattern.MatchString(r.PolicyDigest) || !r.Plan.Valid() || r.StartedAt.IsZero() || r.FinishedAt.Before(r.StartedAt) || r.FinishedAt.Sub(r.StartedAt) > 24*time.Hour || len(r.Copies) > 200 || len(r.Files) > 5000 || !AllowedError(r.ErrorCode) {
		return errors.New("invalid or oversized backup result")
	}
	if !slices.Contains([]string{"succeeded", "failed", "partial", "unknown", "materialized"}, r.State) {
		return errors.New("unsupported backup result state")
	}
	if r.Retention != nil {
		if len(r.Retention.Items) > 1000 || r.Retention.Plan != r.Plan || r.Retention.Digest != retentionDigest(*r.Retention) || !slices.Contains([]string{"retention_preview", "forget"}, r.Operation) {
			return errors.New("invalid retention preview")
		}
		for _, item := range r.Retention.Items {
			if !ID(item.PointID) || len(item.Reason) > 200 {
				return errors.New("invalid retention evidence")
			}
		}
	}
	if r.ExpiredCopyID != "" && (r.Operation != "forget" || !ID(r.ExpiredCopyID)) {
		return errors.New("invalid copy expiration")
	}
	if r.State == "succeeded" && r.Operation == "forget" && (r.Retention == nil || r.ExpiredCopyID == "") {
		return errors.New("successful removal requires exact reviewed retention and copy evidence")
	}
	if r.State == "materialized" && r.MaterializedTarget == "" {
		return errors.New("materialization requires a bound target")
	}
	seen := map[string]bool{}
	for _, c := range r.Copies {
		if !ID(c.ID) || seen[c.ID] || !c.Source.Valid() || !c.Destination.Valid() || !slices.Contains([]string{"stored", "failed"}, c.State) || !AllowedError(c.ErrorCode) || !slices.Contains([]string{"none", "metadata", "read_data"}, c.Verified) || len(c.Snapshot) > 100 || !slices.Contains([]string{"restic", "pgbackrest"}, c.Engine) {
			return errors.New("invalid copy evidence")
		}
		if len(c.Dependencies) > 1000 || len(c.WALStart) > 100 || len(c.WALStop) > 100 {
			return errors.New("oversized physical recovery dependencies")
		}
		for _, label := range c.Dependencies {
			if !physicalLabel.MatchString(label) {
				return errors.New("invalid physical recovery dependency")
			}
		}
		if c.Engine != "pgbackrest" && (len(c.Dependencies) != 0 || c.WALStart != "" || c.WALStop != "") {
			return errors.New("non-physical copy cannot claim WAL coverage")
		}
		seen[c.ID] = true
		if c.State == "stored" && (c.Engine == "restic" && !digestPattern.MatchString(c.Snapshot) || c.Engine == "pgbackrest" && !physicalLabel.MatchString(c.Snapshot)) {
			return errors.New("invalid finalized snapshot identity")
		}
		if c.State == "stored" && (c.Snapshot == "" || c.CaptureFinished.Before(c.CaptureStarted) || c.CaptureStarted.Before(r.StartedAt.Add(-time.Second)) || c.CaptureFinished.After(r.FinishedAt.Add(time.Second))) {
			return errors.New("incomplete copy boundary")
		}
		if _, e := strconv.ParseUint(c.Bytes, 10, 63); e != nil {
			return errors.New("copy bytes must be an unsigned decimal string")
		}
	}
	for _, f := range r.Files {
		if f.Path == "" || len(f.Path) > 1000 || path.IsAbs(f.Path) || path.Clean(f.Path) != f.Path || f.Path == ".." || strings.HasPrefix(f.Path, "../") || strings.ContainsAny(f.Path, "\\\x00") || !ID(f.SourceID) || f.SHA256 != "" && !digestPattern.MatchString(f.SHA256) || len(f.ObjectVersion) > 200 {
			return errors.New("invalid protected inventory path")
		}
		if _, e := strconv.ParseUint(f.Size, 10, 63); e != nil {
			return errors.New("invalid inventory size")
		}
	}
	return nil
}
func AuthorizeResult(p Policy, r Result) error {
	if r.CommandID != "" {
		for _, c := range p.Commands {
			if c.ID == r.CommandID && c.Approved && c.Operation == r.Operation && (r.ExpiredCopyID == "" || r.ExpiredCopyID == c.CopyID) && (r.Operation != "forget" || r.Retention == nil || r.Retention.Digest == c.PreviewDigest) && c.Resource == r.Plan && c.PointID == r.PointID && (r.MaterializedTarget == "" || r.MaterializedTarget == c.Target+":"+c.ID) && r.RunID == CommandRunID(c.ID) && r.StartedAt.Before(c.ExpiresAt) {
				return authorizeCopies(p, r)
			}
		}
		return errors.New("result is not bound to a signed command")
	}
	s, ok := Find(p, r.Schedule)
	if !ok || s.Kind != "schedules" || s.State != "approved" || s.Disabled {
		return errors.New("unknown or inactive signed schedule")
	}
	v := Parse[ScheduleSpec](s)
	if v.Plan != r.Plan || v.Operation != r.Operation || r.RunID != OccurrenceID(r.Schedule, r.Occurrence) || r.Occurrence.After(r.StartedAt) || r.StartedAt.Sub(r.Occurrence) > 366*24*time.Hour {
		return errors.New("result does not match its schedule occurrence")
	}
	next, e := Next(v, r.Occurrence.Add(-time.Nanosecond), 1)
	if e != nil || len(next) != 1 || !next[0].Equal(r.Occurrence) {
		return errors.New("invalid schedule occurrence time")
	}
	return authorizeCopies(p, r)
}
func authorizeCopies(p Policy, r Result) error {
	if r.State == "materialized" && r.Operation != "restore" && r.Operation != "export" {
		return errors.New("only recovery operations materialize private data")
	}
	if r.Operation != "backup" && r.Operation != "full" && r.Operation != "diff" && r.Operation != "incr" && (len(r.Copies) > 0 || len(r.Files) > 0) {
		return errors.New("operation must not invent a new capture")
	}
	plan, ok := Find(p, r.Plan)
	if !ok {
		return errors.New("missing signed resource")
	}
	if r.Operation == "test" {
		if plan.Kind != "sources" && plan.Kind != "destinations" || len(r.Copies) != 0 || len(r.Files) != 0 {
			return errors.New("probe must not return customer backup contents")
		}
		return nil
	}
	if plan.Kind != "plans" || plan.State != "approved" || plan.Disabled && !ReadOperation(r.Operation) {
		return errors.New("plan is not approved")
	}
	v := Parse[PlanSpec](plan)
	seen := map[string]bool{}
	for _, c := range r.Copies {
		src, sok := Find(p, c.Source)
		dst, dok := Find(p, c.Destination)
		if !sok || !dok || src.Disabled || dst.Disabled || src.State != "approved" || dst.State != "approved" || !slices.Contains(v.Sources, c.Source) {
			return errors.New("unapproved source or destination")
		}
		found := false
		for _, b := range v.Destinations {
			found = found || b.Destination == c.Destination
		}
		pair := c.Source.Key() + "/" + c.Destination.Key()
		if !found || seen[pair] {
			return errors.New("duplicate or unrelated copy")
		}
		seen[pair] = true
	}
	for _, f := range r.Files {
		found := false
		for _, src := range v.Sources {
			found = found || src.ID == f.SourceID
		}
		if !found {
			return errors.New("inventory belongs to another source")
		}
	}
	return nil
}
func RequiredComplete(p Policy, r Result) bool {
	plan, ok := Find(p, r.Plan)
	if !ok || plan.Kind != "plans" {
		return false
	}
	v := Parse[PlanSpec](plan)
	if len(v.Sources) == 0 || len(v.Destinations) == 0 {
		return false
	}
	for _, src := range v.Sources {
		domains := map[string]bool{}
		for _, binding := range v.Destinations {
			if !binding.Required {
				continue
			}
			found := false
			for _, c := range r.Copies {
				if c.Source == src && c.Destination == binding.Destination && c.State == "stored" && (c.Verified == v.Verify || c.Verified == "read_data") {
					found = true
				}
			}
			if !found {
				return false
			}
			d, ok := Find(p, binding.Destination)
			if !ok {
				return false
			}
			domains[Parse[DestinationSpec](d).FailureDomain] = true
		}
		if len(domains) < v.MinFailureDomains {
			return false
		}
	}
	return true
}
func CopyID(run string, source, destination int) string {
	return fmt.Sprintf("cp_%s_%d_%d", strings.TrimPrefix(run, "br_"), source, destination)
}

// ReadOperation may inspect historical copies after capture has been paused.
func ReadOperation(op string) bool {
	return slices.Contains([]string{"verify", "restore", "export", "retention_preview"}, op)
}
