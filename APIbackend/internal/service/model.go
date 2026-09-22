package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
)

type Money int64

const MaxMoney Money = 9000000000000000

var digits = regexp.MustCompile(`^(0|[1-9][0-9]{0,15})$`)

func (m Money) MarshalJSON() ([]byte, error) { return json.Marshal(strconv.FormatInt(int64(m), 10)) }
func (m *Money) UnmarshalJSON(raw []byte) error {
	var text string
	if e := json.Unmarshal(raw, &text); e != nil {
		return errors.New("money must be an integer minor-unit string")
	}
	if !digits.MatchString(text) {
		return errors.New("invalid money")
	}
	n, e := strconv.ParseInt(text, 10, 64)
	if e != nil || n > int64(MaxMoney) {
		return errors.New("money outside supported range")
	}
	*m = Money(n)
	return nil
}
func addMoney(a, b Money) (Money, error) {
	if a < 0 || b < 0 || a > MaxMoney-b {
		return 0, Invalid("amount outside supported range")
	}
	return a + b, nil
}

type Fault struct {
	Status  int
	Code    string
	Message string
}

func (f *Fault) Error() string      { return f.Code + ": " + f.Message }
func Invalid(message string) error  { return &Fault{400, "invalid_request", message} }
func denied() error                 { return &Fault{403, "forbidden", "operation not permitted"} }
func missing() error                { return &Fault{404, "not_found", "resource not found"} }
func conflict(message string) error { return &Fault{409, "conflict", message} }
func unauthorized() error {
	return &Fault{401, "unauthorized", "invalid credentials or expired session"}
}
func unavailable() error {
	return &Fault{503, "upstream_unavailable", "service is not available; existing operations remain queryable"}
}
func isMissing(e error) error {
	if errors.Is(e, sql.ErrNoRows) {
		return missing()
	}
	return e
}

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	Verified  bool      `json:"email_verified"`
	Tier      int       `json:"tier"`
	MFA       bool      `json:"mfa_enabled"`
	PINSet    bool      `json:"pin_set"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}
type Principal struct {
	User      User
	SessionID string
	Client    string
	Audience  string
	MFAReady  bool
	CSRFHash  string
}
type Session struct {
	AccessToken      string    `json:"access_token,omitempty"`
	RefreshToken     string    `json:"refresh_token,omitempty"`
	CSRFToken        string    `json:"csrf_token,omitempty"`
	ExpiresAt        time.Time `json:"expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	MFARequired      bool      `json:"mfa_enrolment_required"`
	User             User      `json:"user"`
}
type Balance struct {
	Currency  string `json:"currency"`
	Available Money  `json:"available_minor"`
	Held      Money  `json:"held_minor"`
	Book      Money  `json:"balance_minor"`
}
type Destination struct {
	BillerID      string `json:"biller_id,omitempty"`
	CategoryID    string `json:"category_id,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	BankCode      string `json:"bank_code,omitempty"`
	AccountNumber string `json:"account_number,omitempty"`
	AccountName   string `json:"account_name,omitempty"`
	ProductID     string `json:"product_id,omitempty"`
	CustomerID    string `json:"customer_id,omitempty"`
	ValidationID  string `json:"validation_id,omitempty"`
}
type QuoteInput struct {
	Kind          string `json:"kind"`
	Amount        Money  `json:"amount_minor"`
	Currency      string `json:"currency"`
	BeneficiaryID string `json:"beneficiary_id,omitempty"`
	ValidationID  string `json:"validation_id,omitempty"`
	RecipientID   string `json:"recipient_id,omitempty"`
	Narration     string `json:"narration"`
}
type Quote struct {
	ID            string      `json:"id"`
	Kind          string      `json:"kind"`
	Amount        Money       `json:"amount_minor"`
	Fee           Money       `json:"fee_minor"`
	Total         Money       `json:"total_minor"`
	Currency      string      `json:"currency"`
	Destination   Destination `json:"destination"`
	Narration     string      `json:"narration"`
	PolicyVersion int64       `json:"policy_version"`
	ExpiresAt     time.Time   `json:"expires_at"`
	Used          bool        `json:"used"`
}
type Payment struct {
	OwnerID           string    `json:"-"`
	RecipientID       string    `json:"-"`
	Direction         string    `json:"direction"`
	ID                string    `json:"id"`
	QuoteID           string    `json:"quote_id"`
	Kind              string    `json:"kind"`
	Status            string    `json:"status"`
	Amount            Money     `json:"amount_minor"`
	Fee               Money     `json:"fee_minor"`
	Total             Money     `json:"total_minor"`
	Currency          string    `json:"currency"`
	FulfilmentStatus  string    `json:"fulfilment_status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	ProviderReference string    `json:"provider_reference,omitempty"`
}
type ProviderResult struct {
	UpstreamID string `json:"upstream_id,omitempty"`
	Fulfilled  bool   `json:"fulfilled"`
	Reference  string `json:"reference"`
	Status     string `json:"status"`
	Amount     Money  `json:"amount_minor"`
	Currency   string `json:"currency"`
	Fulfilment string `json:"fulfilment,omitempty"`
}
type ProviderRequest struct {
	UpstreamID  string
	Reference   string
	Kind        string
	Amount      Money
	Currency    string
	Destination Destination
	Narration   string
}
type Bank struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
type Product struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Amount   Money  `json:"amount_minor"`
	Variable bool   `json:"variable_amount"`
}
type Enquiry struct {
	Name      string `json:"name"`
	Reference string `json:"reference"`
}
type Gateway interface {
	Banks(context.Context) ([]Bank, error)
	Products(context.Context) ([]Product, error)
	Enquire(context.Context, Destination) (Enquiry, error)
	ValidateBill(context.Context, Destination, Money) (Enquiry, error)
	Quote(context.Context, string, Destination, Money) (Money, error)
	Submit(context.Context, ProviderRequest) (ProviderResult, error)
	Query(context.Context, ProviderRequest) (ProviderResult, error)
}

type Config struct {
	Environment      string
	Pepper           []byte
	Box              security.Box
	Gateway          Gateway
	ExternalEnabled  bool
	NotificationMode string
	NovuURL          string
	NovuKey          string
	NovuAllowlist    map[string]bool
	PolicyKey        []byte
	Origins          []string
	PolicyAccepted   bool
}
type Service struct {
	DB     *sql.DB
	Config Config
	Now    func() time.Time
}

func New(db *sql.DB, c Config) *Service {
	return &Service{DB: db, Config: c, Now: func() time.Time { return time.Now().UTC() }}
}
func (s *Service) transact(ctx context.Context, fn func(*sql.Tx) error) error {
	for i := 0; i < 4; i++ {
		tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if e != nil {
			return e
		}
		e = fn(tx)
		if e == nil {
			e = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		if e == nil {
			return nil
		}
		_ = tx.Rollback()
		var state interface{ SQLState() string }
		if !errors.As(e, &state) || (state.SQLState() != "40001" && state.SQLState() != "40P01") {
			return e
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(i+1) * 15 * time.Millisecond):
		}
	}
	return &Fault{503, "retry_transaction", "transaction could not be committed; retry with the same idempotency key"}
}
func exec(tx *sql.Tx, ctx context.Context, q string, args ...any) error {
	_, e := tx.ExecContext(ctx, q, args...)
	return e
}
func jsonText(v any) string {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return string(b)
}
func safeText(v string, max int) bool {
	return len(v) > 0 && len(v) <= max && !strings.ContainsFunc(v, unicode.IsControl)
}
func validID(v string) bool {
	return len(v) >= 8 && len(v) <= 100 && regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(v)
}
func (s *Service) audit(ctx context.Context, tx *sql.Tx, actor, action, target string, data any) error {
	return exec(tx, ctx, `INSERT INTO audit_events(id,actor,action,target,details,created_at) VALUES($1,$2,$3,$4,$5,$6)`, security.Random("aud_", 18), actor, action, target, jsonText(data), s.Now())
}
func (s *Service) user(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string, lock bool) (User, error) {
	var u User
	query := `SELECT id,email,name,role,status,verified,tier,mfa_enabled,pin_hash<>'',version,created_at FROM users WHERE id=$1`
	if lock {
		query += " FOR UPDATE"
	}
	e := q.QueryRowContext(ctx, query, id).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.Status, &u.Verified, &u.Tier, &u.MFA, &u.PINSet, &u.Version, &u.CreatedAt)
	return u, isMissing(e)
}
func requireRole(p Principal, roles ...string) error {
	if p.Audience != "staff" || !p.MFAReady || p.User.Status != "active" {
		return denied()
	}
	for _, r := range roles {
		if p.User.Role == r {
			return nil
		}
	}
	return denied()
}
func (s *Service) eligible(u User) error {
	if u.Role != "customer" || u.Status != "active" || !u.Verified || u.Tier < 1 {
		return &Fault{403, "account_not_eligible", "verified and approved active account required"}
	}
	if s.Config.Environment == "production" && !s.Config.PolicyAccepted {
		return &Fault{503, "policy_not_accepted", "production account policy not accepted"}
	}
	return nil
}
func scanPayment(row interface{ Scan(...any) error }) (Payment, error) {
	var p Payment
	e := row.Scan(&p.ID, &p.QuoteID, &p.Kind, &p.Status, &p.Amount, &p.Fee, &p.Total, &p.Currency, &p.FulfilmentStatus, &p.CreatedAt, &p.UpdatedAt, &p.ProviderReference, &p.OwnerID, &p.RecipientID)
	return p, isMissing(e)
}

const paymentColumns = `id,quote_id,kind,status,amount,fee,total,currency,fulfilment_status,created_at,updated_at,provider_reference,owner_id,coalesce(recipient_id,'')`

func (s *Service) Payment(ctx context.Context, owner, id string) (Payment, error) {
	p, e := scanPayment(s.DB.QueryRowContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE id=$1 AND (owner_id=$2 OR recipient_id=$2)`, id, owner))
	p.Direction = "outgoing"
	if p.RecipientID == owner {
		p.Direction = "incoming"
	}
	return p, e
}
func (s *Service) Payments(ctx context.Context, owner string, limit int, before string) ([]Payment, error) {
	if limit < 1 || limit > 100 {
		return nil, Invalid("limit must be 1 to 100")
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT `+paymentColumns+` FROM payments WHERE (owner_id=$1 OR recipient_id=$1) AND ($2='' OR id<$2) ORDER BY id DESC LIMIT $3`, owner, before, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Payment{}
	for rows.Next() {
		p, e := scanPayment(rows)
		if e != nil {
			return nil, e
		}
		p.Direction = "outgoing"
		if p.RecipientID == owner {
			p.Direction = "incoming"
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Service) Ping(ctx context.Context) error {
	var version int
	return s.DB.QueryRowContext(ctx, `SELECT version FROM schema_migrations WHERE version=1`).Scan(&version)
}
func FaultStatus(err error) (int, string, string) {
	var f *Fault
	if errors.As(err, &f) {
		return f.Status, f.Code, f.Message
	}
	return 500, "internal_error", "request could not be completed"
}
func ExternalError(err error) error {
	if err == nil {
		return nil
	}
	return unavailable()
}
func (p Principal) Customer() error {
	if p.Audience != "customer" || p.User.Role != "customer" {
		return denied()
	}
	return nil
}
func stringID(prefix string) string {
	return fmt.Sprintf("%s%013d_%s", prefix, time.Now().UnixMilli(), security.Random("", 12))
}
