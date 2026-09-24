package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"strings"
	"sync"
	"testing"
	"time"
)

func requireErr(t *testing.T, e error) {
	t.Helper()
	if e == nil {
		t.Fatal("expected denied or conflict")
	}
}
func TestAdminEveryProjectionAndDetails(t *testing.T) {
	f := setup(t)
	p := f.staff(t, "admin")
	f.payment(t, "internal", 1000)
	f.payment(t, "bank", 1000)
	f.s.CaseCreate(f.ctx, f.a, CaseInput{Kind: "support", Subject: "Synthetic concern", Message: "Synthetic private message"})
	if _, e := f.s.KYCSubmit(f.ctx, f.a, "synthetic-reference"); e != nil {
		t.Fatal(e)
	}
	for name := range adminResources {
		t.Run(name, func(t *testing.T) {
			page, e := f.s.AdminList(f.ctx, p, name, AdminQuery{Limit: 50})
			if e != nil {
				t.Fatal(e)
			}
			for _, row := range page.Items {
				if _, e := f.s.AdminDetail(f.ctx, p, name, row["id"].(string)); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
	if _, e := f.s.AdminMetrics(f.ctx, p); e != nil {
		t.Fatal(e)
	}
	if _, e := f.s.AdminBootstrap(f.ctx, p); e != nil {
		t.Fatal(e)
	}
}
func TestAdminRoleMatrix(t *testing.T) {
	f := setup(t)
	for _, role := range []string{"admin", "finance", "compliance", "support", "platform", "auditor"} {
		p := f.staff(t, role)
		for name, resource := range adminResources {
			t.Run(role+"/"+name, func(t *testing.T) {
				_, e := f.s.AdminList(f.ctx, p, name, AdminQuery{Limit: 1})
				allowed := false
				for _, r := range resource.Roles {
					if r == role {
						allowed = true
					}
				}
				if allowed && e != nil {
					t.Fatal(e)
				}
				if !allowed && e == nil {
					t.Fatal("resource allowed to wrong role")
				}
			})
		}
	}
}
func TestAdminDenyCustomerRevokedExpiredUnenrolled(t *testing.T) {
	f := setup(t)
	_, e := f.s.AdminList(f.ctx, f.a, "customers", AdminQuery{Limit: 10})
	requireErr(t, e)
	p := f.staff(t, "support")
	_, e = f.s.AdminCommand(f.ctx, p, AdminCommandInput{Action: "emergency_stop", Version: 1, Reason: "unauthorised emergency attempt"})
	requireErr(t, e)
	if _, e = f.s.DB.Exec(`UPDATE users SET mfa_enabled=false WHERE id=$1`, p.User.ID); e != nil {
		t.Fatal(e)
	}
	_, e = f.s.AdminMetrics(f.ctx, p)
	requireErr(t, e)
	p = f.staff(t, "admin")
	if e = f.s.RevokeSession(f.ctx, p, p.SessionID); e != nil {
		t.Fatal(e)
	}
	_, e = f.s.AdminList(f.ctx, p, "customers", AdminQuery{Limit: 1})
	requireErr(t, e)
	p = f.staff(t, "admin")
	if _, e = f.s.DB.Exec(`UPDATE sessions SET expires_at=$2 WHERE id=$1`, p.SessionID, f.clock.Add(-time.Second)); e != nil {
		t.Fatal(e)
	}
	_, e = f.s.AdminMetrics(f.ctx, p)
	requireErr(t, e)
}
func TestAdminPaginateFilterRedact(t *testing.T) {
	f := setup(t)
	p := f.staff(t, "admin")
	first, e := f.s.AdminList(f.ctx, p, "customers", AdminQuery{Limit: 1})
	if e != nil || len(first.Items) != 1 || !first.HasMore {
		t.Fatalf("%+v %v", first, e)
	}
	second, e := f.s.AdminList(f.ctx, p, "customers", AdminQuery{Limit: 1, Before: first.NextBefore})
	if e != nil || len(second.Items) != 1 || second.Items[0]["id"] == first.Items[0]["id"] {
		t.Fatal(second, e)
	}
	for _, term := range []string{"%' OR 1=1 --", "../staff", "<script>"} {
		page, e := f.s.AdminList(f.ctx, p, "customers", AdminQuery{Search: term, Limit: 10})
		if e != nil || len(page.Items) != 0 {
			t.Fatal(page, e)
		}
	}
	_, e = f.s.AdminList(f.ctx, p, "users;DROP TABLE users", AdminQuery{Limit: 1})
	requireErr(t, e)
	raw, _ := json.Marshal(first)
	for _, bad := range []string{"password_hash", "pin_hash", "mfa_secret", "mfa_pending", "user0@example.invalid"} {
		if strings.Contains(string(raw), bad) {
			t.Fatal("secret or full email in projection:", bad)
		}
	}
}
func TestStaffInvitationIndependentSingleUseAndRedacted(t *testing.T) {
	f := setup(t)
	maker := f.staff(t, "admin")
	checker := f.staff(t, "admin")
	id, e := f.s.InviteStaff(f.ctx, maker, StaffInvitationInput{Email: "new-operator@example.invalid", Name: "Synthetic Operator", Role: "support", Reason: "Approved support staffing request"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.s.DecideInvitation(f.ctx, maker, id, "approve", "independent identity review")
	requireErr(t, e)
	result, e := f.s.DecideInvitation(f.ctx, checker, id, "approve", "identity reviewed independently")
	if e != nil {
		t.Fatal(e)
	}
	token := result["invitation_token"].(string)
	if len(token) != 43 {
		t.Fatal("invalid token")
	}
	page, e := f.s.AdminList(f.ctx, maker, "invitations", AdminQuery{Limit: 50})
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(page)
	if strings.Contains(string(raw), token) || strings.Contains(string(raw), "token_hash") {
		t.Fatal("invitation secret exposed")
	}
	uid, e := f.s.AcceptStaffInvitation(f.ctx, StaffInvitationAccept{Token: token, Password: "synthetic-invite-password"})
	if e != nil {
		t.Fatal(e)
	}
	u, e := f.s.Me(f.ctx, uid)
	if e != nil || u.MFA || u.Role != "support" || u.Verified {
		t.Fatal(u, e)
	}
	_, e = f.s.AcceptStaffInvitation(f.ctx, StaffInvitationAccept{Token: token, Password: "synthetic-replayed-password"})
	requireErr(t, e)
	_, e = f.s.DecideInvitation(f.ctx, checker, id, "revoke", "too late after consumption")
	requireErr(t, e)
}
func TestStaffInvitationRevocationExpiryAndMakerAccess(t *testing.T) {
	f := setup(t)
	maker := f.staff(t, "admin")
	checker := f.staff(t, "admin")
	id, e := f.s.InviteStaff(f.ctx, maker, StaffInvitationInput{Email: "revoked@example.invalid", Name: "Synthetic Operator", Role: "auditor", Reason: "Audit staffing approved"})
	if e != nil {
		t.Fatal(e)
	}
	result, e := f.s.DecideInvitation(f.ctx, checker, id, "approve", "Checked independent identity")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.DecideInvitation(f.ctx, maker, id, "revoke", "Cancelled before delivery"); e != nil {
		t.Fatal(e)
	}
	_, e = f.s.AcceptStaffInvitation(f.ctx, StaffInvitationAccept{Token: result["invitation_token"].(string), Password: "synthetic-invite-password"})
	requireErr(t, e)
	id, e = f.s.InviteStaff(f.ctx, maker, StaffInvitationInput{Email: "expired@example.invalid", Name: "Synthetic Operator", Role: "support", Reason: "Support staffing approved"})
	if e != nil {
		t.Fatal(e)
	}
	f.clock = f.clock.Add(49 * time.Hour)
	_, e = f.s.DecideInvitation(f.ctx, checker, id, "approve", "Checked identity after expiry")
	requireErr(t, e)
}
func adminAction(f *fixture, t *testing.T, p Principal, in AdminCommandInput) map[string]any {
	t.Helper()
	v, e := f.s.AdminCommand(f.ctx, p, in)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestStaffRoleChangeRequiresTwoOthersAndRevokesSessions(t *testing.T) {
	f := setup(t)
	maker := f.staff(t, "admin")
	checker := f.staff(t, "admin")
	target := f.staff(t, "support")
	in := AdminCommandInput{Action: "control_propose", Resource: "staff_role", Target: target.User.ID, Version: target.User.Version, Value: "auditor", Reason: "Move colleague to read only audit"}
	out := adminAction(f, t, maker, in)
	id := out["id"].(string)
	_, e := f.s.AdminCommand(f.ctx, maker, AdminCommandInput{Action: "control_decide", Target: id, Value: "approve", Reason: "self approval not allowed"})
	requireErr(t, e)
	adminAction(f, t, checker, AdminCommandInput{Action: "control_decide", Target: id, Value: "approve", Reason: "Independent role review completed"})
	u, e := f.s.Me(f.ctx, target.User.ID)
	if e != nil || u.Role != "auditor" {
		t.Fatal(u, e)
	}
	_, e = f.s.AdminList(f.ctx, target, "support", AdminQuery{Limit: 1})
	requireErr(t, e)
	_, e = f.s.AdminCommand(f.ctx, checker, AdminCommandInput{Action: "control_decide", Target: id, Value: "approve", Reason: "must not apply approval twice"})
	requireErr(t, e)
	_, e = f.s.AdminCommand(f.ctx, maker, AdminCommandInput{Action: "control_propose", Resource: "staff_role", Target: maker.User.ID, Version: 1, Value: "auditor", Reason: "self role change is forbidden"})
	requireErr(t, e)
}
func TestAdminStaleControlAndImmutableProposal(t *testing.T) {
	f := setup(t)
	a := f.staff(t, "admin")
	b := f.staff(t, "admin")
	target := f.staff(t, "support")
	x := adminAction(f, t, a, AdminCommandInput{Action: "control_propose", Resource: "staff_status", Target: target.User.ID, Value: "restricted", Version: 1, Reason: "Pending independent access review"})
	id := x["id"].(string)
	if _, e := f.s.DB.Exec(`UPDATE admin_controls SET value='active' WHERE id=$1`, id); e == nil {
		t.Fatal("immutable snapshot modified")
	}
	if _, e := f.s.DB.Exec(`UPDATE users SET version=version+1 WHERE id=$1`, target.User.ID); e != nil {
		t.Fatal(e)
	}
	_, e := f.s.AdminCommand(f.ctx, b, AdminCommandInput{Action: "control_decide", Target: id, Value: "approve", Reason: "target now stale and rejected"})
	requireErr(t, e)
	proposal, e := f.s.Propose(f.ctx, a, ProposalInput{Action: "policy", Reason: "Synthetic policy", Policy: &PolicyInput{PerPayment: 1000, Daily: 5000, Enabled: true}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.DB.Exec(`UPDATE proposals SET payload='{}' WHERE id=$1`, proposal); e == nil {
		t.Fatal("financial proposal modified")
	}
}
func TestStaffRecoveryTwoApproversAndOneUse(t *testing.T) {
	f := setup(t)
	a := f.staff(t, "admin")
	b := f.staff(t, "admin")
	target := f.staff(t, "support")
	x := adminAction(f, t, a, AdminCommandInput{Action: "control_propose", Resource: "staff_recovery", Target: target.User.ID, Version: 1, Reason: "Independent identity evidence verified"})
	id := x["id"].(string)
	out := adminAction(f, t, b, AdminCommandInput{Action: "control_decide", Target: id, Value: "approve", Reason: "Second reviewer verified ownership"})
	token := out["recovery_token"].(string)
	if e := f.s.AcceptStaffRecovery(f.ctx, StaffInvitationAccept{Token: token, Password: "recovered-synthetic-password"}); e != nil {
		t.Fatal(e)
	}
	u, e := f.s.Me(f.ctx, target.User.ID)
	if e != nil || u.MFA || u.Status != "active" || u.Role != "support" {
		t.Fatal(u, e)
	}
	_, e = f.s.AdminList(f.ctx, target, "support", AdminQuery{Limit: 1})
	requireErr(t, e)
	requireErr(t, f.s.AcceptStaffRecovery(f.ctx, StaffInvitationAccept{Token: token, Password: "second-synthetic-password"}))
	var raw string
	if e = f.s.DB.QueryRow(`SELECT details::text FROM audit_events WHERE action='staff.recovery_consumed'`).Scan(&raw); e != nil || strings.Contains(raw, token) {
		t.Fatal(raw, e)
	}
}
func TestAdminProductDisableRechecksApprovedQuote(t *testing.T) {
	f := setup(t)
	a := f.staff(t, "admin")
	b := f.staff(t, "platform")
	q, auth := f.quote(t, "bill", 1000)
	x := adminAction(f, t, a, AdminCommandInput{Action: "control_propose", Resource: "product", Target: "test-electricity", Version: 1, Value: "disabled", Reason: "Biller unavailable per reviewed evidence"})
	adminAction(f, t, b, AdminCommandInput{Action: "control_decide", Target: x["id"].(string), Value: "approve", Reason: "Independent product outage confirmed"})
	_, e := f.s.CreatePayment(f.ctx, f.a, q.ID, auth, "disabled-product-payment-key")
	requireErr(t, e)
	if f.balance(t, f.a.User.ID).Held != 0 {
		t.Fatal("disabled payment reserved funds")
	}
}
func TestAdminEmergencyStopResumeNeedsIndependentVersion(t *testing.T) {
	f := setup(t)
	a := f.staff(t, "platform")
	b := f.staff(t, "admin")
	adminAction(f, t, a, AdminCommandInput{Action: "emergency_stop", Version: 1, Reason: "Contain unexpected payment acceptance"})
	policy, e := f.s.AdminPaymentControl(f.ctx, a)
	if e != nil || policy["payments_enabled"] != false {
		t.Fatal(policy, e)
	}
	x := adminAction(f, t, a, AdminCommandInput{Action: "control_propose", Resource: "resume", Version: 2, Reason: "Incident mitigation independently verified"})
	_, e = f.s.AdminCommand(f.ctx, a, AdminCommandInput{Action: "control_decide", Target: x["id"].(string), Value: "approve", Reason: "Self approval must be rejected"})
	requireErr(t, e)
	adminAction(f, t, b, AdminCommandInput{Action: "control_decide", Target: x["id"].(string), Value: "approve", Reason: "Independent resumption review complete"})
	policy, e = f.s.AdminPaymentControl(f.ctx, b)
	if e != nil || policy["payments_enabled"] != true {
		t.Fatal(policy, e)
	}
}
func TestAdminWorkNotesAndNoFinancialEffect(t *testing.T) {
	f := setup(t)
	a := f.staff(t, "admin")
	p := f.payment(t, "internal", 1000)
	before := f.balance(t, f.a.User.ID)
	x := adminAction(f, t, a, AdminCommandInput{Action: "work_create", Resource: "refund", Target: p.ID, Title: "Investigate customer refund request", Severity: "medium", Reason: "Customer supplied private investigation details"})
	id := x["id"].(string)
	adminAction(f, t, a, AdminCommandInput{Action: "work_update", Target: id, Version: 1, Value: "investigating", Reason: "Reviewed original posting before investigation"})
	_, e := f.s.AdminCommand(f.ctx, a, AdminCommandInput{Action: "work_update", Target: id, Version: 1, Value: "resolved", Reason: "Stale update must fail"})
	requireErr(t, e)
	d, e := f.s.AdminDetail(f.ctx, a, "investigations", id)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(d)
	if !strings.Contains(string(raw), "Reviewed original posting") {
		t.Fatal("notes missing")
	}
	var plaintext bool
	if e = f.s.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM admin_notes WHERE body_enc LIKE '%Reviewed original%')`).Scan(&plaintext); e != nil || plaintext {
		t.Fatal("plaintext stored", e)
	}
	after := f.balance(t, f.a.User.ID)
	if before != after {
		t.Fatal("investigation changed funds")
	}
}
func TestAdminSupportAssignmentAndNotificationSuppression(t *testing.T) {
	f := setup(t)
	a := f.staff(t, "admin")
	support := f.staff(t, "support")
	id, e := f.s.CaseCreate(f.ctx, f.a, CaseInput{Kind: "support", Subject: "Synthetic question", Message: "Synthetic conversation"})
	if e != nil {
		t.Fatal(e)
	}
	adminAction(f, t, a, AdminCommandInput{Action: "support_assign", Target: id, Version: 1, AssignedTo: support.User.ID, Reason: "Route case to verified support colleague"})
	_, e = f.s.AdminCommand(f.ctx, a, AdminCommandInput{Action: "support_assign", Target: id, Version: 1, Reason: "Stale assignment rejected"})
	requireErr(t, e)
	var notice string
	if e = f.s.DB.QueryRow(`SELECT id FROM notification_intents WHERE NOT secret AND state='queued' LIMIT 1`).Scan(&notice); e != nil {
		t.Fatal(e)
	}
	adminAction(f, t, a, AdminCommandInput{Action: "notification_suppress", Target: notice, Reason: "Customer destination undergoing review"})
	_, e = f.s.AdminCommand(f.ctx, a, AdminCommandInput{Action: "notification_suppress", Target: notice, Reason: "Already suppressed not actionable"})
	requireErr(t, e)
}
func TestAdminCSVExportFormulaAndRoleBoundaries(t *testing.T) {
	f := setup(t)
	a := f.staff(t, "auditor")
	if _, e := f.s.DB.Exec(`UPDATE users SET name='=HYPERLINK("malicious")' WHERE id=$1`, f.a.User.ID); e != nil {
		t.Fatal(e)
	}
	text, e := f.s.AdminExport(f.ctx, a, "customers", "", "Controlled synthetic audit export")
	if e != nil || !strings.Contains(text, "'=HYPERLINK") {
		t.Fatal(text, e)
	}
	if strings.Contains(text, "password_hash") || strings.Contains(text, f.a.User.Email) {
		t.Fatal("private secrets in CSV")
	}
	support := f.staff(t, "support")
	_, e = f.s.AdminExport(f.ctx, support, "customers", "", "Forbidden export attempt")
	requireErr(t, e)
	_, e = f.s.AdminCommand(f.ctx, a, AdminCommandInput{Action: "note", Resource: "customers", Target: f.a.User.ID, Reason: "Auditor cannot mutate records"})
	requireErr(t, e)
	var n int
	if e = f.s.DB.QueryRow(`SELECT count(*) FROM admin_export_events`).Scan(&n); e != nil || n != 1 {
		t.Fatal(n, e)
	}
}
func TestReconciliationConcurrentImportHasOneRun(t *testing.T) {
	f := setup(t)
	p := f.payment(t, "internal", 1000)
	a := f.staff(t, "finance")
	in := ReconciliationInput{Source: "provider", Entries: []ReconciliationEntry{{Reference: p.ID, Amount: 1000, Currency: "NGN", Status: "succeeded"}}}
	var wg sync.WaitGroup
	ids := make(chan string, 4)
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, e := f.s.Reconcile(f.ctx, a, in)
			if e == nil {
				ids <- out["id"].(string)
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatal("duplicate report run")
		}
	}
	var n int
	if e := f.s.DB.QueryRow(`SELECT count(*) FROM reconciliation_runs`).Scan(&n); e != nil || n != 1 {
		t.Fatal(n, e)
	}
}
func TestAdminElevationExpiresAndDeniesWrites(t *testing.T) {
	f := setup(t)
	a := f.staff(t, "admin")
	if _, e := f.s.DB.Exec(`UPDATE sessions SET created_at=$2 WHERE id=$1`, a.SessionID, f.clock.Add(-11*time.Minute)); e != nil {
		t.Fatal(e)
	}
	requireErr(t, f.s.RequireStaffElevation(f.ctx, a))
	_, e := f.s.AdminCommand(f.ctx, a, AdminCommandInput{Action: "emergency_stop", Version: 1, Reason: "Fresh MFA required before mutation"})
	requireErr(t, e)
	if e = f.s.transact(context.Background(), func(tx *sql.Tx) error {
		return exec(tx, f.ctx, `INSERT INTO staff_elevations(session_id,expires_at) VALUES($1,$2)`, a.SessionID, f.clock.Add(time.Minute))
	}); e != nil {
		t.Fatal(e)
	}
	if e = f.s.RequireStaffElevation(f.ctx, a); e != nil {
		t.Fatal(e)
	}
}
func TestAdminBootstrapCheckerLimited(t *testing.T) {
	db, _ := database(t)
	key := []byte(strings.Repeat("x", 32))
	s := New(db, Config{Environment: "local", Pepper: key, Box: security.Box{Active: "v1", Keys: map[string][]byte{"v1": key}}})
	ctx := context.Background()
	if _, e := s.BootstrapChecker(ctx, "checker@example.invalid", "Checker", "synthetic-checker-password"); e == nil {
		t.Fatal("checker before administrator")
	}
	if _, e := s.BootstrapStaff(ctx, "admin@example.invalid", "Admin", "synthetic-admin-password"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.BootstrapChecker(ctx, "checker@example.invalid", "Checker", "synthetic-checker-password"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.BootstrapChecker(ctx, "other@example.invalid", "Third", "synthetic-third-password"); e == nil {
		t.Fatal("repeat bootstrap")
	}
}
