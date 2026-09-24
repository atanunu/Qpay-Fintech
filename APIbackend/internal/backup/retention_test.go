package backup

import (
	"context"
	"testing"
	"time"
)

func TestRetentionBucketsHoldsAndUncertainPoints(t *testing.T) {
	p := testPolicy()
	now := time.Now()
	history := []JournalRun{}
	for i := 0; i < 5; i++ {
		r := Result{RunID: "br_retention_" + string(rune('a'+i)), Plan: Ref{"plan_test", 1}, Operation: "backup", State: "succeeded", FinishedAt: now.Add(-time.Duration(i+2) * 24 * time.Hour), Copies: []Copy{{Source: Ref{"source_test", 1}, Destination: Ref{"destination_test", 1}, State: "stored", Engine: "restic", Verified: "read_data"}}}
		history = append(history, JournalRun{Complete: true, Result: r})
	}
	p.Holds = []string{"br_retention_d"}
	history[4].Result.State = "unknown"
	out, err := PlanRetention(p, Ref{"plan_test", 1}, history, now)
	if err != nil {
		t.Fatal(err)
	}
	for i, item := range out.Items {
		if item.Eligible != (i == 2) {
			t.Fatalf("unexpected eligibility %+v", out)
		}
	}
	if len(out.Digest) != 64 {
		t.Fatal(out)
	}
	again, err := PlanRetention(p, Ref{"plan_test", 1}, history, now)
	if err != nil || out.Digest != again.Digest {
		t.Fatal("unstable preview")
	}
	history = append(history, history[0])
	if _, err = PlanRetention(p, Ref{"plan_test", 1}, history, now); err == nil {
		t.Fatal("duplicate inventory accepted")
	}
}
func TestRealRetentionRechecksCopiesAndForgetsOnlyApprovedSnapshot(t *testing.T) {
	e, p := engineFixture(t)
	ctx := context.Background()
	now := time.Now()
	history := []JournalRun{}
	for i := 0; i < 3; i++ {
		cmd := Command{ID: "command_retention_" + string(rune('a'+i)), Operation: "backup", Resource: Ref{"plan_test", 1}, Approved: true, ExpiresAt: now.Add(time.Hour)}
		p.Commands = append(p.Commands, cmd)
		r := e.Execute(ctx, p, NewCommandResult(p, Digest(Canonical(p)), cmd, now), &cmd, nil)
		if r.State != "succeeded" {
			t.Fatal(r)
		}
		r.FinishedAt = now.Add(-time.Duration(5-i) * 24 * time.Hour)
		history = append(history, JournalRun{Complete: true, Result: r})
	}
	e.History = func() ([]JournalRun, error) { return history, nil }
	e.Now = func() time.Time { return now }
	e.Config.MaintenanceEnabled = true
	preview, err := PlanRetention(p, Ref{"plan_test", 1}, history, now)
	if err != nil {
		t.Fatal(err)
	}
	point := history[0].Result
	cmd := Command{ID: "command_forget_exact", Operation: "forget", Resource: point.Plan, PointID: point.RunID, CopyID: point.Copies[0].ID, Approved: true, PreviewDigest: preview.Digest}
	stale := cmd
	stale.PreviewDigest = "changed"
	if _, err = e.retention(ctx, p, stale, point); err == nil {
		t.Fatal("changed preview accepted")
	}
	held := p
	held.Holds = []string{point.RunID}
	if _, err = e.retention(ctx, held, cmd, point); err == nil {
		t.Fatal("held copy removed")
	}
	if _, err = e.retention(ctx, p, cmd, point); err != nil {
		t.Fatal(err)
	}
	snapshots, err := e.snapshots(ctx, e.Config.Destinations["repo_test"])
	if err != nil || len(snapshots) != 2 {
		t.Fatal("did not preserve two complete remote snapshots", len(snapshots), err)
	}
	if _, err = e.retention(ctx, p, cmd, point); err == nil {
		t.Fatal("already removed point reaccepted")
	}
}
