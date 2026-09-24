package httpapi

import (
	"encoding/json"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/service"
	"io"
	"mime"
	"net/http"
	"time"
)

type StateInput struct {
	Status  string `json:"status"`
	Version int64  `json:"version"`
}
type FavouriteInput struct {
	Favourite bool `json:"favourite"`
}
type FreezeInput struct {
	Password string `json:"password"`
	MFACode  string `json:"mfa_code,omitempty"`
	Frozen   bool   `json:"frozen"`
}
type VersionInput struct {
	Version int64 `json:"version"`
}
type RequestQuoteInput struct {
	ShareID string        `json:"share_id,omitempty"`
	Amount  service.Money `json:"amount_minor"`
}
type RequestActionInput struct {
	Action string `json:"action"`
}
type ContactVerifyInput struct {
	OldCode string `json:"old_code"`
	NewCode string `json:"new_code"`
}
type ClosureInput struct {
	Password string `json:"password"`
	MFACode  string `json:"mfa_code,omitempty"`
	Reason   string `json:"reason"`
}
type AvailabilityInput struct {
	ProductID string `json:"product_id"`
	Status    string `json:"status"`
}

func (h *Handler) parityRoutes() {
	s := h.Service
	h.get("/v1/me/capabilities", "Current customer's actual capabilities and limits", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.CustomerCapabilities(r.Context(), p) })
	h.get("/v1/me/controls", "Opt-in handle, freeze and personal spending limits", "customer", service.CustomerControls{}, func(r *http.Request, p service.Principal) (any, error) { return s.Controls(r.Context(), p) })
	jsonRoute(h, "PATCH", "/v1/me/controls", "Reauthenticate and update personal controls", "customer", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.ControlsInput) (any, error) {
		return accepted(), s.ChangeControls(r.Context(), p, in)
	})
	jsonRoute(h, "POST", "/v1/me/freeze", "Freeze or unfreeze own outgoing payments without removing compliance restrictions", "customer", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in FreezeInput) (any, error) {
		return accepted(), s.SecureAccount(r.Context(), p, in.Password, in.MFACode, in.Frozen)
	})
	h.add(Route{Method: "POST", Path: "/v1/auth/logout-all", Summary: "Revoke every own session", Auth: "customer", Status: 204, Run: func(w http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		e := s.SignOutAll(r.Context(), p)
		if e == nil {
			h.cookies(w, "customer", service.Session{}, true)
		}
		return nil, e
	}})
	h.get("/v1/auth/history", "Recent account access events", "customer", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.LoginHistory(r.Context(), p) })
	h.get("/v1/recipients/lookup", "Exact opt-in handle lookup; no email enumeration", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.FindRecipient(r.Context(), p, r.URL.Query().Get("handle"))
	})
	h.get("/v1/payments/lookup", "Recover original request by scoped idempotency key or quote", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.LookupPayment(r.Context(), p, r.URL.Query().Get("idempotency_key"), r.URL.Query().Get("quote_id"))
	})
	h.get("/v1/payments/{id}/timeline", "Authoritative original-payment timeline and hold state", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.PaymentTimeline(r.Context(), p, key(r))
	})
	h.get("/v1/payments/{id}/draft", "Prepare details for a new quote; never repeat execution", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.PaymentDraft(r.Context(), p, key(r)) })
	h.get("/v1/payments/{id}/annotation", "Own private category and note", "customer", service.AnnotationInput{}, func(r *http.Request, p service.Principal) (any, error) { return s.Annotation(r.Context(), p, key(r)) })
	jsonRoute(h, "PATCH", "/v1/payments/{id}/annotation", "Set own category, private note and insight exclusion", "customer", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.AnnotationInput) (any, error) {
		return accepted(), s.Annotate(r.Context(), p, key(r), in)
	})
	jsonRoute(h, "PATCH", "/v1/beneficiaries/{id}/favourite", "Favourite an owned beneficiary", "customer", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in FavouriteInput) (any, error) {
		return accepted(), s.FavourBeneficiary(r.Context(), p, key(r), in.Favourite)
	})
	h.get("/v1/saved-bills", "Private named household bill accounts", "customer", []service.SavedBill{}, func(r *http.Request, p service.Principal) (any, error) { return s.SavedBills(r.Context(), p) })
	jsonRoute(h, "POST", "/v1/saved-bills", "Create a named bill shortcut, not a payment mandate", "customer", 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.SavedBillInput) (any, error) {
		return created(s.SaveBill(r.Context(), p, "", in))
	})
	jsonRoute(h, "PATCH", "/v1/saved-bills/{id}", "Edit an owned saved bill", "customer", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.SavedBillInput) (any, error) {
		return created(s.SaveBill(r.Context(), p, key(r), in))
	})
	h.add(Route{Method: "DELETE", Path: "/v1/saved-bills/{id}", Summary: "Deactivate owned bill and cancel linked reminders", Auth: "customer", Status: 204, Run: func(_ http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		return nil, s.DeleteSavedBill(r.Context(), p, key(r))
	}})
	h.get("/v1/bills/token-archive", "Cursor-paginated completed bill records; values remain protected", "customer", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.TokenArchive(r.Context(), p, r.URL.Query().Get("before"))
	})
	h.get("/v1/bills/availability", "Timestamped bill service availability, unknown when stale", "customer", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.BillerHealth(r.Context(), p) })
	jsonRoute(h, "POST", "/v1/bills/watch", "Opt in or out of a biller-restored notification", "customer", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in struct {
		ProductID string `json:"product_id"`
		Watching  bool   `json:"watching"`
	}) (any, error) {
		return accepted(), s.WatchBiller(r.Context(), p, in.ProductID, in.Watching)
	})
	h.get("/v1/reminders", "Household payment reminders; never debit funds", "customer", []service.Reminder{}, func(r *http.Request, p service.Principal) (any, error) { return s.Reminders(r.Context(), p) })
	jsonRoute(h, "POST", "/v1/reminders", "Create a once, weekly or monthly reminder", "customer", 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.ReminderInput) (any, error) {
		return created(s.CreateReminder(r.Context(), p, in))
	})
	jsonRoute(h, "PATCH", "/v1/reminders/{id}", "Pause, resume or cancel versioned reminder", "customer", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in StateInput) (any, error) {
		return accepted(), s.ReminderState(r.Context(), p, key(r), in.Status, in.Version)
	})
	h.get("/v1/mandates", "Bounded internal scheduled-transfer authorisations", "customer", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.Mandates(r.Context(), p) })
	jsonRoute(h, "POST", "/v1/mandates", "Authorise explicit schedule limits with PIN and optional MFA", "customer", 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.MandateInput) (any, error) {
		return created(s.CreateMandate(r.Context(), p, in))
	})
	jsonRoute(h, "PATCH", "/v1/mandates/{id}", "Pause or cancel a mandate; resumption requires new approval", "customer", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in StateInput) (any, error) {
		return accepted(), s.MandateState(r.Context(), p, key(r), in.Status, in.Version)
	})
	h.get("/v1/mandates/{id}/history", "Each individual scheduled occurrence", "customer", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.MandateHistory(r.Context(), p, key(r))
	})
	h.get("/v1/money-requests", "Owned or assigned money requests", "customer", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.MoneyRequests(r.Context(), p, r.URL.Query().Get("before"))
	})
	jsonRoute(h, "POST", "/v1/money-requests", "Create an idempotent request with exact participant shares", "customer", 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.MoneyRequestInput) (any, error) {
		return created(s.RequestMoney(r.Context(), p, in, r.Header.Get("Idempotency-Key")))
	})
	h.get("/v1/money-requests/{id}", "Scoped request detail; other participants remain private", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.MoneyRequest(r.Context(), p, key(r)) })
	jsonRoute(h, "POST", "/v1/money-requests/{id}/action", "Cancel, decline or remind without debiting", "customer", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in RequestActionInput) (any, error) {
		return accepted(), s.MoneyRequestState(r.Context(), p, key(r), in.Action)
	})
	jsonRoute(h, "POST", "/v1/money-requests/{id}/quote", "Create a quote bound to an unpaid owned request share", "customer", 201, service.Quote{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in RequestQuoteInput) (any, error) {
		return s.RequestQuote(r.Context(), p, key(r), in.Amount)
	})
	h.get("/v1/insights", "Complete monthly aggregates with explicit exclusions", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.Insights(r.Context(), p, r.URL.Query().Get("month"))
	})
	h.get("/v1/insights/categories", "Supported budget categories", "customer", []string{}, func(*http.Request, service.Principal) (any, error) { return service.SpendingCategories, nil })
	jsonRoute(h, "POST", "/v1/budgets", "Set a monthly category budget", "customer", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.BudgetInput) (any, error) {
		return accepted(), s.SetBudget(r.Context(), p, in)
	})
	h.add(Route{Method: "DELETE", Path: "/v1/budgets/{id}", Summary: "Remove own category budget for month", Auth: "customer", Status: 204, Run: func(_ http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		return nil, s.RemoveBudget(r.Context(), p, r.URL.Query().Get("month"), key(r))
	}})
	h.get("/v1/search", "Search owned financial records", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.Search(r.Context(), p, r.URL.Query().Get("q"))
	})
	h.get("/v1/funding/account", "Actual provisioning state and approved funding details", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.FundingAccount(r.Context(), p) })
	h.add(Route{Method: "POST", Path: "/v1/funding/account", Summary: "Request an owned partner funding account", Auth: "customer", Status: 202, Response: map[string]any{}, Run: func(_ http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		return s.RequestFundingAccount(r.Context(), p)
	}})
	h.get("/v1/identity/draft", "Resume encrypted private identity draft", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.IdentityDraft(r.Context(), p) })
	jsonRoute(h, "PATCH", "/v1/identity/draft", "Save an optimistic-versioned identity draft", "customer", 200, map[string]any{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.IdentityInput) (any, error) {
		return s.SaveIdentityDraft(r.Context(), p, in)
	})
	jsonRoute(h, "POST", "/v1/identity/submit", "Submit owned checked documents for independent review", "customer", 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in VersionInput) (any, error) {
		return created(s.SubmitIdentity(r.Context(), p, in.Version))
	})
	h.get("/v1/uploads", "Own private document metadata", "customer", []service.Upload{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.Uploads(r.Context(), p, r.URL.Query().Get("purpose"), r.URL.Query().Get("case_id"))
	})
	h.add(Route{Method: "POST", Path: "/v1/uploads", Summary: "Upload JPEG PNG PDF via checked encrypted private storage (multipart/form-data)", Auth: "customer", Status: 201, Response: service.Upload{}, Run: func(w http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
		if e := r.ParseMultipartForm(6 << 20); e != nil {
			return nil, service.Invalid("one file up to five MiB is required")
		}
		defer r.MultipartForm.RemoveAll()
		if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
			return nil, service.Invalid("provide exactly one file")
		}
		file, header, e := r.FormFile("file")
		if e != nil {
			return nil, service.Invalid("file required")
		}
		defer file.Close()
		raw, e := io.ReadAll(io.LimitReader(file, (5<<20)+1))
		if e != nil {
			return nil, e
		}
		return s.UploadDocument(r.Context(), p, r.FormValue("purpose"), r.FormValue("case_id"), header.Filename, raw)
	}})
	for _, v := range []struct{ path, auth string }{{"/v1/uploads/{id}/download", "customer"}, {"/v1/admin/uploads/{id}/download", "staff"}} {
		h.add(Route{Method: "GET", Path: v.path, Summary: "Audited attachment-only private document download", Auth: v.auth, Status: 200, Run: func(w http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
			if p.Audience == "staff" {
				return nil, &service.Fault{Status: 410, Code: "reasoned_access_required", Message: "use the reasoned staff console evidence endpoint"}
			}
			meta, data, e := s.DownloadDocument(r.Context(), p, key(r))
			if e != nil {
				return nil, e
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": meta.Name}))
			w.WriteHeader(200)
			_, e = w.Write(data)
			return rawWritten, e
		}})
	}
	h.add(Route{Method: "DELETE", Path: "/v1/uploads/{id}", Summary: "Delete unsubmitted owned document; submitted evidence retained", Auth: "customer", Status: 204, Run: func(_ http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		return nil, s.DeleteUpload(r.Context(), p, key(r))
	}})
	jsonRoute(h, "POST", "/v1/me/email-change", "Start dual-mailbox change with reauthentication", "customer", 202, map[string]any{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.ContactChangeInput) (any, error) {
		return s.BeginEmailChange(r.Context(), p, in)
	})
	jsonRoute(h, "POST", "/v1/me/email-change/{id}/verify", "Consume both codes and revoke sessions", "customer", 200, map[string]string{}, func(w http.ResponseWriter, r *http.Request, p service.Principal, in ContactVerifyInput) (any, error) {
		e := s.FinishEmailChange(r.Context(), p, key(r), in.OldCode, in.NewCode)
		if e == nil {
			h.cookies(w, "customer", service.Session{}, true)
		}
		return accepted(), e
	})
	h.get("/v1/me/closure", "Account closure blockers and retention explanation", "customer", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.ClosureEligibility(r.Context(), p) })
	jsonRoute(h, "POST", "/v1/me/closure", "Close only a settled unrestricted zero-balance account", "customer", 200, map[string]string{}, func(w http.ResponseWriter, r *http.Request, p service.Principal, in ClosureInput) (any, error) {
		e := s.CloseAccount(r.Context(), p, in.Password, in.MFACode, in.Reason)
		if e == nil {
			h.cookies(w, "customer", service.Session{}, true)
		}
		return accepted(), e
	})
	jsonRoute(h, "POST", "/v1/me/export", "Reauthenticated scoped profile and preferences export", "customer", 200, map[string]any{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in PasswordProof) (any, error) {
		return s.PrivacyExport(r.Context(), p, in.Password, in.MFACode)
	})
	h.add(Route{Method: "POST", Path: "/v1/support/cases/{id}/escalate", Summary: "Escalate own unresolved case", Auth: "customer", Status: 200, Response: map[string]string{}, Run: func(_ http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		return accepted(), s.EscalateCase(r.Context(), p, key(r))
	}})
	h.get("/v1/support/cases/{id}/timeline", "Own case events", "customer", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.CaseTimeline(r.Context(), p, key(r)) })
	h.get("/v1/admin/identity/{id}", "Permissioned digital identity evidence", "staff", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return nil, &service.Fault{Status: 410, Code: "reasoned_access_required", Message: "use POST /v1/admin/console/identity/{id} with access reason"}
	})
	jsonRoute(h, "POST", "/v1/admin/bills/availability", "Audited observed biller state; never a success guarantee", "staff", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in AvailabilityInput) (any, error) {
		return accepted(), s.ObserveBiller(r.Context(), p, in.ProductID, in.Status)
	})
	h.add(Route{Method: "POST", Path: "/internal/funding/observations", Summary: "Signed funding hint independently verified with partner", Auth: "internal", Status: 202, Run: func(_ http.ResponseWriter, r *http.Request, _ service.Principal) (any, error) {
		raw, e := readSigned(r, h.Config.FundingKey)
		if e != nil {
			return nil, e
		}
		var in struct {
			Reference string `json:"reference"`
		}
		if json.Unmarshal(raw, &in) != nil {
			return nil, service.Invalid("invalid reference")
		}
		return created(s.VerifyIncomingFunding(r.Context(), in.Reference))
	}})
	h.passkeyRoutes()
}
func (h *Handler) passkeyRoutes() {
	s := h.Service
	for _, register := range []bool{true, false} {
		auth, path := "customer", "/v1/passkeys/register"
		if !register {
			auth = "public"
			path = "/v1/auth/passkeys"
		}
		jsonRoute(h, "POST", path+"/begin", "Start origin-bound passkey ceremony", ""+auth, 200, map[string]any{}, func(w http.ResponseWriter, r *http.Request, p service.Principal, in PasswordProof) (any, error) {
			if !h.originAllowed(r.Header.Get("Origin")) {
				return nil, &service.Fault{Status: 403, Code: "origin_required", Message: "approved browser origin required"}
			}
			binding := security.Random("", 32)
			result, e := s.PasskeyBegin(r.Context(), p, r.Header.Get("Origin"), binding, in.Password, in.MFACode, register)
			if e != nil {
				return nil, e
			}
			http.SetCookie(w, &http.Cookie{Name: h.cookieName("customer", "passkey"), Value: binding, Path: "/", HttpOnly: true, Secure: h.Config.Environment != "local", SameSite: http.SameSiteStrictMode, MaxAge: 180})
			return result, nil
		})
		jsonRoute(h, "POST", path+"/finish", "Verify and consume passkey ceremony", ""+auth, 200, map[string]any{}, func(w http.ResponseWriter, r *http.Request, p service.Principal, in service.PasskeyFinishInput) (any, error) {
			if !h.originAllowed(r.Header.Get("Origin")) {
				return nil, &service.Fault{Status: 403, Code: "origin_required", Message: "approved browser origin required"}
			}
			out, e := s.PasskeyFinish(r.Context(), p, h.cookie(r, "customer", "passkey"), in, register)
			if e != nil {
				return nil, e
			}
			http.SetCookie(w, &http.Cookie{Name: h.cookieName("customer", "passkey"), Path: "/", HttpOnly: true, Secure: h.Config.Environment != "local", SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
			if register {
				return accepted(), nil
			}
			return h.session(w, out, "customer", "web"), nil
		})
	}
	h.get("/v1/passkeys", "Own passkey devices without public key material", "customer", []map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.PasskeyList(r.Context(), p) })
	jsonRoute(h, "POST", "/v1/passkeys/{id}/revoke", "Reauthenticate and revoke passkey and other sessions", "customer", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in PasswordProof) (any, error) {
		return accepted(), s.RevokePasskey(r.Context(), p, key(r), in.Password, in.MFACode)
	})
}
