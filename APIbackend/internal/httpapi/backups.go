package httpapi

import (
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/backup"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/service"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
	"time"
)

func (h *Handler) backupRoutes() {
	s := h.Service
	prefix := "/v1/admin/backups"
	h.get(prefix+"/bootstrap", "Backup permissions and provisioned non-secret profiles", "staff", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.BackupBootstrap(r.Context(), p) })
	h.get(prefix+"/overview", "Protection coverage and actual evidence freshness", "staff", map[string]any{}, func(r *http.Request, p service.Principal) (any, error) { return s.BackupOverview(r.Context(), p) })
	h.get(prefix+"/resources/{kind}", "List sources destinations plans schedules retention runs and recovery points", "staff", service.AdminPage{}, func(r *http.Request, p service.Principal) (any, error) {
		n := 50
		if r.URL.Query().Get("limit") != "" {
			n, _ = strconv.Atoi(r.URL.Query().Get("limit"))
		}
		return s.BackupList(r.Context(), p, chi.URLParam(r, "kind"), service.AdminQuery{Limit: n, Search: r.URL.Query().Get("search"), Before: r.URL.Query().Get("before")})
	})
	h.get(prefix+"/resources/{kind}/{id}", "Read immutable configuration revision history", "staff", []backup.Resource{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.BackupVersions(r.Context(), p, chi.URLParam(r, "kind"), key(r))
	})
	jsonRoute(h, "POST", prefix+"/resources/{kind}", "Create a draft or submitted immutable backup configuration", "staff", 201, backup.Resource{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.BackupResourceInput) (any, error) {
		return s.BackupSave(r.Context(), p, chi.URLParam(r, "kind"), "", in)
	})
	jsonRoute(h, "POST", prefix+"/resources/{kind}/{id}/revisions", "Create a version checked backup configuration revision", "staff", 201, backup.Resource{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.BackupResourceInput) (any, error) {
		return s.BackupSave(r.Context(), p, chi.URLParam(r, "kind"), key(r), in)
	})
	jsonRoute(h, "POST", prefix+"/resources/{kind}/{id}/decision", "Submit or independently approve exact tested configuration digest", "staff", 200, backup.Resource{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.BackupDecision) (any, error) {
		return s.BackupDecide(r.Context(), p, chi.URLParam(r, "kind"), key(r), in)
	})
	jsonRoute(h, "POST", prefix+"/resources/{id}/pause", "Pause future backup operations without deleting stored recovery data", "staff", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.BackupDecision) (any, error) {
		return accepted(), s.BackupPause(r.Context(), p, key(r), in)
	})
	jsonRoute(h, "POST", prefix+"/schedule-preview", "Preview next five timezone-aware executions without scheduling work", "staff", 200, []time.Time{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in backup.ScheduleSpec) (any, error) {
		if _, err := s.BackupBootstrap(r.Context(), p); err != nil {
			return nil, err
		}
		out, err := backup.Next(in, s.Now(), 5)
		if err != nil {
			return nil, service.Invalid(err.Error())
		}
		return out, nil
	})
	jsonRoute(h, "POST", prefix+"/jobs", "Queue an immutable backup command; recovery requires independent approval", "staff", 201, map[string]any{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.BackupCommandInput) (any, error) {
		return s.BackupQueue(r.Context(), p, in)
	})
	jsonRoute(h, "POST", prefix+"/jobs/{id}/decision", "Independently approve or reject an exact recovery request", "staff", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in service.BackupDecision) (any, error) {
		return accepted(), s.BackupCommandDecide(r.Context(), p, key(r), in)
	})
	jsonRoute(h, "POST", prefix+"/jobs/{id}/cancel", "Cancel unclaimed command; leased nondestructive jobs may finish", "staff", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in AdminDecision) (any, error) {
		return accepted(), s.BackupCancel(r.Context(), p, key(r), in.Reason)
	})
	jsonRoute(h, "POST", prefix+"/points/{id}/inventory", "Audited access to encrypted recovery-point file and copy metadata", "staff", 200, backup.Result{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in AdminDecision) (any, error) {
		return s.BackupPoint(r.Context(), p, key(r), in.Reason)
	})
	jsonRoute(h, "POST", prefix+"/points/{id}/hold", "Pin a recovery point against automated expiry", "staff", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in AdminDecision) (any, error) {
		return accepted(), s.BackupHold(r.Context(), p, key(r), in.Reason)
	})
	h.get("/v1/backup-agent/policy", "Signed short-lived policy for independently enrolled backup agent", "backup_agent", backup.Signed{}, func(r *http.Request, p service.Principal) (any, error) {
		return s.BackupAgentPolicy(r.Context(), p.User.ID)
	})
	jsonRoute(h, "POST", "/v1/backup-agent/heartbeat", "Signed agent heartbeat with provisioned profile identity", "backup_agent", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in backup.AgentStatus) (any, error) {
		return accepted(), s.BackupAgentHeartbeat(r.Context(), p.User.ID, in)
	})
	jsonRoute(h, "POST", "/v1/backup-agent/results", "Append signed immutable result evidence; identical replay is safe", "backup_agent", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in backup.Signed) (any, error) {
		return accepted(), s.BackupAgentResult(r.Context(), p.User.ID, in)
	})
	jsonRoute(h, "POST", "/v1/backup-agent/commands/{id}/claim", "Claim current independently approved sensitive recovery command", "backup_agent", 200, map[string]string{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in struct{}) (any, error) {
		return accepted(), s.BackupAgentClaim(r.Context(), p.User.ID, key(r))
	})
	jsonRoute(h, "POST", "/v1/backup-agent/commands/{id}/point", "Read exact historical recovery evidence for a scoped agent command", "backup_agent", 200, backup.Result{}, func(_ http.ResponseWriter, r *http.Request, p service.Principal, in struct{}) (any, error) {
		return s.BackupAgentPoint(r.Context(), p.User.ID, key(r))
	})
}
