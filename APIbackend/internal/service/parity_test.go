package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func must(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
func reject(t *testing.T, e error) {
	t.Helper()
	if e == nil {
		t.Fatal("operation unexpectedly accepted")
	}
}
func TestParityExactPrivateDiscoveryAndLimits(t *testing.T) {
	f := setup(t)
	_, e := f.s.FindRecipient(f.ctx, f.a, "tunde")
	reject(t, e)
	must(t, f.s.ChangeControls(f.ctx, f.b, ControlsInput{Handle: "tunde", Discoverable: true, PerPayment: 10000000, Daily: 50000000, Password: "test-password-123"}))
	found, e := f.s.FindRecipient(f.ctx, f.a, "@tunde")
	must(t, e)
	if found["id"] != f.b.User.ID {
		t.Fatal(found)
	}
	_, e = f.s.FindRecipient(f.ctx, f.a, "tun")
	reject(t, e)
	must(t, f.s.ChangeControls(f.ctx, f.a, ControlsInput{Handle: "amina", PerPayment: 1000, Daily: 2000, Password: "test-password-123"}))
	q, token := f.quote(t, "internal", 1100)
	_, e = f.s.CreatePayment(f.ctx, f.a, q.ID, token, "over-personal-limit")
	reject(t, e)
	if f.balance(t, f.a.User.ID).Book != 10000000 {
		t.Fatal("denied personal limit mutated funds")
	}
	c, e := f.s.CustomerCapabilities(f.ctx, f.a)
	must(t, e)
	if c["features"].(map[string]string)["credit"] != "not_offered" {
		t.Fatal(c)
	}
}
func TestParityFreezeCannotLiftComplianceOrSpend(t *testing.T) {
	f := setup(t)
	q, token := f.quote(t, "internal", 1000)
	must(t, f.s.SecureAccount(f.ctx, f.a, "test-password-123", "", true))
	_, e := f.s.CreatePayment(f.ctx, f.a, q.ID, token, "frozen-original-payment")
	reject(t, e)
	_, e = f.s.DB.Exec(`UPDATE users SET status='restricted' WHERE id=$1`, f.a.User.ID)
	must(t, e)
	e = f.s.SecureAccount(f.ctx, f.a, "test-password-123", "", false)
	if e != nil {
		status, _, _ := FaultStatus(e)
		if status != 403 {
			t.Fatal(e)
		}
	}
	u, e := f.s.Me(f.ctx, f.a.User.ID)
	must(t, e)
	if u.Status != "restricted" {
		t.Fatal("customer removed compliance restriction")
	}
}
func TestParityOneOffBankAndOwnedOriginalLookup(t *testing.T) {
	f := setup(t)
	enq, e := f.s.Enquire(f.ctx, f.a, Destination{BankCode: "999", AccountNumber: "1234567890"}, 0, "bank")
	must(t, e)
	in := QuoteInput{Kind: "bank", Amount: 1000, Currency: "NGN", Narration: "One-off test", EnquiryID: enq["id"].(string)}
	_, e = f.s.CreateQuote(f.ctx, f.b, in)
	reject(t, e)
	q, e := f.s.CreateQuote(f.ctx, f.a, in)
	must(t, e)
	auth, e := f.s.Authorise(f.ctx, f.a, q.ID, "123456", "")
	must(t, e)
	payment, e := f.s.CreatePayment(f.ctx, f.a, q.ID, auth["authorisation_token"].(string), "one-off-original-001")
	must(t, e)
	got, e := f.s.LookupPayment(f.ctx, f.a, "one-off-original-001", q.ID)
	must(t, e)
	if got["found"] != true || got["payment"].(Payment).ID != payment.ID {
		t.Fatal(got)
	}
	hidden, e := f.s.LookupPayment(f.ctx, f.b, "one-off-original-001", q.ID)
	must(t, e)
	if hidden["found"] != false {
		t.Fatal("another owner found payment")
	}
	timeline, e := f.s.PaymentTimeline(f.ctx, f.a, payment.ID)
	must(t, e)
	if timeline["hold_state"] != "active" {
		t.Fatal(timeline)
	}
	bs, e := f.s.Beneficiaries(f.ctx, f.a.User.ID)
	must(t, e)
	if len(bs) != 0 {
		t.Fatal("one-off created beneficiary")
	}
}
func TestParitySavedBillOwnerEncryptionAndReminderCancellation(t *testing.T) {
	f := setup(t)
	in := SavedBillInput{Label: "Home", ProductID: "test-electricity", CustomerID: "PRIVATE-METER-001", Amount: 1000, Favourite: true}
	id, e := f.s.SaveBill(f.ctx, f.a, "", in)
	must(t, e)
	_, e = f.s.SaveBill(f.ctx, f.b, id, in)
	reject(t, e)
	all, e := f.s.SavedBills(f.ctx, f.a)
	must(t, e)
	if len(all) != 1 || all[0].CustomerID != in.CustomerID {
		t.Fatal(all)
	}
	var raw string
	must(t, f.s.DB.QueryRow(`SELECT customer_enc FROM bill_favourites WHERE id=$1`, id).Scan(&raw))
	if strings.Contains(raw, in.CustomerID) {
		t.Fatal("plaintext meter persisted")
	}
	_, e = f.s.CreateReminder(f.ctx, f.a, ReminderInput{Title: "Home power", Amount: 1000, BillID: id, DueAt: f.clock.Add(time.Hour), Cadence: "monthly"})
	must(t, e)
	must(t, f.s.DeleteSavedBill(f.ctx, f.a, id))
	rs, e := f.s.Reminders(f.ctx, f.a)
	must(t, e)
	if len(rs) != 1 || rs[0].Status != "cancelled" {
		t.Fatal(rs)
	}
}
func TestParityCalendarAnchorsAndLagosMonthBoundary(t *testing.T) {
	jan := time.Date(2028, 1, 31, 8, 0, 0, 0, lagosZone)
	feb := NextDue(jan, "monthly", 31)
	mar := NextDue(feb, "monthly", 31)
	if feb.In(lagosZone).Day() != 29 || mar.In(lagosZone).Day() != 31 || mar.In(lagosZone).Hour() != 8 {
		t.Fatal(feb, mar)
	}
	a, b, e := MonthRange("2026-09")
	must(t, e)
	if a.Format(time.RFC3339) != "2026-08-31T23:00:00Z" || b.Format(time.RFC3339) != "2026-09-30T23:00:00Z" {
		t.Fatal(a, b)
	}
	_, _, e = MonthRange("2026-13")
	reject(t, e)
}
func TestParityReminderConcurrentWorkersHaveNoFinancialEffect(t *testing.T) {
	f := setup(t)
	due := f.clock.Add(time.Minute)
	id, e := f.s.CreateReminder(f.ctx, f.a, ReminderInput{Title: "Rent review", Amount: 1000, DueAt: due, Cadence: "once"})
	must(t, e)
	f.clock = due.Add(time.Second)
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := f.s.WorkReminder(f.ctx); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		must(t, e)
	}
	var n int
	must(t, f.s.DB.QueryRow(`SELECT count(*) FROM reminder_occurrences WHERE reminder_id=$1`, id).Scan(&n))
	if n != 1 {
		t.Fatal(n)
	}
	if f.balance(t, f.a.User.ID).Book != 10000000 {
		t.Fatal("reminder moved money")
	}
}
func newRequest(t *testing.T, f *fixture) string {
	t.Helper()
	id, e := f.s.RequestMoney(f.ctx, f.b, MoneyRequestInput{Memo: "Shared bill", Amount: 1000, ExpiresAt: f.clock.Add(24 * time.Hour), Shares: []RequestShareInput{{PayerID: f.a.User.ID, Amount: 1000}}}, "request-idempotency-001")
	must(t, e)
	return id
}
func requestPay(t *testing.T, f *fixture, rid string, amount Money, key string) Payment {
	t.Helper()
	q, e := f.s.RequestQuote(f.ctx, f.a, rid, amount)
	must(t, e)
	auth, e := f.s.Authorise(f.ctx, f.a, q.ID, "123456", "")
	must(t, e)
	p, e := f.s.CreatePayment(f.ctx, f.a, q.ID, auth["authorisation_token"].(string), key)
	must(t, e)
	return p
}
func TestParityRequestSharesPartialAndFullAtomicPayment(t *testing.T) {
	f := setup(t)
	id := newRequest(t, f)
	requestPay(t, f, id, 400, "request-partial-key")
	v, e := f.s.MoneyRequest(f.ctx, f.a, id)
	must(t, e)
	share := v["shares"].([]map[string]any)[0]
	if share["received_minor"] != Money(400) || share["remaining_minor"] != Money(600) {
		t.Fatal(v)
	}
	_, e = f.s.RequestQuote(f.ctx, f.a, id, 601)
	reject(t, e)
	requestPay(t, f, id, 600, "request-final-key-001")
	v, e = f.s.MoneyRequest(f.ctx, f.a, id)
	must(t, e)
	if v["status"] != "completed" {
		t.Fatal(v)
	}
	if f.balance(t, f.a.User.ID).Book != 9999000 || f.balance(t, f.b.User.ID).Book != 10001000 {
		t.Fatal("request and ledger do not agree")
	}
}
func TestParityCancelledRequestRejectsAlreadyAuthorisedQuote(t *testing.T) {
	f := setup(t)
	id := newRequest(t, f)
	q, e := f.s.RequestQuote(f.ctx, f.a, id, 1000)
	must(t, e)
	a, e := f.s.Authorise(f.ctx, f.a, q.ID, "123456", "")
	must(t, e)
	must(t, f.s.MoneyRequestState(f.ctx, f.b, id, "cancel"))
	_, e = f.s.CreatePayment(f.ctx, f.a, q.ID, a["authorisation_token"].(string), "cancelled-request-key")
	reject(t, e)
	if f.balance(t, f.a.User.ID).Book != 10000000 {
		t.Fatal("cancelled request debited")
	}
	var used bool
	must(t, f.s.DB.QueryRow(`SELECT used FROM quotes WHERE id=$1`, q.ID).Scan(&used))
	if used {
		t.Fatal("rollback lost quote")
	}
}
func TestParityRequestIdempotencyAndNoOtherShareDisclosure(t *testing.T) {
	f := setup(t)
	id := newRequest(t, f)
	if again := newRequest(t, f); again != id {
		t.Fatal("request duplicated")
	}
	_, e := f.s.RequestMoney(f.ctx, f.b, MoneyRequestInput{Memo: "Changed", Amount: 1000, ExpiresAt: f.clock.Add(24 * time.Hour), Shares: []RequestShareInput{{PayerID: f.a.User.ID, Amount: 1000}}}, "request-idempotency-001")
	reject(t, e)
	stranger := f.staff(t, "support")
	_, e = f.s.MoneyRequest(f.ctx, stranger, id)
	reject(t, e)
	must(t, f.s.MoneyRequestState(f.ctx, f.a, id, "decline"))
	_, e = f.s.RequestQuote(f.ctx, f.a, id, 1000)
	reject(t, e)
}
func newMandate(t *testing.T, f *fixture, amount Money, cadence string, max int) string {
	t.Helper()
	f.s.Config.SchedulesEnabled = true
	q, e := f.s.CreateQuote(f.ctx, f.a, QuoteInput{Kind: "internal", Amount: amount, Currency: "NGN", RecipientID: f.b.User.ID, Narration: "Household schedule"})
	must(t, e)
	id, e := f.s.CreateMandate(f.ctx, f.a, MandateInput{QuoteID: q.ID, Title: "Household schedule", Cadence: cadence, FirstAt: f.clock.Add(time.Minute), EndsAt: f.clock.Add(90 * 24 * time.Hour), MaxDebit: amount, MaxTotal: amount * Money(max), MaxOccurrences: max, PIN: "123456", Consent: true})
	must(t, e)
	return id
}
func TestParityMandateConcurrentWorkersAndImmutableTerms(t *testing.T) {
	f := setup(t)
	id := newMandate(t, f, 1000, "once", 1)
	f.clock = f.clock.Add(2 * time.Minute)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := f.s.WorkMandate(f.ctx); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		must(t, e)
	}
	h, e := f.s.MandateHistory(f.ctx, f.a, id)
	must(t, e)
	if len(h) != 1 || h[0]["payment_id"] == "" {
		t.Fatal(h)
	}
	if f.balance(t, f.a.User.ID).Book != 9999000 {
		t.Fatal("duplicate scheduled debit")
	}
	_, e = f.s.DB.Exec(`UPDATE payment_mandates SET max_debit=max_debit+1 WHERE id=$1`, id)
	reject(t, e)
	_, e = f.s.DB.Exec(`UPDATE payment_mandates SET status='active' WHERE id=$1`, id)
	reject(t, e)
	var raw string
	must(t, f.s.DB.QueryRow(`SELECT row_to_json(m)::text FROM payment_mandates m WHERE id=$1`, id).Scan(&raw))
	if strings.Contains(raw, "123456") {
		t.Fatal("PIN stored in mandate")
	}
}
func TestParityMandateInsufficientFundsAndLateExecutionPause(t *testing.T) {
	for _, mode := range []string{"balance", "late", "freeze"} {
		t.Run(mode, func(t *testing.T) {
			f := setup(t)
			id := newMandate(t, f, 6000000, "monthly", 2)
			if mode == "balance" {
				f.payment(t, "internal", 8000000)
			}
			if mode == "freeze" {
				must(t, f.s.SecureAccount(f.ctx, f.a, "test-password-123", "", true))
			}
			f.clock = f.clock.Add(2 * time.Minute)
			if mode == "late" {
				f.clock = f.clock.Add(48 * time.Hour)
			}
			before := f.balance(t, f.a.User.ID)
			_, e := f.s.WorkMandate(f.ctx)
			must(t, e)
			after := f.balance(t, f.a.User.ID)
			if before != after {
				t.Fatal("paused occurrence moved money")
			}
			var state string
			must(t, f.s.DB.QueryRow(`SELECT status FROM payment_mandates WHERE id=$1`, id).Scan(&state))
			if state != "paused" {
				t.Fatal(state)
			}
			h, e := f.s.MandateHistory(f.ctx, f.a, id)
			must(t, e)
			if len(h) != 1 || h[0]["status"] != "action_required" {
				t.Fatal(h)
			}
		})
	}
}
func TestParityScheduleConsentAndExternalGate(t *testing.T) {
	f := setup(t)
	f.s.Config.SchedulesEnabled = true
	q, _ := f.quote(t, "bank", 1000)
	_, e := f.s.CreateMandate(f.ctx, f.a, MandateInput{QuoteID: q.ID, Title: "Test", Cadence: "once", FirstAt: f.clock.Add(time.Hour), EndsAt: f.clock.Add(24 * time.Hour), MaxDebit: 1100, MaxTotal: 1100, MaxOccurrences: 1, PIN: "123456", Consent: true})
	reject(t, e)
	f.s.Config.SchedulesEnabled = false
	_, e = f.s.CreateMandate(f.ctx, f.a, MandateInput{})
	reject(t, e)
}
func TestParityInsightsBudgetExclusionAndPostedMonth(t *testing.T) {
	f := setup(t)
	actual := f.clock
	f.clock = actual.AddDate(0, -1, 0)
	p := f.payment(t, "internal", 1000)
	f.clock = actual
	month := actual.In(lagosZone).Format("2006-01")
	must(t, f.s.SetBudget(f.ctx, f.a, BudgetInput{Month: month, Category: "housing", Amount: 10000}))
	must(t, f.s.Annotate(f.ctx, f.a, p.ID, AnnotationInput{Category: "housing", Note: "Private reference", Excluded: false}))
	v, e := f.s.Insights(f.ctx, f.a, month)
	must(t, e)
	if v["spending_minor"] != "1000" {
		t.Fatal("must group by posting month, not quote month", v)
	}
	must(t, f.s.Annotate(f.ctx, f.a, p.ID, AnnotationInput{Category: "housing", Note: "Not spending", Excluded: true}))
	v, e = f.s.Insights(f.ctx, f.a, month)
	must(t, e)
	if v["spending_minor"] != "0" || v["excluded_minor"] != "1000" {
		t.Fatal(v)
	}
	other, e := f.s.Annotation(f.ctx, f.b, p.ID)
	must(t, e)
	if other.Note != "" {
		t.Fatal("recipient read sender private note")
	}
}

type memoryObjects struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (m *memoryObjects) Put(_ context.Context, k string, b []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = map[string][]byte{}
	}
	m.data[k] = append([]byte(nil), b...)
	return nil
}
func (m *memoryObjects) Get(_ context.Context, k string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[k]
	if !ok {
		return nil, errors.New("absent")
	}
	return append([]byte(nil), v...), nil
}
func (m *memoryObjects) Delete(_ context.Context, k string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, k)
	return nil
}

type scanState string

func (s scanState) Scan(context.Context, []byte) (string, error) { return string(s), nil }
func samplePNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	must(t, png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 10, 10))))
	return b.Bytes()
}
func enableUpload(f *fixture) *memoryObjects {
	store := &memoryObjects{}
	f.s.Config.UploadStore = store
	f.s.Config.UploadScanner = scanState("clean")
	return store
}
func TestParityCheckedEncryptedDocumentsAndOwnerIsolation(t *testing.T) {
	f := setup(t)
	store := enableUpload(f)
	raw := samplePNG(t)
	v, e := f.s.UploadDocument(f.ctx, f.a, "kyc", "", "test.png", raw)
	must(t, e)
	if bytes.Contains(store.data[v.ID], raw) {
		t.Fatal("plaintext object")
	}
	_, actual, e := f.s.DownloadDocument(f.ctx, f.a, v.ID)
	must(t, e)
	if !bytes.Equal(actual, raw) {
		t.Fatal("download mismatch")
	}
	_, _, e = f.s.DownloadDocument(f.ctx, f.b, v.ID)
	reject(t, e)
	reject(t, f.s.DeleteUpload(f.ctx, f.b, v.ID))
	_, e = f.s.UploadDocument(f.ctx, f.a, "kyc", "", "test.png", []byte("<html>invalid</html>"))
	reject(t, e)
	f.s.Config.UploadScanner = scanState("rejected")
	_, e = f.s.UploadDocument(f.ctx, f.a, "kyc", "", "malware.png", raw)
	reject(t, e)
	if len(store.data) != 1 {
		t.Fatal("rejected upload persisted")
	}
	must(t, f.s.DeleteUpload(f.ctx, f.a, v.ID))
	if len(store.data) != 0 {
		t.Fatal("unsubmitted document not removed")
	}
}
func TestParityDigitalIdentityIsOwnedVersionedAndIndependent(t *testing.T) {
	f := setup(t)
	enableUpload(f)
	v, e := f.s.UploadDocument(f.ctx, f.a, "kyc", "", "identity.png", samplePNG(t))
	must(t, e)
	in := IdentityInput{LegalName: "Synthetic Legal Name", DateOfBirth: "1990-01-02", Address: "Synthetic test address", DocumentType: "national-id", Uploads: []string{v.ID}, Consent: true}
	draft, e := f.s.SaveIdentityDraft(f.ctx, f.a, in)
	must(t, e)
	version := draft["version"].(int64)
	_, e = f.s.SaveIdentityDraft(f.ctx, f.a, in)
	reject(t, e)
	bad := in
	bad.Uploads = []string{v.ID}
	_, e = f.s.SaveIdentityDraft(f.ctx, f.b, bad)
	must(t, e)
	_, e = f.s.SubmitIdentity(f.ctx, f.b, 1)
	reject(t, e)
	cid, e := f.s.SubmitIdentity(f.ctx, f.a, version)
	must(t, e)
	reject(t, f.s.DeleteUpload(f.ctx, f.a, v.ID))
	_, e = f.s.UpdateName(f.ctx, f.a, "Editable Display Name")
	must(t, e)
	maker := f.staff(t, "admin")
	checker := f.staff(t, "compliance")
	prop, e := f.s.Propose(f.ctx, maker, ProposalInput{Action: "kyc_approve", Target: cid, Tier: 2, Reason: "Checked synthetic identity"})
	must(t, e)
	must(t, f.s.Decide(f.ctx, checker, prop, true))
	var legal string
	must(t, f.s.DB.QueryRow(`SELECT legal_name_enc FROM verified_identities WHERE owner_id=$1`, f.a.User.ID).Scan(&legal))
	decoded, e := f.s.Config.Box.Open(legal, "verified-identity:"+f.a.User.ID)
	must(t, e)
	if decoded != "Synthetic Legal Name" {
		t.Fatal("display name changed legal identity", decoded)
	}
	_, e = f.s.IdentityCase(f.ctx, f.staff(t, "support"), cid)
	reject(t, e)
}
func TestParitySupportAttachmentCannotBeDeletedOrReadByOtherOwner(t *testing.T) {
	f := setup(t)
	enableUpload(f)
	cid, e := f.s.CaseCreate(f.ctx, f.a, CaseInput{Kind: "complaint", Subject: "Missing token", Message: "Synthetic complaint", IssueType: "missing-token"})
	must(t, e)
	upload, e := f.s.UploadDocument(f.ctx, f.a, "support", cid, "evidence.png", samplePNG(t))
	must(t, e)
	reject(t, f.s.DeleteUpload(f.ctx, f.a, upload.ID))
	_, e = f.s.UploadDocument(f.ctx, f.b, "support", cid, "evidence.png", samplePNG(t))
	reject(t, e)
	must(t, f.s.EscalateCase(f.ctx, f.a, cid))
	must(t, f.s.EscalateCase(f.ctx, f.a, cid))
	events, e := f.s.CaseTimeline(f.ctx, f.a, cid)
	must(t, e)
	if len(events) != 2 {
		t.Fatal(events)
	}
}
func TestParityDualMailboxChangeRevokesAllSessionsAndPurgesCodes(t *testing.T) {
	f := setup(t)
	newEmail := "new-address@example.invalid"
	change, e := f.s.BeginEmailChange(f.ctx, f.a, ContactChangeInput{NewEmail: newEmail, Password: "test-password-123"})
	must(t, e)
	old, e := f.s.LocalChallenge(f.ctx, f.a.User.Email)
	must(t, e)
	next, e := f.s.LocalChallenge(f.ctx, newEmail)
	must(t, e)
	reject(t, f.s.FinishEmailChange(f.ctx, f.b, change["id"].(string), old["code"].(string), next["code"].(string)))
	must(t, f.s.FinishEmailChange(f.ctx, f.a, change["id"].(string), old["code"].(string), next["code"].(string)))
	var active int
	must(t, f.s.DB.QueryRow(`SELECT count(*) FROM sessions WHERE user_id=$1 AND NOT revoked`, f.a.User.ID).Scan(&active))
	if active != 0 {
		t.Fatal("sessions survived contact change")
	}
	u, e := f.s.Me(f.ctx, f.a.User.ID)
	must(t, e)
	if u.Email != newEmail {
		t.Fatal(u)
	}
	reject(t, f.s.FinishEmailChange(f.ctx, f.a, change["id"].(string), old["code"].(string), next["code"].(string)))
}
func TestParityClosureBlockersAndSettledAccountClosure(t *testing.T) {
	f := setup(t)
	reject(t, f.s.CloseAccount(f.ctx, f.a, "test-password-123", "", "Synthetic closure"))
	f.payment(t, "internal", 10000000)
	v, e := f.s.ClosureEligibility(f.ctx, f.a)
	must(t, e)
	if v["can_close"] != true {
		t.Fatal(v)
	}
	must(t, f.s.CloseAccount(f.ctx, f.a, "test-password-123", "", "Synthetic closure"))
	u, e := f.s.Me(f.ctx, f.a.User.ID)
	must(t, e)
	if u.Status != "closed" {
		t.Fatal(u)
	}
	var journals int
	must(t, f.s.DB.QueryRow(`SELECT count(*) FROM journals`).Scan(&journals))
	if journals != 3 {
		t.Fatal("closure erased accounting")
	}
}
func TestParityPrivacyExportNeverIncludesCredentialMaterial(t *testing.T) {
	f := setup(t)
	data, e := f.s.PrivacyExport(f.ctx, f.a, "test-password-123", "")
	must(t, e)
	raw, _ := json.Marshal(data)
	for _, needle := range []string{"password_hash", "mfa_secret", "pin_hash", "access_token", "refresh_token", "test-password-123"} {
		if strings.Contains(string(raw), needle) {
			t.Fatal("export leaked", needle)
		}
	}
}

type fundingTest struct {
	calls, queries int
	lost           bool
	credit         FundingEvidence
}

func (f *fundingTest) Name() string { return "test-partner" }
func (f *fundingTest) Provision(_ context.Context, key string, u User) (FundingAccountResult, error) {
	f.calls++
	if f.lost {
		return FundingAccountResult{}, errors.New("accepted but response lost")
	}
	return f.Account(context.Background(), key)
}
func (f *fundingTest) Account(context.Context, string) (FundingAccountResult, error) {
	f.queries++
	return FundingAccountResult{Reference: "test-account-reference", State: "active", AccountName: "Synthetic", AccountNumber: "TEST-NONBANK", Institution: "Synthetic only", Synthetic: true}, nil
}
func (f *fundingTest) VerifyCredit(context.Context, string) (FundingEvidence, error) {
	return f.credit, nil
}
func TestParityFundingProvisioningAndIndependentCreditIdempotence(t *testing.T) {
	f := setup(t)
	partner := &fundingTest{lost: true}
	f.s.Config.FundingGateway = partner
	_, e := f.s.RequestFundingAccount(f.ctx, f.a)
	must(t, e)
	f.clock = f.clock.Add(time.Second)
	_, e = f.s.WorkFundingAccount(f.ctx)
	reject(t, e)
	f.clock = f.clock.Add(2 * time.Minute)
	_, e = f.s.WorkFundingAccount(f.ctx)
	must(t, e)
	if partner.calls != 1 || partner.queries != 1 {
		t.Fatal("ambiguous provisioning repeated", partner)
	}
	partner.credit = FundingEvidence{Reference: "verified-credit-001", AccountReference: "test-account-reference", Amount: 500, Currency: "NGN", Final: true}
	first, e := f.s.VerifyIncomingFunding(f.ctx, "verified-credit-001")
	must(t, e)
	second, e := f.s.VerifyIncomingFunding(f.ctx, "verified-credit-001")
	must(t, e)
	if first != second || f.balance(t, f.a.User.ID).Book != 10000500 {
		t.Fatal("credit duplicated")
	}
	partner.credit.Amount = 600
	_, e = f.s.VerifyIncomingFunding(f.ctx, "verified-credit-001")
	reject(t, e)
	partner.credit.Final = false
	_, e = f.s.VerifyIncomingFunding(f.ctx, "verified-credit-001")
	reject(t, e)
}
func TestParityPasskeyCeremonyOriginBindingAndOneTimeFailure(t *testing.T) {
	f := setup(t)
	var e error
	f.s.Config.Passkeys, e = webauthn.New(&webauthn.Config{RPDisplayName: "Synthetic Qpay", RPID: "localhost", RPOrigins: []string{"http://localhost:5173"}, AuthenticatorSelection: protocol.AuthenticatorSelection{UserVerification: protocol.VerificationRequired}})
	must(t, e)
	binding := security.Random("", 32)
	begin, e := f.s.PasskeyBegin(f.ctx, f.a, "http://localhost:5173", binding, "test-password-123", "", true)
	must(t, e)
	in := PasskeyFinishInput{ID: begin["id"].(string), Label: "Synthetic device", Credential: json.RawMessage(`{"id":"invalid","rawId":"invalid","type":"public-key","response":{}}`)}
	_, e = f.s.PasskeyFinish(f.ctx, f.a, security.Random("", 32), in, true)
	reject(t, e)
	var used bool
	must(t, f.s.DB.QueryRow(`SELECT consumed FROM webauthn_ceremonies WHERE id=$1`, in.ID).Scan(&used))
	if used {
		t.Fatal("wrong browser consumed ceremony")
	}
	_, e = f.s.PasskeyFinish(f.ctx, f.a, binding, in, true)
	reject(t, e)
	must(t, f.s.DB.QueryRow(`SELECT consumed FROM webauthn_ceremonies WHERE id=$1`, in.ID).Scan(&used))
	if !used {
		t.Fatal("invalid signed response remained reusable")
	}
	var n int
	must(t, f.s.DB.QueryRow(`SELECT count(*) FROM webauthn_credentials`).Scan(&n))
	if n != 0 {
		t.Fatal("invalid passkey stored")
	}
}
func TestParityMigrationTwoDriftRejected(t *testing.T) {
	db, _ := database(t)
	_, e := db.Exec(`UPDATE schema_migrations SET checksum='incorrect' WHERE version=2`)
	must(t, e)
	reject(t, Migrate(context.Background(), db))
}

// Keep sql linked for future transaction-level regression additions without generic test mocks.
var _ *sql.Tx
