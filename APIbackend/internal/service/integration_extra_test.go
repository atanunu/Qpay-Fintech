package service

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestNotificationExpiredLeasePersistsAsUnknown(t *testing.T) {
	f := setup(t)
	f.s.Config.NotificationMode = "novu"
	f.s.Config.NovuAllowlist = map[string]bool{}
	// No network call: all other queued work is intentionally suppressed.
	if _, e := f.s.DB.Exec(`UPDATE notification_intents SET state='suppressed'`); e != nil {
		t.Fatal(e)
	}
	var id string
	if e := f.s.DB.QueryRow(`UPDATE notification_intents SET state='leased',lease_until=$1 WHERE id=(SELECT id FROM notification_intents LIMIT 1) RETURNING id`, f.clock.Add(-time.Minute)).Scan(&id); e != nil {
		t.Fatal(e)
	}
	worked, e := f.s.WorkNotification(f.ctx)
	if e != nil || worked {
		t.Fatal(worked, e)
	}
	var state string
	if e = f.s.DB.QueryRow(`SELECT state FROM notification_intents WHERE id=$1`, id).Scan(&state); e != nil || state != "unknown" {
		t.Fatal(state, e)
	}
}
func TestRevokedOperatorCannotRequeuePayment(t *testing.T) {
	f := setup(t)
	operator := f.staff(t, "finance")
	payment := f.payment(t, "bank", 1000)
	if _, e := f.s.DB.Exec(`UPDATE payments SET status='pending' WHERE id=$1`, payment.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.s.DB.Exec(`UPDATE jobs SET status='dead' WHERE object_id=$1`, payment.ID); e != nil {
		t.Fatal(e)
	}
	if e := f.s.RevokeSession(f.ctx, operator, operator.SessionID); e != nil {
		t.Fatal(e)
	}
	if e := f.s.RetryJob(f.ctx, operator, payment.ID); e == nil {
		t.Fatal("revoked session requeued financial work")
	}
}
func TestKYCIndependentReviewAndEligibility(t *testing.T) {
	f := setup(t)
	maker := f.staff(t, "admin")
	checker := f.staff(t, "compliance")
	id, e := f.s.KYCSubmit(f.ctx, f.a, "synthetic-reviewed-private-evidence")
	if e != nil {
		t.Fatal(e)
	}
	proposal, e := f.s.Propose(f.ctx, maker, ProposalInput{Action: "kyc_approve", Target: id, Tier: 2, Reason: "Synthetic verified manual evidence"})
	if e != nil {
		t.Fatal(e)
	}
	if e = f.s.Decide(f.ctx, maker, proposal, true); e == nil {
		t.Fatal("self-approved identity")
	}
	if e = f.s.Decide(f.ctx, checker, proposal, true); e != nil {
		t.Fatal(e)
	}
	u, e := f.s.Me(f.ctx, f.a.User.ID)
	if e != nil || u.Tier != 2 {
		t.Fatal(u, e)
	}
}
func TestInsufficientFundsDoesNotConsumeAuthorisation(t *testing.T) {
	f := setup(t)
	q, token := f.quote(t, "internal", 9000000)
	// Spend most funds on a different approved operation first.
	f.payment(t, "internal", 8000000)
	if _, e := f.s.CreatePayment(f.ctx, f.a, q.ID, token, "insufficient-funds-request"); e == nil {
		t.Fatal("overspend accepted")
	}
	var used bool
	if e := f.s.DB.QueryRow(`SELECT used FROM quotes WHERE id=$1`, q.ID).Scan(&used); e != nil || used {
		t.Fatal("failed transaction consumed quote", e)
	}
}
func TestExpiredSessionCannotExecuteNewPayment(t *testing.T) {
	f := setup(t)
	q, token := f.quote(t, "internal", 1000)
	if _, e := f.s.DB.Exec(`UPDATE sessions SET expires_at=$2 WHERE id=$1`, f.a.SessionID, f.clock.Add(-time.Second)); e != nil {
		t.Fatal(e)
	}
	if _, e := f.s.CreatePayment(f.ctx, f.a, q.ID, token, "expired-session-request"); e == nil {
		t.Fatal("expired session spent funds")
	}
}
func TestExpiredSessionInsideTransaction(t *testing.T) {
	f := setup(t)
	if e := f.s.RevokeSession(f.ctx, f.a, f.a.SessionID); e != nil {
		t.Fatal(e)
	}
	e := f.s.transact(context.Background(), func(tx *sql.Tx) error { _, e := f.s.activePrincipal(context.Background(), tx, f.a); return e })
	if e == nil {
		t.Fatal("revoked session passed transactional check")
	}
}
