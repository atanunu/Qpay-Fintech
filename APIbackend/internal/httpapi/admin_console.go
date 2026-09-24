package httpapi

import (
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/service"
	"github.com/go-chi/chi/v5"
	"net/http"
)

type AdminDecision struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}
type BillerObservation struct {
	ProductID string `json:"product_id"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}
type AdminExportInput struct {
	Resource string `json:"resource"`
	Search   string `json:"search"`
	Reason   string `json:"reason"`
}

func (h *Handler) consoleRoutes() {
	s := h.Service
	jsonRoute(h, "POST", "/v1/admin/console/biller-observation", "Audited time-bounded biller observation", "staff", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in BillerObservation) (any, error) {
		return accepted(), s.ObserveBillerReasoned(r.Context(), p, in.ProductID, in.Status, in.Reason)
	})
	jsonRoute(h, "POST", "/v1/admin/console/payments/{id}/fulfilment", "Reasoned access to delivered bill value", "staff", 200, map[string]any{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in AdminDecision) (any, error) {
		return s.AdminFulfilment(r.Context(), p, key(r), in.Reason)
	})

	h.get("/v1/admin/console/payment-control", "Current emergency containment policy version", "staff", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.AdminPaymentControl(r.Context(), p) })
	jsonRoute(h, "POST", "/v1/admin/console/identity/{id}", "Reasoned audited identity case access", "staff", 200, map[string]any{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in AdminDecision) (any, error) {
		if len(in.Reason) < 8 || len(in.Reason) > 1000 {
			return nil, service.Invalid("access reason of 8 to 1000 characters required")
		}
		if _, e := s.AdminCommand(r.Context(), p, service.AdminCommandInput{Action: "note", Resource: "kyc", Target: key(r), Reason: "Identity evidence access: " + in.Reason}); e != nil {
			return nil, e
		}
		return s.IdentityCase(r.Context(), p, key(r))
	})
	jsonRoute(h, "POST", "/v1/admin/console/proposals/{id}/decision", "Independent financial approval with decision evidence", "staff", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in AdminDecision) (any, error) {
		if in.Decision != "approve" && in.Decision != "reject" {
			return nil, service.Invalid("approve or reject required")
		}
		return accepted(), s.DecideReasoned(r.Context(), p, key(r), in.Decision == "approve", in.Reason)
	})
	jsonRoute(h, "POST", "/v1/admin/console/payments/{id}/requery", "Reasoned original-reference query recovery", "staff", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in AdminDecision) (any, error) {
		return accepted(), s.RetryJobReasoned(r.Context(), p, key(r), in.Reason)
	})
	jsonRoute(h, "POST", "/v1/admin/recovery/accept", "Consume independently approved staff recovery; mandatory MFA re-enrolment", "public", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, _ service.Principal, in service.StaffInvitationAccept) (any, error) {
		if !h.requestOriginAllowed(r) {
			return nil, &service.Fault{Status: 403, Code: "origin_required", Message: "approved staff origin required"}
		}
		return accepted(), s.AcceptStaffRecovery(r.Context(), in)
	})
	h.get("/v1/admin/console/bootstrap", "Current staff identity, allowed resources, actions and acceptance gates", "staff", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.AdminBootstrap(r.Context(), p) })
	h.get("/v1/admin/console/metrics", "Authoritative operational snapshot with separately permissioned ledger totals", "staff", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.AdminMetrics(r.Context(), p) })
	h.get("/v1/admin/console/records/{resource}", "Paginated allowlisted and redacted operational records", "staff", service.AdminPage{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.AdminList(r.Context(), p, chi.URLParam(r, "resource"), service.AdminQuery{Limit: limit(r), Search: r.URL.Query().Get("search"), Before: r.URL.Query().Get("before")})
	})
	h.get("/v1/admin/console/records/{resource}/{id}", "Audited record detail and permissioned related evidence", "staff", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.AdminDetail(r.Context(), p, chi.URLParam(r, "resource"), key(r))
	})
	jsonRoute(h, "POST", "/v1/admin/console/actions", "Reasoned and version-checked operational action", "staff", 200, map[string]any{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.AdminCommandInput) (any, error) {
		return s.AdminCommand(r.Context(), p, in)
	})
	jsonRoute(h, "POST", "/v1/admin/auth/elevate", "Reauthenticate staff for ten minutes of sensitive operations", "staff_enrol", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in PasswordProof) (any, error) {
		return accepted(), s.StaffElevate(r.Context(), p, in.Password, in.MFACode)
	})
	jsonRoute(h, "POST", "/v1/admin/console/invitations", "Propose invitation; another administrator must approve", "staff", 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.StaffInvitationInput) (any, error) {
		return created(s.InviteStaff(r.Context(), p, in))
	})
	jsonRoute(h, "POST", "/v1/admin/console/invitations/{id}/decision", "Approve, reject or revoke an invitation; token displayed once", "staff", 200, map[string]any{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in AdminDecision) (any, error) {
		return s.DecideInvitation(r.Context(), p, key(r), in.Decision, in.Reason)
	})
	jsonRoute(h, "POST", "/v1/admin/invitations/accept", "Consume independently approved invitation; MFA enrolment remains mandatory", "public", 201, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, _ service.Principal, in service.StaffInvitationAccept) (any, error) {
		if !h.requestOriginAllowed(r) {
			return nil, &service.Fault{Status: 403, Code: "origin_required", Message: "approved staff browser origin required"}
		}
		return created(s.AcceptStaffInvitation(r.Context(), in))
	})
	jsonRoute(h, "POST", "/v1/admin/console/exports", "Audited bounded CSV export; no truncated financial reports", "staff", 200, nil, func(w http.ResponseWriter, r *http.Request, p service.Principal, in AdminExportInput) (any, error) {
		text, e := s.AdminExport(r.Context(), p, in.Resource, in.Search, in.Reason)
		if e != nil {
			return nil, e
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="qpay-admin-export.csv"`)
		w.WriteHeader(200)
		_, e = w.Write([]byte(text))
		return rawWritten, e
	})
	h.get("/v1/admin/console/catalogue", "Normalised catalogue from configured adapter; unavailable is not synthetic success", "staff", []service.Product{}, func(r *http.Request, p service.Principal) (any, error) {
		if p.User.Role != "admin" && p.User.Role != "platform" && p.User.Role != "finance" {
			return nil, &service.Fault{Status: 403, Code: "forbidden", Message: "catalogue role required"}
		}
		return s.Products(r.Context())
	})
	jsonRoute(h, "POST", "/v1/admin/console/documents/{id}", "Audited private evidence download with reason", "staff", 200, nil, func(w http.ResponseWriter, r *http.Request, p service.Principal, in AdminDecision) (any, error) {
		if len(in.Reason) < 8 || len(in.Reason) > 1000 {
			return nil, service.Invalid("access reason of 8 to 1000 characters required")
		}
		if e := s.RecordEvidenceReason(r.Context(), p, key(r), in.Reason); e != nil {
			return nil, e
		}
		upload, data, e := s.DownloadDocument(r.Context(), p, key(r))
		if e != nil {
			return nil, e
		}
		w.Header().Set("Content-Type", upload.Mime)
		w.Header().Set("Content-Disposition", `attachment; filename="private-evidence"`)
		w.WriteHeader(200)
		_, e = w.Write(data)
		return rawWritten, e
	})
}
