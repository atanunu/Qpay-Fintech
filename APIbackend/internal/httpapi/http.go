// Package httpapi owns transport policy, not financial decisions.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/security"
	"github.com/atanunu/Qpay-Fintech/APIbackend/internal/service"
	"github.com/go-chi/chi/v5"
)

type Config struct {
	Environment  string
	Origins      []string
	StaffOrigins []string
	Logger       *slog.Logger
	PolicyKey    []byte
	FundingKey   []byte
}
type Handler struct {
	Service *service.Service
	Config  Config
	Router  *chi.Mux
	Routes  []Route
}
type Route struct {
	Method, Path, Summary, Auth string
	Status                      int
	Request, Response           any
	Run                         func(http.ResponseWriter, *http.Request, service.Principal) (any, error)
}
type requestKey struct{}

func New(s *service.Service, c Config) *Handler {
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	h := &Handler{Service: s, Config: c, Router: chi.NewRouter()}
	h.register()
	return h
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := security.Random("req_", 18)
	r = r.WithContext(context.WithValue(r.Context(), requestKey{}, id))
	w.Header().Set("X-Request-ID", id)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	defer func() {
		if recover() != nil {
			h.Config.Logger.Error("request_panic", "request_id", id)
			h.fail(w, r, errors.New("internal error"))
		}
	}()
	if origin := r.Header.Get("Origin"); origin != "" {
		if !h.requestOriginAllowed(r) {
			h.fail(w, r, &service.Fault{Status: 403, Code: "origin_denied", Message: "browser origin not permitted"})
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID,Retry-After")
	}
	if r.Method == "OPTIONS" {
		if !h.requestOriginAllowed(r) {
			h.fail(w, r, service.Invalid("approved origin required"))
			return
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-CSRF-Token,Idempotency-Key")
		w.WriteHeader(204)
		return
	}
	h.Router.ServeHTTP(w, r)
}
func (h *Handler) originAllowed(origin string) bool {
	for _, v := range h.Config.Origins {
		if origin == v {
			return true
		}
	}
	return false
}

// Staff and customer browser origins are separately configured. Local tests may share the legacy allowlist.
func (h *Handler) requestOriginAllowed(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/v1/admin/") {
		origins := h.Config.StaffOrigins
		if len(origins) == 0 && h.Config.Environment == "local" {
			origins = h.Config.Origins
		}
		for _, origin := range origins {
			if origin == r.Header.Get("Origin") {
				return true
			}
		}
		return false
	}
	return h.originAllowed(r.Header.Get("Origin"))
}
func (h *Handler) cookieName(audience, kind string) string {
	prefix := ""
	if h.Config.Environment != "local" {
		prefix = "__Host-"
	}
	return prefix + "qpf_" + audience + "_" + kind
}
func (h *Handler) cookie(r *http.Request, audience, kind string) string {
	v, e := r.Cookie(h.cookieName(audience, kind))
	if e != nil {
		return ""
	}
	return v.Value
}
func (h *Handler) cookies(w http.ResponseWriter, audience string, s service.Session, clear bool) {
	for _, v := range []struct {
		name, value string
		expiry      time.Time
	}{{"access", s.AccessToken, s.ExpiresAt}, {"refresh", s.RefreshToken, s.RefreshExpiresAt}} {
		cookie := &http.Cookie{Name: h.cookieName(audience, v.name), Value: v.value, Path: "/", HttpOnly: true, Secure: h.Config.Environment != "local", SameSite: http.SameSiteStrictMode, Expires: v.expiry}
		if clear {
			cookie.Value = ""
			cookie.MaxAge = -1
			cookie.Expires = time.Unix(1, 0)
		}
		http.SetCookie(w, cookie)
	}
}
func (h *Handler) session(w http.ResponseWriter, s service.Session, audience, client string) service.Session {
	if client == "web" {
		h.cookies(w, audience, s, false)
		s.AccessToken = ""
		s.RefreshToken = ""
	}
	return s
}
func (h *Handler) authenticate(r *http.Request, auth string) (service.Principal, error) {
	var p service.Principal
	if auth == "backup_agent" {
		if r.Header.Get("Origin") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			return p, &service.Fault{Status: 403, Code: "agent_transport_denied", Message: "backup agents cannot use browser or customer credentials"}
		}
		body, e := io.ReadAll(io.LimitReader(r.Body, (8<<20)+1))
		if e != nil || len(body) > 8<<20 {
			return p, service.Invalid("agent body exceeds 8 MiB")
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		id, e := h.Service.BackupAgentAuthenticate(r.Context(), r.Header.Get("X-Backup-Agent"), r.Method, r.URL.EscapedPath(), body)
		if e != nil {
			return p, e
		}
		p.User.ID = id
		p.Audience = "backup_agent"
		return p, nil
	}
	if auth == "public" || auth == "internal" {
		return p, nil
	}
	audience := "customer"
	if strings.HasPrefix(auth, "staff") {
		audience = "staff"
	}
	token := h.cookie(r, audience, "access")
	fromCookie := token != ""
	bearer := r.Header.Get("Authorization")
	if bearer != "" {
		if fromCookie || !strings.HasPrefix(bearer, "Bearer ") {
			return p, &service.Fault{Status: 401, Code: "unauthorized", Message: "use exactly one authentication mechanism"}
		}
		token = strings.TrimPrefix(bearer, "Bearer ")
	}
	p, e := h.Service.Authenticate(r.Context(), token)
	if e != nil {
		return p, e
	}
	if p.Audience != audience || (fromCookie && p.Client != "web") || (!fromCookie && p.Client != "mobile") {
		return p, &service.Fault{Status: 403, Code: "forbidden", Message: "session transport or audience mismatch"}
	}
	if fromCookie && r.Method != "GET" && r.Method != "HEAD" {
		if !h.requestOriginAllowed(r) || !security.Equal(p.CSRFHash, security.Digest(r.Header.Get("X-CSRF-Token"))) {
			return p, &service.Fault{Status: 403, Code: "csrf_rejected", Message: "valid origin and CSRF token required"}
		}
	}
	if auth == "staff" && (!p.MFAReady || !p.User.MFA || p.User.Status != "active") {
		return p, &service.Fault{Status: 403, Code: "staff_mfa_required", Message: "active staff with enrolled MFA required"}
	}
	if auth == "staff" && r.Method != "GET" && r.Method != "HEAD" {
		if e := h.Service.RequireStaffElevation(r.Context(), p); e != nil {
			return p, e
		}
	}
	return p, nil
}
func (h *Handler) add(route Route) {
	h.Routes = append(h.Routes, route)
	h.Router.MethodFunc(route.Method, route.Path, func(w http.ResponseWriter, r *http.Request) {
		p, e := h.authenticate(r, route.Auth)
		if e != nil {
			h.fail(w, r, e)
			return
		}
		if route.Auth != "internal" && route.Path != "/health/live" && route.Path != "/health/ready" && route.Path != "/openapi.json" {
			key := p.User.ID
			if key == "" {
				host, _, _ := net.SplitHostPort(r.RemoteAddr)
				key = "ip:" + host
			}
			if e = h.Service.Rate(r.Context(), "http:"+key, 180, time.Minute); e != nil {
				h.fail(w, r, e)
				return
			}
		}
		result, e := route.Run(w, r, p)
		if e != nil {
			h.fail(w, r, e)
			return
		}
		if result == rawWritten {
			return
		}
		if route.Status == 204 {
			w.WriteHeader(204)
			return
		}
		h.json(w, route.Status, map[string]any{"data": result, "request_id": r.Context().Value(requestKey{})})
	})
}

const rawWritten = "__response_written__"

func (h *Handler) json(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, e error) {
	status, code, message := service.FaultStatus(e)
	if status == 429 {
		w.Header().Set("Retry-After", "60")
	}
	if status >= 500 {
		h.Config.Logger.Error("request_failed", "request_id", r.Context().Value(requestKey{}), "error_type", fmt.Sprintf("%T", e))
	}
	h.json(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}, "request_id": r.Context().Value(requestKey{})})
}
func bind(r *http.Request, target any) error {
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "application/json" {
		return &service.Fault{Status: 415, Code: "content_type", Message: "application/json required"}
	}
	limit := 1 << 20
	if strings.HasPrefix(r.URL.Path, "/v1/backup-agent/") {
		limit = 8 << 20
	}
	raw, e := io.ReadAll(io.LimitReader(r.Body, int64(limit)+1))
	if e != nil {
		return service.Invalid("could not read request")
	}
	if len(raw) > limit {
		return &service.Fault{Status: 413, Code: "body_too_large", Message: "body exceeds one MiB"}
	}
	if e = uniqueJSON(raw); e != nil {
		return service.Invalid(e.Error())
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(target); e != nil {
		return service.Invalid("invalid JSON fields or values")
	}
	return nil
}
func uniqueJSON(raw []byte) error {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '{' {
		return errors.New("JSON object required")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 24 {
			return errors.New("JSON nesting limit exceeded")
		}
		token, e := d.Token()
		if e != nil {
			return errors.New("invalid JSON")
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			keys := map[string]bool{}
			for d.More() {
				t, e := d.Token()
				if e != nil {
					return errors.New("invalid JSON")
				}
				key, ok := t.(string)
				if !ok || keys[key] {
					return errors.New("duplicate JSON field")
				}
				keys[key] = true
				if e = value(depth + 1); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e = value(depth + 1); e != nil {
					return e
				}
			}
		default:
			return errors.New("invalid JSON delimiter")
		}
		_, e = d.Token()
		return e
	}
	if e := value(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
func jsonRoute[T any](h *Handler, method, path, summary, auth string, status int, response any, run func(http.ResponseWriter, *http.Request, service.Principal, T) (any, error)) {
	var model T
	h.add(Route{Method: method, Path: path, Summary: summary, Auth: auth, Status: status, Request: model, Response: response, Run: func(w http.ResponseWriter, r *http.Request, p service.Principal) (any, error) {
		var in T
		if e := bind(r, &in); e != nil {
			return nil, e
		}
		return run(w, r, p, in)
	}})
}
func limit(r *http.Request) int {
	if r.URL.Query().Get("limit") == "" {
		return 50
	}
	n, e := strconv.Atoi(r.URL.Query().Get("limit"))
	if e != nil {
		return -1
	}
	return n
}
func key(r *http.Request) string { return chi.URLParam(r, "id") }
func (h *Handler) get(path, summary, auth string, response any, run func(*http.Request, service.Principal) (any, error)) {
	h.add(Route{Method: "GET", Path: path, Summary: summary, Auth: auth, Status: 200, Response: response, Run: func(_ http.ResponseWriter, r *http.Request, p service.Principal) (any, error) { return run(r, p) }})
}

// schema uses the actual Go transport types. Money never becomes a JSON number.
func schema(t reflect.Type) map[string]any {
	if t == nil {
		return map[string]any{}
	}
	if t == reflect.TypeOf(json.RawMessage{}) {
		return map[string]any{"type": "object", "description": "Typed JSON object; signed envelopes preserve the original serialized payload bytes"}
	}
	if t == reflect.TypeOf(service.Money(0)) {
		return map[string]any{"type": "string", "pattern": "^(0|[1-9][0-9]{0,15})$", "description": "Integer NGN minor units; maximum 9000000000000000"}
	}
	if t == reflect.TypeOf(time.Time{}) {
		return map[string]any{"type": "string", "format": "date-time"}
	}
	switch t.Kind() {
	case reflect.Pointer:
		return schema(t.Elem())
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64, reflect.Int32:
		return map[string]any{"type": "integer"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": schema(t.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": schema(t.Elem())}
	case reflect.Struct:
		props := map[string]any{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			tag := strings.Split(f.Tag.Get("json"), ",")
			name := tag[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			props[name] = schema(f.Type)
			if !strings.Contains(f.Tag.Get("json"), "omitempty") {
				required = append(required, name)
			}
		}
		m := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
		if len(required) > 0 {
			m["required"] = required
		}
		return m
	}
	return map[string]any{}
}
func (h *Handler) OpenAPI() map[string]any {
	paths := map[string]any{}
	for _, r := range h.Routes {
		if strings.HasPrefix(r.Path, "/internal/") {
			continue
		}
		operation := map[string]any{"summary": r.Summary, "operationId": strings.ToLower(r.Method) + strings.NewReplacer("/", "_", "{", "", "}", "", "-", "_").Replace(r.Path), "responses": map[string]any{strconv.Itoa(r.Status): map[string]any{"description": "Successful response", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "properties": map[string]any{"data": schema(reflect.TypeOf(r.Response)), "request_id": map[string]any{"type": "string"}}}}}}, "default": map[string]any{"description": "Stable error envelope; no financial success is implied", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "required": []string{"error", "request_id"}, "properties": map[string]any{"error": map[string]any{"type": "object", "required": []string{"code", "message"}, "properties": map[string]any{"code": map[string]any{"type": "string"}, "message": map[string]any{"type": "string"}}}, "request_id": map[string]any{"type": "string"}}}}}}}}
		if r.Request != nil {
			operation["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": schema(reflect.TypeOf(r.Request))}}}
		}
		parameters := []any{}
		if strings.Contains(r.Path, "{id}") {
			parameters = append(parameters, map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
		}
		if (r.Path == "/v1/payments" || r.Path == "/v1/money-requests") && r.Method == "POST" {
			parameters = append(parameters, map[string]any{"name": "Idempotency-Key", "in": "header", "required": true, "schema": map[string]any{"type": "string", "minLength": 16, "maxLength": 100}})
		}
		queryFields := map[string][]string{
			"/v1/me/controls": {}, "/v1/recipients/lookup": {"handle"},
			"/v1/payments/lookup": {"idempotency_key", "quote_id"}, "/v1/payments": {"limit", "before"},
			"/v1/notifications": {"limit", "before"}, "/v1/wallet/entries": {"limit", "before"},
			"/v1/statements": {"from", "to", "format"}, "/v1/insights": {"month"}, "/v1/search": {"q"},
			"/v1/money-requests": {"before"}, "/v1/bills/token-archive": {"before"},
			"/v1/uploads": {"purpose", "case_id"}, "/v1/budgets/{id}": {"month"},
		}
		if r.Method == "GET" || r.Method == "DELETE" {
			for _, name := range queryFields[r.Path] {
				parameters = append(parameters, map[string]any{"name": name, "in": "query", "required": false, "schema": map[string]any{"type": "string"}})
			}
		}
		if r.Path == "/v1/uploads" && r.Method == "POST" {
			operation["requestBody"] = map[string]any{"required": true, "content": map[string]any{"multipart/form-data": map[string]any{"schema": map[string]any{"type": "object", "required": []string{"file", "purpose"}, "properties": map[string]any{"file": map[string]any{"type": "string", "format": "binary", "description": "JPEG, PNG or PDF; at most 5 MiB"}, "purpose": map[string]any{"type": "string", "enum": []string{"kyc", "support"}}, "case_id": map[string]any{"type": "string"}}}}}}
		}
		if len(parameters) > 0 {
			operation["parameters"] = parameters
		}
		if r.Auth != "public" {
			security := []any{map[string]any{"mobileBearer": []string{}}, map[string]any{"customerCookie": []string{}}}
			if strings.HasPrefix(r.Auth, "staff") {
				security = []any{map[string]any{"staffCookie": []string{}}}
			}
			if r.Auth == "backup_agent" {
				security = []any{map[string]any{"backupAgentSignature": []string{}}}
			}
			operation["security"] = security
			operation["description"] = "Cookie writes also require an approved Origin and X-CSRF-Token. Staff operations require MFA and server-side role permissions."
			if r.Auth == "backup_agent" {
				operation["description"] = "Pinned Ed25519 agent signature binds method, exact path, timestamp, unique nonce and SHA-256 of the request body. Browser Origin, cookies and bearer tokens are rejected."
			}
		}
		path, _ := paths[r.Path].(map[string]any)
		if path == nil {
			path = map[string]any{}
			paths[r.Path] = path
		}
		path[strings.ToLower(r.Method)] = operation
	}
	return enrichOpenAPI(map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Qpay-Fintech API", "version": "0.6.0", "description": "Persistent core API. Local execution is synthetic. Provider, regulatory, security and production acceptance remain separate release gates."}, "paths": paths, "components": map[string]any{"securitySchemes": map[string]any{"backupAgentSignature": map[string]any{"type": "apiKey", "in": "header", "name": "X-Backup-Agent", "description": "Signed, short-lived, single-use agent request envelope"}, "mobileBearer": map[string]any{"type": "http", "scheme": "bearer", "description": "Opaque mobile session token"}, "customerCookie": map[string]any{"type": "apiKey", "in": "cookie", "name": h.cookieName("customer", "access")}, "staffCookie": map[string]any{"type": "apiKey", "in": "cookie", "name": h.cookieName("staff", "access")}}}})
}
