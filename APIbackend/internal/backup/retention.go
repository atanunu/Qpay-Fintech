package backup

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"
)

type RetentionItem struct {
	PointID    string    `json:"point_id"`
	CapturedAt time.Time `json:"captured_at"`
	Eligible   bool      `json:"eligible"`
	Reason     string    `json:"reason"`
}
type RetentionPreview struct {
	Plan   Ref             `json:"plan"`
	Policy Ref             `json:"policy"`
	Items  []RetentionItem `json:"items"`
	Digest string          `json:"digest"`
}

func retentionDigest(p RetentionPreview) string { p.Digest = ""; return Digest(Canonical(p)) }
func PlanRetention(p Policy, ref Ref, history []JournalRun, now time.Time) (RetentionPreview, error) {
	plan, ok := Find(p, ref)
	if !ok || plan.Kind != "plans" || plan.State != "approved" {
		return RetentionPreview{}, errors.New("approved plan required")
	}
	ps := Parse[PlanSpec](plan)
	policy, ok := Find(p, ps.Retention)
	if !ok || policy.Kind != "retention" || policy.State != "approved" {
		return RetentionPreview{}, errors.New("historical retention policy missing")
	}
	spec := Parse[RetentionSpec](policy)
	if _, err := ValidateSpec("retention", policy.Spec); err != nil {
		return RetentionPreview{}, err
	}
	points := []Result{}
	seen := map[string]bool{}
	for _, entry := range history {
		r := entry.Result
		if !entry.Complete || r.Plan != ref || !slices.Contains([]string{"backup", "full", "diff", "incr"}, r.Operation) {
			continue
		}
		if seen[r.RunID] {
			return RetentionPreview{}, errors.New("duplicate recovery inventory")
		}
		seen[r.RunID] = true
		points = append(points, r)
		if len(points) > 1000 {
			return RetentionPreview{}, errors.New("retention inventory exceeds review bound; no deletion is authorized")
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].FinishedAt.After(points[j].FinishedAt) })
	keep := map[string]string{}
	good := []Result{}
	for _, r := range points {
		if r.State == "succeeded" && RequiredComplete(p, r) {
			good = append(good, r)
		} else {
			keep[r.RunID] = "Incomplete or uncertain point requires investigation"
		}
		if now.Sub(r.FinishedAt) < time.Duration(spec.MinimumDays)*24*time.Hour {
			keep[r.RunID] = "Minimum recovery window"
		}
		if slices.Contains(p.Holds, r.RunID) {
			keep[r.RunID] = "Protected by an active hold"
		}
	}
	for i, r := range good {
		if i < spec.KeepLast {
			keep[r.RunID] = "Latest required complete recovery points"
		}
	}
	bucket := func(level string, n int) {
		keys := map[string]bool{}
		for _, r := range good {
			t := r.FinishedAt.UTC()
			key := ""
			switch level {
			case "hourly":
				key = t.Format("2006-01-02T15")
			case "daily":
				key = t.Format("2006-01-02")
			case "weekly":
				y, w := t.ISOWeek()
				key = fmt.Sprintf("%04d-W%02d", y, w)
			case "monthly":
				key = t.Format("2006-01")
			}
			if !keys[key] {
				if len(keys) >= n {
					break
				}
				keys[key] = true
				keep[r.RunID] = "Retained " + level + " recovery bucket"
			}
		}
	}
	bucket("hourly", spec.KeepHourly)
	bucket("daily", spec.KeepDaily)
	bucket("weekly", spec.KeepWeekly)
	bucket("monthly", spec.KeepMonthly)
	out := RetentionPreview{Plan: ref, Policy: ps.Retention, Items: []RetentionItem{}}
	for _, r := range points {
		reason := keep[r.RunID]
		if len(good) < 2 {
			reason = "At least two complete recovery points must remain"
		}
		for _, c := range r.Copies {
			if c.Engine == "pgbackrest" {
				reason = "Physical base/WAL expiration requires a dedicated dependency-aware maintenance procedure"
			}
		}
		out.Items = append(out.Items, RetentionItem{PointID: r.RunID, CapturedAt: r.FinishedAt, Eligible: reason == "", Reason: reason})
	}
	out.Digest = retentionDigest(out)
	return out, nil
}
func (e *Engine) retention(ctx context.Context, p Policy, cmd Command, point Result) (RetentionPreview, error) {
	if e.History == nil {
		return RetentionPreview{}, errors.New("independent recovery journal required")
	}
	history, err := e.History()
	if err != nil {
		return RetentionPreview{}, err
	}
	preview, err := PlanRetention(p, cmd.Resource, history, e.Now())
	if err != nil {
		return preview, err
	}
	if cmd.Operation == "retention_preview" {
		return preview, nil
	}
	if cmd.Operation != "forget" || !cmd.Approved || !e.Config.MaintenanceEnabled || cmd.PreviewDigest != preview.Digest {
		return preview, errors.New("maintenance authority or reviewed retention inventory changed")
	}
	eligible := false
	for _, item := range preview.Items {
		if item.PointID == point.RunID {
			eligible = item.Eligible
		}
	}
	if !eligible || slices.Contains(p.Holds, point.RunID) {
		return preview, errors.New("recovery point is retained or held")
	}
	copy, err := selectedCopy(point, cmd.CopyID)
	if err != nil {
		return preview, err
	}
	if copy.Engine != "restic" {
		return preview, errors.New("physical chain deletion is not a snapshot forget")
	}
	resource, ok := Find(p, copy.Destination)
	if !ok {
		return preview, errors.New("destination revision missing")
	}
	dest, err := e.destination(resource)
	if err != nil {
		return preview, err
	}
	if e.Config.Environment != "local" && (dest.MaintenanceEnvironmentFile == "" || dest.MaintenanceEnvironmentFile == dest.EnvironmentFile) {
		return preview, errors.New("independent maintenance credential required")
	}
	if dest.MaintenanceEnvironmentFile != "" {
		dest.EnvironmentFile = dest.MaintenanceEnvironmentFile
	}
	// A journal alone cannot prove that newer remote copies still exist. Re-read the
	// repository and require two newer retained complete points in this destination.
	snapshots, err := e.snapshots(ctx, dest)
	if err != nil {
		return preview, err
	}
	present := map[string]snapshotInfo{}
	for _, s := range snapshots {
		present[s.ID] = s
	}
	selected, ok := present[copy.Snapshot]
	if !ok || !slices.Contains(selected.Tags, point.RunID) || !slices.Contains(selected.Tags, copy.Source.Key()) {
		return preview, errors.New("exact snapshot correlation is missing")
	}
	retained := map[string]bool{}
	for _, item := range preview.Items {
		if !item.Eligible && item.PointID != point.RunID {
			retained[item.PointID] = true
		}
	}
	available := map[string]bool{}
	for _, entry := range history {
		r := entry.Result
		if !retained[r.RunID] || r.State != "succeeded" || !RequiredComplete(p, r) {
			continue
		}
		for _, c := range r.Copies {
			if c.Source == copy.Source && c.Destination == copy.Destination && c.State == "stored" {
				if s, ok := present[c.Snapshot]; ok && slices.Contains(s.Tags, r.RunID) {
					available[r.RunID] = true
				}
			}
		}
	}
	if len(available) < 2 {
		return preview, errors.New("fewer than two retained remote points remain")
	}
	if err = e.check(ctx, dest, "read_data"); err != nil {
		return preview, err
	}
	process, err := resticProcess(dest, "forget", copy.Snapshot)
	if err != nil {
		return preview, err
	}
	if _, err = e.Runner.Run(ctx, process); err != nil {
		return preview, err
	}
	snapshots, err = e.snapshots(ctx, dest)
	if err != nil {
		return preview, err
	}
	for _, s := range snapshots {
		if s.ID == copy.Snapshot {
			return preview, errors.New("snapshot removal was not confirmed")
		}
	}
	// Pruning shared content is deliberately separate. Never delete individual pack
	// files or apply cloud object-expiry rules to a live encrypted repository.
	return preview, nil
}
