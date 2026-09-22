package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type testGateway struct {
	mu               sync.Mutex
	submits, queries int
	failSubmit       bool
	state            string
	wrongAmount      bool
	fulfilled        bool
	requests         map[string]ProviderRequest
}

func (g *testGateway) Banks(context.Context) ([]Bank, error) {
	return []Bank{{Code: "999", Name: "SYNTHETIC"}}, nil
}
func (g *testGateway) Products(context.Context) ([]Product, error) {
	return []Product{{ID: "test-electricity", Name: "Synthetic electricity", Category: "electricity", Variable: true}}, nil
}
func (g *testGateway) Enquire(context.Context, Destination) (Enquiry, error) {
	return Enquiry{Name: "TEST RECIPIENT", Reference: "enquiry_test"}, nil
}
func (g *testGateway) ValidateBill(context.Context, Destination, Money) (Enquiry, error) {
	return Enquiry{Name: "TEST BILL CUSTOMER", Reference: "validation_test"}, nil
}
func (g *testGateway) Quote(context.Context, string, Destination, Money) (Money, error) {
	return 100, nil
}
func (g *testGateway) result(p ProviderRequest) ProviderResult {
	amount := p.Amount
	if g.wrongAmount {
		amount++
	}
	return ProviderResult{Reference: p.Reference, UpstreamID: "up_" + p.Reference, Status: g.state, Amount: amount, Currency: p.Currency, Fulfilled: g.fulfilled, Fulfilment: func() string {
		if g.fulfilled {
			return "SYNTHETIC TOKEN"
		}
		return ""
	}()}
}
func (g *testGateway) Submit(_ context.Context, p ProviderRequest) (ProviderResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.submits++
	if g.requests == nil {
		g.requests = map[string]ProviderRequest{}
	}
	g.requests[p.Reference] = p
	if g.failSubmit {
		return ProviderResult{}, errors.New("simulated response lost after receipt")
	}
	return g.result(p), nil
}
func (g *testGateway) Query(_ context.Context, p ProviderRequest) (ProviderResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.queries++
	if _, ok := g.requests[p.Reference]; !ok {
		return ProviderResult{}, errors.New("original request unknown")
	}
	return g.result(p), nil
}

type fixture struct {
	s       *Service
	ctx     context.Context
	a, b    Principal
	gateway *testGateway
	dsn     string
	clock   time.Time
}

func database(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("QPF_REQUIRE_POSTGRES") == "true" {
			t.Fatal("real PostgreSQL test database is required")
		}
		t.Skip("set TEST_DATABASE_URL for real PostgreSQL integration tests")
	}
	admin, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := "test_" + security.Digest(security.Random("", 24))[:20]
	if _, e = admin.Exec(`CREATE SCHEMA ` + name); e != nil {
		t.Fatal(e)
	}
	u, e := url.Parse(dsn)
	if e != nil {
		t.Fatal(e)
	}
	q := u.Query()
	q.Set("search_path", name)
	u.RawQuery = q.Encode()
	db, e := sql.Open("pgx", u.String())
	if e != nil {
		t.Fatal(e)
	}
	db.SetMaxOpenConns(24)
	t.Cleanup(func() { db.Close(); _, _ = admin.Exec(`DROP SCHEMA ` + name + ` CASCADE`); admin.Close() })
	if e = Migrate(context.Background(), db); e != nil {
		t.Fatal(e)
	}
	return db, u.String()
}
func setup(t *testing.T) *fixture {
	t.Helper()
	db, dsn := database(t)
	f := &fixture{ctx: context.Background(), dsn: dsn, clock: time.Now().UTC(), gateway: &testGateway{state: "pending"}}
	key := bytes.Repeat([]byte{5}, 32)
	f.s = New(db, Config{Environment: "local", Pepper: key, Box: security.Box{Active: "test", Keys: map[string][]byte{"test": key}}, Gateway: f.gateway, ExternalEnabled: true, NotificationMode: "local"})
	f.s.Now = func() time.Time { return f.clock }
	for i, p := range []*Principal{&f.a, &f.b} {
		email := fmt.Sprintf("user%d@example.invalid", i)
		if _, e := f.s.SeedLocal(f.ctx, email, "Synthetic Customer", "test-password-123", "123456"); e != nil {
			t.Fatal(e)
		}
		session, e := f.s.Login(f.ctx, LoginInput{Email: email, Password: "test-password-123", Client: "mobile", Device: "test-device"}, "customer")
		if e != nil {
			t.Fatal(e)
		}
		*p, e = f.s.Authenticate(f.ctx, session.AccessToken)
		if e != nil {
			t.Fatal(e)
		}
	}
	return f
}
func (f *fixture) quote(t *testing.T, kind string, amount Money) (Quote, string) {
	t.Helper()
	in := QuoteInput{Kind: kind, Amount: amount, Currency: "NGN", Narration: "Synthetic payment"}
	if kind == "internal" {
		in.RecipientID = f.b.User.ID
	} else {
		d := Destination{BankCode: "999", AccountNumber: "1234567890"}
		enquiryKind := "bank"
		if kind == "bill" {
			enquiryKind = "bill"
			d = Destination{ProductID: "test-electricity", CustomerID: "SYNTHETIC-CUSTOMER"}
		}
		result, e := f.s.Enquire(f.ctx, f.a, d, amount, enquiryKind)
		if e != nil {
			t.Fatal(e)
		}
		id := result["id"].(string)
		if kind == "bank" {
			in.BeneficiaryID, e = f.s.BeneficiaryCreate(f.ctx, f.a, id, "Synthetic beneficiary")
			if e != nil {
				t.Fatal(e)
			}
		} else {
			in.ValidationID = id
		}
	}
	q, e := f.s.CreateQuote(f.ctx, f.a, in)
	if e != nil {
		t.Fatal(e)
	}
	authorisation, e := f.s.Authorise(f.ctx, f.a, q.ID, "123456", "")
	if e != nil {
		t.Fatal(e)
	}
	return q, authorisation["authorisation_token"].(string)
}
func (f *fixture) payment(t *testing.T, kind string, amount Money) Payment {
	t.Helper()
	q, token := f.quote(t, kind, amount)
	p, e := f.s.CreatePayment(f.ctx, f.a, q.ID, token, security.Random("idem_", 18))
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func (f *fixture) work(t *testing.T) {
	t.Helper()
	if _, e := f.s.DB.Exec(`UPDATE jobs SET available_at=$1`, f.clock); e != nil {
		t.Fatal(e)
	}
	worked, e := f.s.WorkPayment(f.ctx)
	if e != nil || !worked {
		t.Fatalf("work: %v worked=%v", e, worked)
	}
}
func (f *fixture) balance(t *testing.T, owner string) Balance {
	t.Helper()
	b, e := f.s.Balance(f.ctx, owner)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func (f *fixture) staff(t *testing.T, role string) Principal {
	t.Helper()
	id := security.Random("staff_", 18)
	email := id + "@example.invalid"
	hash, e := security.Password("staff-password-123", f.s.Config.Pepper)
	if e != nil {
		t.Fatal(e)
	}
	_, e = f.s.DB.Exec(`INSERT INTO users(id,email,name,password_hash,role,verified,mfa_enabled) VALUES($1,$2,'Synthetic Operator',$3,$4,true,true)`, id, email, hash, role)
	if e != nil {
		t.Fatal(e)
	}
	u, e := f.s.user(f.ctx, f.s.DB, id, false)
	if e != nil {
		t.Fatal(e)
	}
	var session Session
	e = f.s.transact(f.ctx, func(tx *sql.Tx) error {
		var err error
		session, err = f.s.newSession(f.ctx, tx, u, "web", "test-operator", "staff", true)
		return err
	})
	if e != nil {
		t.Fatal(e)
	}
	p, e := f.s.Authenticate(f.ctx, session.AccessToken)
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestMigrationIdempotenceAndDrift(t *testing.T) {
	db, _ := database(t)
	ctx := context.Background()
	if e := Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(`UPDATE schema_migrations SET checksum='drift' WHERE version=1`); e != nil {
		t.Fatal(e)
	}
	if e := Migrate(ctx, db); e == nil {
		t.Fatal("schema drift accepted")
	}
}
func TestMoneyContract(t *testing.T) {
	for _, text := range []string{`1`, `1.1`, `"1.1"`, `"-1"`, `"01"`, `"1e3"`, `"9000000000000001"`, `null`} {
		t.Run(text, func(t *testing.T) {
			var amount Money
			if json.Unmarshal([]byte(text), &amount) == nil {
				t.Fatal("invalid amount accepted")
			}
		})
	}
	var m Money
	if e := json.Unmarshal([]byte(`"9000000000000000"`), &m); e != nil {
		t.Fatal(e)
	}
	out, _ := json.Marshal(m)
	if string(out) != `"9000000000000000"` {
		t.Fatal(string(out))
	}
}
func TestInternalTransferAndBalancedImmutableLedger(t *testing.T) {
	f := setup(t)
	p := f.payment(t, "internal", 12345)
	if p.Status != "succeeded" {
		t.Fatal(p.Status)
	}
	if f.balance(t, f.a.User.ID).Book != 10000000-12345 || f.balance(t, f.b.User.ID).Book != 10000000+12345 {
		t.Fatal("wrong balances")
	}
	var bad int
	if e := f.s.DB.QueryRow(`SELECT count(*) FROM (SELECT j.id FROM journals j LEFT JOIN postings p ON p.journal_id=j.id GROUP BY j.id HAVING count(p.id)<2 OR sum(p.amount::numeric)<>0) x`).Scan(&bad); e != nil || bad != 0 {
		t.Fatalf("unbalanced journals: %v %d", e, bad)
	}
	if _, e := f.s.DB.Exec(`UPDATE postings SET amount=amount+1`); e == nil {
		t.Fatal("posting update allowed")
	}
	tx, e := f.s.DB.Begin()
	if e != nil {
		t.Fatal(e)
	}
	_, e = tx.Exec(`INSERT INTO journals(id,reference,currency,kind) VALUES('invalid_journal','invalid_journal','NGN','test')`)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e == nil {
		t.Fatal("empty journal committed")
	}
	if _, e = f.s.DB.Exec(`INSERT INTO postings(journal_id,account_id,amount) SELECT id,'house:clearing',1 FROM journals LIMIT 1`); e == nil {
		t.Fatal("append to old journal accepted")
	}
}
func TestConcurrentIdempotencyAndPayloadConflict(t *testing.T) {
	f := setup(t)
	q, token := f.quote(t, "internal", 10000)
	var wg sync.WaitGroup
	ids := make(chan string, 12)
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, e := f.s.CreatePayment(f.ctx, f.a, q.ID, token, "same-idempotency-key-0001")
			ids <- p.ID
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
	var first string
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatal("different payment effects")
		}
	}
	if f.balance(t, f.a.User.ID).Book != 9990000 {
		t.Fatal("duplicate debit")
	}
	other, auth := f.quote(t, "internal", 20000)
	if _, e := f.s.CreatePayment(f.ctx, f.a, other.ID, auth, "same-idempotency-key-0001"); e == nil {
		t.Fatal("changed request accepted for same key")
	}
}
func TestConcurrentOverspendRejectedAtomically(t *testing.T) {
	f := setup(t)
	q1, a1 := f.quote(t, "internal", 6000000)
	q2, a2 := f.quote(t, "internal", 6000000)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i, pair := range []struct {
		q    Quote
		auth string
	}{{q1, a1}, {q2, a2}} {
		wg.Add(1)
		go func(i int, pair struct {
			q    Quote
			auth string
		}) {
			defer wg.Done()
			_, e := f.s.CreatePayment(f.ctx, f.a, pair.q.ID, pair.auth, fmt.Sprintf("overspend-request-%04d", i))
			errs <- e
		}(i, pair)
	}
	wg.Wait()
	close(errs)
	success := 0
	for e := range errs {
		if e == nil {
			success++
		}
	}
	if success != 1 || f.balance(t, f.a.User.ID).Book != 4000000 {
		t.Fatalf("overspend guard failed: %d", success)
	}
}
func TestQuoteAuthorisationBindingAndExpiry(t *testing.T) {
	f := setup(t)
	q1, token := f.quote(t, "internal", 100)
	q2, _ := f.quote(t, "internal", 200)
	if _, e := f.s.CreatePayment(f.ctx, f.a, q2.ID, token, "mismatched-quote-auth"); e == nil {
		t.Fatal("authorisation moved to another quote")
	}
	f.clock = f.clock.Add(6 * time.Minute)
	if _, e := f.s.CreatePayment(f.ctx, f.a, q1.ID, token, "expired-quote-request"); e == nil {
		t.Fatal("expired quote accepted")
	}
	if f.balance(t, f.a.User.ID).Book != 10000000 {
		t.Fatal("failed authorisation changed balance")
	}
}
func TestChangedPolicyInvalidatesQuote(t *testing.T) {
	f := setup(t)
	q, token := f.quote(t, "internal", 1000)
	if _, e := f.s.DB.Exec(`UPDATE policies SET version=version+1`); e != nil {
		t.Fatal(e)
	}
	if _, e := f.s.CreatePayment(f.ctx, f.a, q.ID, token, "policy-change-request"); e == nil {
		t.Fatal("stale policy quote executed")
	}
}
func TestScopedFinancialReads(t *testing.T) {
	f := setup(t)
	q, _ := f.quote(t, "bank", 1000)
	if _, e := f.s.Quote(f.ctx, f.b, q.ID); e == nil {
		t.Fatal("cross-owner quote read")
	}
	p := f.payment(t, "bank", 1000)
	if _, e := f.s.Payment(f.ctx, f.b.User.ID, p.ID); e == nil {
		t.Fatal("cross-owner bank payment read")
	}
	internal := f.payment(t, "internal", 500)
	got, e := f.s.Payment(f.ctx, f.b.User.ID, internal.ID)
	if e != nil || got.Direction != "incoming" {
		t.Fatal("internal recipient cannot see own transfer")
	}
}
func TestBankPaymentHoldAndSingleCapture(t *testing.T) {
	f := setup(t)
	p := f.payment(t, "bank", 12000)
	if f.balance(t, f.a.User.ID).Held != 12100 {
		t.Fatal("hold missing")
	}
	f.work(t)
	if f.balance(t, f.a.User.ID).Book != 10000000 {
		t.Fatal("pending submission charged book balance")
	}
	f.gateway.state = "succeeded"
	f.work(t)
	balance := f.balance(t, f.a.User.ID)
	if balance.Held != 0 || balance.Book != 9987900 {
		t.Fatal(balance)
	}
	got, e := f.s.Payment(f.ctx, f.a.User.ID, p.ID)
	if e != nil || got.Status != "succeeded" {
		t.Fatal(got, e)
	}
	if worked, e := f.s.WorkPayment(f.ctx); e != nil || worked {
		t.Fatal("completed payment re-executed", e)
	}
	if f.gateway.submits != 1 || f.gateway.queries != 1 {
		t.Fatal("unexpected provider calls")
	}
}
func TestLostSubmissionResponseQueriesOriginal(t *testing.T) {
	f := setup(t)
	f.gateway.failSubmit = true
	p := f.payment(t, "bank", 1000)
	f.work(t)
	f.gateway.state = "succeeded"
	f.work(t)
	result, e := f.s.Payment(f.ctx, f.a.User.ID, p.ID)
	if e != nil || result.Status != "succeeded" || f.gateway.submits != 1 || f.gateway.queries != 1 {
		t.Fatalf("unsafe recovery: %+v %v", result, e)
	}
}
func TestFailureReleasesHoldWithoutDebit(t *testing.T) {
	f := setup(t)
	f.gateway.state = "failed"
	p := f.payment(t, "bank", 1000)
	f.work(t)
	b := f.balance(t, f.a.User.ID)
	if b.Held != 0 || b.Book != 10000000 {
		t.Fatal(b)
	}
	if _, e := f.s.Receipt(f.ctx, f.a.User.ID, p.ID); e == nil {
		t.Fatal("failure emitted success receipt")
	}
}
func TestConflictingEvidenceRetainsFundsForReview(t *testing.T) {
	f := setup(t)
	f.gateway.state = "succeeded"
	f.gateway.wrongAmount = true
	p := f.payment(t, "bank", 1000)
	f.work(t)
	v, e := f.s.Payment(f.ctx, f.a.User.ID, p.ID)
	if e != nil || v.Status != "pending_review" {
		t.Fatal(v, e)
	}
	if b := f.balance(t, f.a.User.ID); b.Held != 1100 || b.Book != 10000000 {
		t.Fatal(b)
	}
}
func TestBillFinancialSuccessAndFulfilmentAreSeparate(t *testing.T) {
	f := setup(t)
	f.gateway.state = "succeeded"
	p := f.payment(t, "bill", 10000)
	f.work(t)
	v, e := f.s.Payment(f.ctx, f.a.User.ID, p.ID)
	if e != nil || v.Status != "succeeded" || v.FulfilmentStatus != "pending" {
		t.Fatal(v, e)
	}
	if pending, err := f.s.Fulfilment(f.ctx, f.a.User.ID, p.ID); err != nil || pending["status"] != "pending" || pending["value"] != nil {
		t.Fatalf("unexpected pending fulfilment: %#v %v", pending, err)
	}
	f.gateway.fulfilled = true
	f.work(t)
	value, e := f.s.Fulfilment(f.ctx, f.a.User.ID, p.ID)
	if e != nil || value["value"] != "SYNTHETIC TOKEN" {
		t.Fatal(value, e)
	}
	if f.gateway.submits != 1 || f.balance(t, f.a.User.ID).Book != 9989900 {
		t.Fatal("bill recovery duplicated vend or debit")
	}
}
func TestCrashBeforeNetworkDoesNotBlindlySubmitAgain(t *testing.T) {
	f := setup(t)
	p := f.payment(t, "bank", 1000)
	if _, e := f.s.DB.Exec(`UPDATE payments SET status='submitted' WHERE id=$1`, p.ID); e != nil {
		t.Fatal(e)
	}
	f.work(t)
	if f.gateway.submits != 0 || f.gateway.queries != 1 {
		t.Fatal("uncertain operation was newly submitted")
	}
	if f.balance(t, f.a.User.ID).Held != 1100 {
		t.Fatal("hold released without outcome")
	}
}
func TestFundingIdempotencyAndConflict(t *testing.T) {
	f := setup(t)
	first, e := f.s.CreditFunding(f.ctx, "verified-test-provider", "external_reference_001", f.a.User.ID, 500, "NGN")
	if e != nil || first == "" {
		t.Fatal(first, e)
	}
	second, e := f.s.CreditFunding(f.ctx, "verified-test-provider", "external_reference_001", f.a.User.ID, 500, "NGN")
	if e != nil || first != second {
		t.Fatal(first, second, e)
	}
	if _, e = f.s.CreditFunding(f.ctx, "verified-test-provider", "external_reference_001", f.b.User.ID, 500, "NGN"); e == nil {
		t.Fatal("cross-customer funding conflict accepted")
	}
	if f.balance(t, f.a.User.ID).Book != 10000500 {
		t.Fatal("funding duplicated")
	}
}
func TestConcurrentFundingHasOneEffect(t *testing.T) {
	f := setup(t)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := f.s.CreditFunding(f.ctx, "verified-test-provider", "shared_reference_001", f.a.User.ID, 500, "NGN")
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if f.balance(t, f.a.User.ID).Book != 10000500 {
		t.Fatal("duplicate concurrent funding")
	}
}
func TestRefreshRotationAndReuseRevokesFamily(t *testing.T) {
	f := setup(t)
	first, e := f.s.Login(f.ctx, LoginInput{Email: f.a.User.Email, Password: "test-password-123", Client: "mobile", Device: "second-device"}, "customer")
	if e != nil {
		t.Fatal(e)
	}
	second, e := f.s.Refresh(f.ctx, first.RefreshToken, "", "mobile", "customer")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.Authenticate(f.ctx, first.AccessToken); e == nil {
		t.Fatal("old access token still valid")
	}
	if _, e = f.s.Refresh(f.ctx, first.RefreshToken, "", "mobile", "customer"); e == nil {
		t.Fatal("reused refresh accepted")
	}
	if _, e = f.s.Authenticate(f.ctx, second.AccessToken); e == nil {
		t.Fatal("replayed session family not revoked")
	}
}
func TestRegistrationVerificationAndSecretInboxIsolation(t *testing.T) {
	f := setup(t)
	email := "new@example.invalid"
	for i := 0; i < 2; i++ {
		if e := f.s.Register(f.ctx, email, "New Synthetic Customer", "new-password-123"); e != nil {
			t.Fatal(e)
		}
	}
	var count int
	if e := f.s.DB.QueryRow(`SELECT count(*) FROM users WHERE email=$1`, email).Scan(&count); e != nil || count != 1 {
		t.Fatal(count, e)
	}
	message, e := f.s.LocalChallenge(f.ctx, email)
	if e != nil {
		t.Fatal(e)
	}
	id := message["notification_id"].(string)
	body, _ := json.Marshal(message)
	policy, e := f.s.NotificationPolicy(f.ctx, id, body)
	if e != nil || policy["allowed"] != true {
		t.Fatal(policy, e)
	}
	message["name"] = "Changed"
	body, _ = json.Marshal(message)
	policy, e = f.s.NotificationPolicy(f.ctx, id, body)
	if e != nil || policy["allowed"] != false {
		t.Fatal("modified stored payload accepted")
	}
	if e = f.s.VerifyChallenge(f.ctx, email, "verify", message["code"].(string), ""); e != nil {
		t.Fatal(e)
	}
	policy, e = f.s.NotificationPolicy(f.ctx, id, nil)
	if e != nil || policy["allowed"] != false {
		t.Fatal("used challenge can still send")
	}
	var uid string
	if e = f.s.DB.QueryRow(`SELECT id FROM users WHERE email=$1`, email).Scan(&uid); e != nil {
		t.Fatal(e)
	}
	notices, e := f.s.Notifications(f.ctx, uid, 100, "")
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range notices["items"].([]map[string]any) {
		if v["workflow"] == "identity-verify-email" {
			t.Fatal("secret challenge in customer inbox")
		}
	}
}
func TestFailedVerificationAttemptsPersist(t *testing.T) {
	f := setup(t)
	email := "attempts@example.invalid"
	if e := f.s.Register(f.ctx, email, "Attempts Customer", "new-password-123"); e != nil {
		t.Fatal(e)
	}
	message, e := f.s.LocalChallenge(f.ctx, email)
	if e != nil {
		t.Fatal(e)
	}
	wrong := "000000"
	if message["code"] == wrong {
		wrong = "999999"
	}
	for i := 0; i < 5; i++ {
		if e = f.s.VerifyChallenge(f.ctx, email, "verify", wrong, ""); e == nil {
			t.Fatal("wrong code accepted")
		}
	}
	if e = f.s.VerifyChallenge(f.ctx, email, "verify", message["code"].(string), ""); e == nil {
		t.Fatal("exhausted challenge accepted")
	}
}
func TestTOTPEnrolmentAndExpiredPendingSecret(t *testing.T) {
	f := setup(t)
	enrol, e := f.s.MFAStart(f.ctx, f.a, "test-password-123", "")
	if e != nil {
		t.Fatal(e)
	}
	code, e := security.TOTP(enrol["secret"], f.clock.Unix()/30)
	if e != nil {
		t.Fatal(e)
	}
	codes, e := f.s.MFAConfirm(f.ctx, f.a, code)
	if e != nil || len(codes) != 10 {
		t.Fatal(codes, e)
	}
	if _, e = f.s.Login(f.ctx, LoginInput{Email: f.a.User.Email, Password: "test-password-123", Client: "mobile", Device: "totp-device", MFACode: code}, "customer"); e == nil {
		t.Fatal("replayed TOTP accepted")
	}
	session, e := f.s.Login(f.ctx, LoginInput{Email: f.a.User.Email, Password: "test-password-123", Client: "mobile", Device: "recovery-device", RecoveryCode: codes[0]}, "customer")
	if e != nil || session.AccessToken == "" {
		t.Fatal(e)
	}
	if _, e = f.s.Login(f.ctx, LoginInput{Email: f.a.User.Email, Password: "test-password-123", Client: "mobile", Device: "recovery-device", RecoveryCode: codes[0]}, "customer"); e == nil {
		t.Fatal("recovery code reused")
	}
	pending, e := f.s.MFAStart(f.ctx, f.b, "test-password-123", "")
	if e != nil {
		t.Fatal(e)
	}
	f.clock = f.clock.Add(11 * time.Minute)
	code, _ = security.TOTP(pending["secret"], f.clock.Unix()/30)
	if _, e = f.s.MFAConfirm(f.ctx, f.b, code); e == nil {
		t.Fatal("expired enrolment accepted")
	}
}
func TestIndependentPolicyApprovalAndStaleProposal(t *testing.T) {
	f := setup(t)
	maker := f.staff(t, "admin")
	checker := f.staff(t, "finance")
	in := ProposalInput{Action: "policy", Reason: "Synthetic policy review", Policy: &PolicyInput{PerPayment: 100000, Daily: 200000, InternalFee: 10, Enabled: true}}
	id, e := f.s.Propose(f.ctx, maker, in)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.s.Decide(f.ctx, maker, id, true); e == nil {
		t.Fatal("self approval accepted")
	}
	if e = f.s.Decide(f.ctx, checker, id, true); e != nil {
		t.Fatal(e)
	}
	if e = f.s.Decide(f.ctx, checker, id, true); e == nil {
		t.Fatal("decision replay unexpectedly re-executed")
	}
	policy, e := f.s.Policy(f.ctx)
	if e != nil || policy["version"] != int64(2) {
		t.Fatal(policy, e)
	}
}
func TestRestrictedOperatorCannotApprove(t *testing.T) {
	f := setup(t)
	maker := f.staff(t, "admin")
	checker := f.staff(t, "compliance")
	id, e := f.s.Propose(f.ctx, maker, ProposalInput{Action: "restrict", Target: f.a.User.ID, Reason: "Synthetic restriction review"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.DB.Exec(`UPDATE users SET status='restricted' WHERE id=$1`, checker.User.ID); e != nil {
		t.Fatal(e)
	}
	if e = f.s.Decide(f.ctx, checker, id, true); e == nil {
		t.Fatal("restricted staff approved")
	}
}
func TestSupportOwnershipAndEncryptedMessages(t *testing.T) {
	f := setup(t)
	id, e := f.s.CaseCreate(f.ctx, f.a, CaseInput{Kind: "support", Subject: "Synthetic question", Message: "Private synthetic message"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.CaseMessages(f.ctx, f.b, id); e == nil {
		t.Fatal("cross-owner case read")
	}
	if e = f.s.CaseReply(f.ctx, f.b, id, "unauthorised reply", ""); e == nil {
		t.Fatal("cross-owner reply")
	}
	staff := f.staff(t, "support")
	if e = f.s.CaseReply(f.ctx, staff, id, "Approved synthetic reply", "resolved"); e != nil {
		t.Fatal(e)
	}
	messages, e := f.s.CaseMessages(f.ctx, f.a, id)
	if e != nil || len(messages) != 2 {
		t.Fatal(messages, e)
	}
	var plaintext bool
	if e = f.s.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM case_messages WHERE body LIKE '%Private synthetic%')`).Scan(&plaintext); e != nil || plaintext {
		t.Fatal("plaintext support body persisted")
	}
}
func TestStatementAndReconciliationDoNotChangeMoney(t *testing.T) {
	f := setup(t)
	p := f.payment(t, "internal", 1000)
	f.clock = time.Now().UTC()
	data, e := f.s.Statement(f.ctx, f.a.User.ID, f.clock.Add(-time.Hour), f.clock.Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if data["closing_minor"] != "9999000" {
		t.Fatal(data)
	}
	csv, e := StatementCSV(data)
	if e != nil || !strings.Contains(csv, "internal_transfer") {
		t.Fatal(csv, e)
	}
	operator := f.staff(t, "finance")
	result, e := f.s.Reconcile(f.ctx, operator, ReconciliationInput{Source: "provider", Entries: []ReconciliationEntry{{Reference: p.ID, Amount: 1000, Currency: "NGN", Status: "succeeded"}}})
	if e != nil {
		t.Fatal(e)
	}
	if !result["results"].([]map[string]any)[0]["matched"].(bool) {
		t.Fatal(result)
	}
	if f.balance(t, f.a.User.ID).Book != 9999000 {
		t.Fatal("reconciliation changed money")
	}
}
func TestFreshServiceInstanceRecoversPersistentState(t *testing.T) {
	f := setup(t)
	p := f.payment(t, "bank", 1000)
	db, e := sql.Open("pgx", f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	fresh := New(db, f.s.Config)
	fresh.Now = f.s.Now
	got, e := fresh.Payment(f.ctx, f.a.User.ID, p.ID)
	if e != nil || got.Status != "accepted" {
		t.Fatal(got, e)
	}
	balance, e := fresh.Balance(f.ctx, f.a.User.ID)
	if e != nil || balance.Held != 1100 {
		t.Fatal(balance, e)
	}
}
