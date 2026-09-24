package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/service"
)

type RegisterInput struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}
type ChallengeInput struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"`
}
type VerifyInput struct {
	Email    string `json:"email"`
	Purpose  string `json:"purpose"`
	Code     string `json:"code"`
	Password string `json:"new_password,omitempty"`
}
type RefreshInput struct {
	Client       string `json:"client"`
	RefreshToken string `json:"refresh_token,omitempty"`
}
type PasswordProof struct {
	Password string `json:"password"`
	MFACode  string `json:"mfa_code,omitempty"`
}
type CodeInput struct {
	Code string `json:"code"`
}
type CredentialInput struct {
	Password string `json:"password"`
	MFACode  string `json:"mfa_code,omitempty"`
	NewValue string `json:"new_value"`
}
type NameInput struct {
	Name string `json:"name"`
}
type BankInput struct {
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
}
type BillInput struct {
	ProductID  string        `json:"product_id"`
	CustomerID string        `json:"customer_id"`
	Amount     service.Money `json:"amount_minor"`
}
type BeneficiaryInput struct {
	EnquiryID string `json:"enquiry_id"`
	Label     string `json:"label"`
}
type AuthorisationInput struct {
	PIN     string `json:"pin"`
	MFACode string `json:"mfa_code,omitempty"`
}
type PaymentInput struct {
	QuoteID       string `json:"quote_id"`
	Authorisation string `json:"authorisation_token"`
}
type KYCInput struct {
	EvidenceReference string `json:"evidence_reference"`
}
type PreferencesInput struct {
	Optional  bool `json:"optional_email"`
	Marketing bool `json:"marketing_email"`
}
type CaseReplyInput struct {
	Message string `json:"message"`
	Status  string `json:"status,omitempty"`
}
type StaffInput struct {
	Email             string `json:"email"`
	Name              string `json:"name"`
	TemporaryPassword string `json:"temporary_password"`
	Role              string `json:"role"`
}
type DecisionInput struct {
	Approve bool `json:"approve"`
}
type PolicyRequest struct {
	NotificationID string          `json:"notification_id"`
	Payload        json.RawMessage `json:"payload"`
}

func accepted() map[string]string             { return map[string]string{"status": "accepted"} }
func created(id string, e error) (any, error) { return map[string]string{"id": id}, e }
func (h *Handler) register() {
	s := h.Service
	h.parityRoutes()
	h.Router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		h.fail(w, r, &service.Fault{Status: 404, Code: "not_found", Message: "route not found"})
	})
	h.Router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		h.fail(w, r, &service.Fault{Status: 405, Code: "method_not_allowed", Message: "method not allowed"})
	})
	h.get("/health/live", "Process liveness", "public", map[string]string{}, func(*http.Request, service.Principal) (any, error) {
		return map[string]string{"status": "alive", "service": "APIbackend"}, nil
	})
	h.get("/health/ready", "Database and migration readiness", "public", map[string]string{}, func(r *http.Request, _ service.Principal) (any, error) {
		if e := s.Ping(r.Context()); e != nil {
			return nil, &service.Fault{Status: 503, Code: "not_ready", Message: "database or migration unavailable"}
		}
		return map[string]string{"status": "ready"}, nil
	})
	h.add(Route{Method: "GET", Path: "/openapi.json", Summary: "Machine-readable API contract", Auth: "public", Status: 200, Run: func(w http.ResponseWriter, r *http.Request, _ service.Principal) (any, error) {
		h.json(w, 200, h.OpenAPI())
		return rawWritten, nil
	}})
	h.get("/v1/capabilities", "Deployment capabilities; not provider acceptance", "public", map[string]any{}, func(r *http.Request, _ service.Principal) (any, error) {
		return map[string]any{"environment": s.Config.Environment, "currency": "NGN", "external_payments_configured": s.Config.ExternalEnabled, "notifications": s.Config.NotificationMode, "synthetic_execution": s.Config.Environment == "local", "growth_products_enabled": false, "production_acceptance": false}, nil
	})
	jsonRoute(h, "POST", "/v1/auth/register", "Register an unverified customer", "public", 202, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, _ service.Principal, in RegisterInput) (any, error) {
		return accepted(), s.Register(r.Context(), in.Email, in.Name, in.Password)
	})
	jsonRoute(h, "POST", "/v1/auth/challenges", "Request verification or reset without account enumeration", "public", 202, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, _ service.Principal, in ChallengeInput) (any, error) {
		return accepted(), s.RequestChallenge(r.Context(), in.Email, in.Purpose)
	})
	jsonRoute(h, "POST", "/v1/auth/challenges/verify", "Consume a short-lived verification challenge", "public", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, _ service.Principal, in VerifyInput) (any, error) {
		return accepted(), s.VerifyChallenge(r.Context(), in.Email, in.Purpose, in.Code, in.Password)
	})
	h.authRoutes("customer", "/v1/auth")
	h.authRoutes("staff", "/v1/admin/auth")
	h.get("/v1/me", "Current customer profile", "customer", service.User{}, func(r *http.Request, p service.Principal) (any, error) { return s.Me(r.Context(), p.User.ID) })
	jsonRoute(h, "PATCH", "/v1/me", "Update display name", "customer", 200, service.User{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in NameInput) (any, error) {
		return s.UpdateName(r.Context(), p, in.Name)
	})
	h.get("/v1/wallet", "Authoritative available, held and book balances", "customer", service.Balance{}, func(r *http.Request, p service.Principal) (any, error) { return s.Balance(r.Context(), p.User.ID) })
	h.get("/v1/wallet/entries", "Cursor-paginated signed ledger entries", "customer", []service.LedgerEntry{}, func(r *http.Request, p service.Principal) (any, error) {
		before := int64(0)
		if text := r.URL.Query().Get("before"); text != "" {
			n, e := strconv.ParseInt(text, 10, 64)
			if e != nil || n < 0 {
				return nil, service.Invalid("invalid ledger cursor")
			}
			before = n
		}
		return s.Entries(r.Context(), p.User.ID, limit(r), before)
	})
	h.get("/v1/wallet/funding", "Verified posted funding history", "customer", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.FundingHistory(r.Context(), p.User.ID)
	})
	h.add(Route{Method: "GET", Path: "/v1/statements", Summary: "Complete bounded-range JSON or CSV statement", Auth: "customer", Status: 200, Response: map[string]any{}, Run: func(w http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		from, e := time.Parse(time.RFC3339, r.URL.Query().Get("from"))
		if e != nil {
			return nil, service.Invalid("RFC3339 from is required")
		}
		to, e := time.Parse(time.RFC3339, r.URL.Query().Get("to"))
		if e != nil {
			return nil, service.Invalid("RFC3339 to is required")
		}
		data, e := s.Statement(r.Context(), p.User.ID, from, to)
		if e != nil {
			return nil, e
		}
		if r.URL.Query().Get("format") == "csv" {
			csv, e := service.StatementCSV(data)
			if e != nil {
				return nil, e
			}
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="statement.csv"`)
			w.WriteHeader(200)
			_, e = w.Write([]byte(csv))
			return rawWritten, e
		}
		return data, nil
	}})
	h.get("/v1/banks", "Bank directory from selected execution service", "customer", []service.Bank{}, func(r *http.Request, p service.Principal) (any, error) { return s.Banks(r.Context()) })
	h.get("/v1/bills/products", "Normalised bill products", "customer", []service.Product{}, func(r *http.Request, p service.Principal) (any, error) { return s.Products(r.Context()) })
	jsonRoute(h, "POST", "/v1/banks/enquiries", "Verify recipient account with the execution service", "customer", 201, map[string]any{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in BankInput) (any, error) {
		return s.Enquire(r.Context(), p, service.Destination{BankCode: in.BankCode, AccountNumber: in.AccountNumber}, 0, "bank")
	})
	jsonRoute(h, "POST", "/v1/bills/validations", "Validate a bill customer and amount", "customer", 201, map[string]any{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in BillInput) (any, error) {
		return s.Enquire(r.Context(), p, service.Destination{ProductID: in.ProductID, CustomerID: in.CustomerID}, in.Amount, "bill")
	})
	h.get("/v1/beneficiaries", "Saved verified beneficiaries", "customer", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.Beneficiaries(r.Context(), p.User.ID)
	})
	jsonRoute(h, "POST", "/v1/beneficiaries", "Save an owned unexpired account enquiry", "customer", 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in BeneficiaryInput) (any, error) {
		return created(s.BeneficiaryCreate(r.Context(), p, in.EnquiryID, in.Label))
	})
	h.add(Route{Method: "DELETE", Path: "/v1/beneficiaries/{id}", Summary: "Deactivate own beneficiary", Auth: "customer", Status: 204, Run: func(_ http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		return nil, s.BeneficiaryDelete(r.Context(), p, key(r))
	}})
	jsonRoute(h, "POST", "/v1/quotes", "Create immutable quote for internal, bank or bill payment", "customer", 201, service.Quote{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.QuoteInput) (any, error) {
		return s.CreateQuote(r.Context(), p, in)
	})
	h.get("/v1/quotes/{id}", "Recover an existing quote", "customer", service.Quote{}, func(r *http.Request, p service.Principal) (any, error) { return s.Quote(r.Context(), p, key(r)) })
	jsonRoute(h, "POST", "/v1/quotes/{id}/authorisations", "Bind PIN and optional MFA approval to an immutable quote", "customer", 201, map[string]any{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in AuthorisationInput) (any, error) {
		return s.Authorise(r.Context(), p, key(r), in.PIN, in.MFACode)
	})
	jsonRoute(h, "POST", "/v1/payments", "Create or safely recover an idempotent payment intent", "customer", 202, service.Payment{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in PaymentInput) (any, error) {
		return s.CreatePayment(r.Context(), p, in.QuoteID, in.Authorisation, r.Header.Get("Idempotency-Key"))
	})
	h.get("/v1/payments", "Cursor-paginated customer transaction history", "customer", []service.Payment{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.Payments(r.Context(), p.User.ID, limit(r), r.URL.Query().Get("before"))
	})
	h.get("/v1/payments/{id}", "Authoritative payment state after retry or application restart", "customer", service.Payment{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.Payment(r.Context(), p.User.ID, key(r))
	})
	h.get("/v1/payments/{id}/receipt", "Receipt copy only for a confirmed successful payment", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.Receipt(r.Context(), p.User.ID, key(r))
	})
	h.get("/v1/payments/{id}/fulfilment", "Authenticated bill fulfilment retrieval", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.Fulfilment(r.Context(), p.User.ID, key(r))
	})
	jsonRoute(h, "POST", "/v1/kyc/cases", "Submit an identity-review case", "customer", 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in KYCInput) (any, error) {
		return created(s.KYCSubmit(r.Context(), p, in.EvidenceReference))
	})
	h.get("/v1/kyc/cases", "Own identity-review case status", "customer", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.KYCList(r.Context(), p) })
	h.get("/v1/notifications", "Persistent notification centre excluding secret challenges", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.Notifications(r.Context(), p.User.ID, limit(r), r.URL.Query().Get("before"))
	})
	h.add(Route{Method: "POST", Path: "/v1/notifications/{id}/read", Summary: "Mark own non-secret notice read", Auth: "customer", Status: 204, Run: func(_ http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		return nil, s.ReadNotification(r.Context(), p.User.ID, key(r))
	}})
	h.get("/v1/preferences", "Current optional notification preferences", "customer", map[string]bool{}, func(r *http.Request, p service.Principal) (any, error) { return s.Preferences(r.Context(), p.User.ID) })
	jsonRoute(h, "PATCH", "/v1/preferences", "Update optional notification preferences", "customer", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in PreferencesInput) (any, error) {
		return accepted(), s.SetPreferences(r.Context(), p, in.Optional, in.Marketing)
	})
	h.caseRoutes("/v1/support/cases", "customer")
	h.caseRoutes("/v1/admin/support/cases", "staff")
	h.adminRoutes()
	h.consoleRoutes()
	h.backupRoutes()
	h.add(Route{Method: "POST", Path: "/internal/notifications/authorise", Summary: "Signed bridge eligibility check", Auth: "internal", Status: 200, Request: PolicyRequest{}, Response: map[string]any{}, Run: func(_ http.ResponseWriter, r *http.Request, _ service.Principal) (any, error) {
		raw, e := readSigned(r, h.Config.PolicyKey)
		if e != nil {
			return nil, e
		}
		var in PolicyRequest
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&in) != nil {
			return nil, service.Invalid("invalid eligibility request")
		}
		return s.NotificationPolicy(r.Context(), in.NotificationID, in.Payload)
	}})
}
func readSigned(r *http.Request, key []byte) ([]byte, error) {
	if len(key) < 32 {
		return nil, &service.Fault{Status: 503, Code: "integration_disabled", Message: "private integration is not configured"}
	}
	raw, e := ioReadLimit(r)
	if e != nil {
		return nil, e
	}
	if !security.VerifySignature(key, r.Header.Get("X-QPF-Timestamp"), string(raw), r.Header.Get("X-QPF-Signature"), time.Now()) {
		return nil, &service.Fault{Status: 401, Code: "invalid_signature", Message: "invalid integration signature"}
	}
	if e = uniqueJSON(raw); e != nil {
		return nil, service.Invalid("invalid integration body")
	}
	return raw, nil
}
func ioReadLimit(r *http.Request) ([]byte, error) {
	raw, e := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if e != nil || len(raw) > 1<<20 {
		return nil, service.Invalid("invalid integration body size")
	}
	return raw, nil
}
func (h *Handler) authRoutes(audience, prefix string) {
	s := h.Service
	auth := "customer"
	if audience == "staff" {
		auth = "staff_enrol"
	}
	jsonRoute(h, "POST", prefix+"/login", "Authenticate "+audience+" and create a transport-bound session", "public", 200, service.Session{}, func(w http.ResponseWriter, r *http.Request, _ service.Principal, in service.LoginInput) (any, error) {
		if in.Client == "web" && !h.requestOriginAllowed(r) {
			return nil, &service.Fault{Status: 403, Code: "origin_required", Message: "approved browser origin required"}
		}
		if audience == "staff" && in.Client != "web" {
			return nil, service.Invalid("staff sessions require web transport")
		}
		out, e := s.Login(r.Context(), in, audience)
		if e != nil {
			return nil, e
		}
		return h.session(w, out, audience, in.Client), nil
	})
	jsonRoute(h, "POST", prefix+"/refresh", "Rotate credentials and detect refresh-token reuse", "public", 200, service.Session{}, func(w http.ResponseWriter, r *http.Request, _ service.Principal, in RefreshInput) (any, error) {
		token := in.RefreshToken
		if in.Client == "web" {
			if token != "" || !h.requestOriginAllowed(r) {
				return nil, &service.Fault{Status: 403, Code: "csrf_rejected", Message: "approved origin and cookie transport required"}
			}
			token = h.cookie(r, audience, "refresh")
		}
		if audience == "staff" && in.Client != "web" {
			return nil, service.Invalid("staff web transport required")
		}
		out, e := s.Refresh(r.Context(), token, r.Header.Get("X-CSRF-Token"), in.Client, audience)
		if e != nil {
			return nil, e
		}
		if (out.User.Role == "customer") != (audience == "customer") {
			return nil, &service.Fault{Status: 403, Code: "forbidden", Message: "session audience mismatch"}
		}
		return h.session(w, out, audience, in.Client), nil
	})
	h.get(prefix+"/csrf", "Restore browser CSRF material using the refresh cookie", "public", map[string]string{}, func(r *http.Request, _ service.Principal) (any, error) {
		if !h.requestOriginAllowed(r) {
			return nil, &service.Fault{Status: 403, Code: "origin_required", Message: "approved browser origin required"}
		}
		token, e := s.BrowserCSRF(r.Context(), h.cookie(r, audience, "refresh"), audience)
		return map[string]string{"csrf_token": token}, e
	})
	h.get(prefix+"/me", "Session account and MFA-enrolment status", auth, service.User{}, func(r *http.Request, p service.Principal) (any, error) { return s.Me(r.Context(), p.User.ID) })
	h.get(prefix+"/sessions", "Own devices and sessions", auth, []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.Sessions(r.Context(), p) })
	h.add(Route{Method: "DELETE", Path: prefix + "/sessions/{id}", Summary: "Revoke own device session", Auth: auth, Status: 204, Run: func(_ http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		return nil, s.RevokeSession(r.Context(), p, key(r))
	}})
	h.add(Route{Method: "POST", Path: prefix + "/logout", Summary: "Revoke current session", Auth: auth, Status: 204, Run: func(w http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		e := s.RevokeSession(r.Context(), p, p.SessionID)
		if e == nil && p.Client == "web" {
			h.cookies(w, audience, service.Session{}, true)
		}
		return nil, e
	}})
	jsonRoute(h, "POST", prefix+"/mfa/enrol", "Start expiring TOTP enrolment", auth, 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in PasswordProof) (any, error) {
		return s.MFAStart(r.Context(), p, in.Password, in.MFACode)
	})
	jsonRoute(h, "POST", prefix+"/mfa/confirm", "Confirm TOTP and return single-display recovery codes", auth, 200, []string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in CodeInput) (any, error) {
		return s.MFAConfirm(r.Context(), p, in.Code)
	})
	for _, kind := range []string{"password", "pin"} {
		kind := kind
		if audience == "staff" && kind == "pin" {
			continue
		}
		jsonRoute(h, "POST", prefix+"/"+kind, "Change "+kind+" and revoke sessions", auth, 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in CredentialInput) (any, error) {
			return accepted(), s.ChangeCredential(r.Context(), p, kind, in.Password, in.MFACode, in.NewValue)
		})
	}
}
func (h *Handler) caseRoutes(prefix, auth string) {
	s := h.Service
	h.get(prefix, "Owned or authorised support queue", auth, []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.Cases(r.Context(), p) })
	if auth == "customer" {
		jsonRoute(h, "POST", prefix, "Create a support, complaint or payment-dispute case", auth, 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.CaseInput) (any, error) {
			return created(s.CaseCreate(r.Context(), p, in))
		})
	}
	h.get(prefix+"/{id}/messages", "Authorised case conversation", auth, []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.CaseMessages(r.Context(), p, key(r)) })
	jsonRoute(h, "POST", prefix+"/{id}/messages", "Reply to an authorised case", auth, 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in CaseReplyInput) (any, error) {
		return accepted(), s.CaseReply(r.Context(), p, key(r), in.Message, in.Status)
	})
}
func (h *Handler) adminRoutes() {
	s := h.Service
	h.get("/v1/admin/overview", "Database-backed operational totals", "staff", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.Overview(r.Context(), p) })
	h.get("/v1/admin/customers", "Permissioned customer directory", "staff", []service.User{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.CustomerList(r.Context(), p, limit(r), r.URL.Query().Get("before"))
	})
	jsonRoute(h, "POST", "/v1/admin/staff", "Create staff credentials with mandatory MFA enrolment", "staff", 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in StaffInput) (any, error) {
		return nil, &service.Fault{Status: 410, Code: "use_staff_invitation", Message: "Use independently approved staff invitations; direct HTTP credential creation is retired"}
	})
	h.get("/v1/admin/kyc/cases", "Permissioned identity review queue", "staff", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.KYCList(r.Context(), p) })
	h.get("/v1/admin/policy", "Current versioned transaction policy", "staff", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		if p.User.Role != "admin" && p.User.Role != "finance" && p.User.Role != "compliance" {
			return nil, &service.Fault{Status: 403, Code: "forbidden", Message: "policy role required"}
		}
		return s.Policy(r.Context())
	})
	h.get("/v1/admin/proposals", "Independent approval queue", "staff", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.Proposals(r.Context(), p) })
	jsonRoute(h, "POST", "/v1/admin/proposals", "Propose a version-bound policy or customer operation", "staff", 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.ProposalInput) (any, error) {
		return created(s.Propose(r.Context(), p, in))
	})
	jsonRoute(h, "POST", "/v1/admin/proposals/{id}/decision", "Independent maker-checker decision", "staff", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in DecisionInput) (any, error) {
		return accepted(), s.Decide(r.Context(), p, key(r), in.Approve)
	})
	h.get("/v1/admin/payments/{id}", "Payment and immutable observation timeline", "staff", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.PaymentAdmin(r.Context(), p, key(r)) })
	h.add(Route{Method: "POST", Path: "/v1/admin/payments/{id}/requery", Summary: "Requeue only the original payment query", Auth: "staff", Status: 202, Response: map[string]string{}, Run: func(_ http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		return accepted(), s.RetryJob(r.Context(), p, key(r))
	}})
	h.get("/v1/admin/audit", "Immutable permissioned audit trail", "staff", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.Audit(r.Context(), p, limit(r), r.URL.Query().Get("before"))
	})
	jsonRoute(h, "POST", "/v1/admin/reconciliations", "Compare imported records without mutating balances", "staff", 201, map[string]any{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.ReconciliationInput) (any, error) {
		return s.Reconcile(r.Context(), p, in)
	})
	h.get("/v1/admin/reconciliations", "Reconciliation runs and exception totals", "staff", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.Reconciliations(r.Context(), p) })
}
