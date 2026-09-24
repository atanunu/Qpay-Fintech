package httpapi

import (
	"strings"
)

// enrichOpenAPI supplies concrete shapes for map-backed views, query contracts,
// and non-JSON responses. The routes and typed request/financial models remain
// generated from the same transport registry used by the running server.
func enrichOpenAPI(spec map[string]any) map[string]any {
	text := func() map[string]any { return map[string]any{"type": "string"} }
	boolean := func() map[string]any { return map[string]any{"type": "boolean"} }
	integer := func() map[string]any { return map[string]any{"type": "integer"} }
	timestamp := func() map[string]any { return map[string]any{"type": "string", "format": "date-time"} }
	money := func() map[string]any {
		return map[string]any{"type": "string", "pattern": "^(0|[1-9][0-9]{0,15})$", "description": "Exact non-negative NGN minor units, at most 9000000000000000."}
	}
	signed := func() map[string]any { return map[string]any{"type": "string", "pattern": "^-?(0|[1-9][0-9]*)$"} }
	object := func(p map[string]any, required ...string) map[string]any {
		o := map[string]any{"type": "object", "properties": p, "additionalProperties": false}
		if len(required) > 0 {
			o["required"] = required
		}
		return o
	}
	array := func(items map[string]any) map[string]any { return map[string]any{"type": "array", "items": items} }
	status := object(map[string]any{"status": text()}, "status")
	identity := object(map[string]any{"id": text()}, "id")
	notice := object(map[string]any{"id": text(), "workflow": text(), "subject": text(), "reference": text(), "created_at": timestamp(), "read": boolean()}, "id", "workflow", "subject", "reference", "created_at", "read")
	caseView := object(map[string]any{"id": text(), "owner_id": text(), "payment_id": text(), "kind": text(), "subject": text(), "status": text(), "created_at": timestamp(), "updated_at": timestamp()}, "id", "owner_id", "payment_id", "kind", "subject", "status", "created_at", "updated_at")
	message := object(map[string]any{"id": text(), "author_id": text(), "message": text(), "created_at": timestamp()}, "id", "author_id", "message", "created_at")
	kyc := array(object(map[string]any{"id": text(), "owner_id": text(), "status": text(), "evidence_reference": text(), "created_at": timestamp()}, "id", "owner_id", "status", "evidence_reference", "created_at"))
	funding := array(object(map[string]any{"id": text(), "amount_minor": money(), "currency": text(), "created_at": timestamp()}, "id", "amount_minor", "currency", "created_at"))
	sessions := array(object(map[string]any{"id": text(), "device": text(), "client": text(), "created_at": timestamp(), "expires_at": timestamp(), "revoked": boolean(), "current": boolean()}, "id", "device", "client", "created_at", "expires_at", "revoked", "current"))
	policy := object(map[string]any{"version": integer(), "per_payment_minor": money(), "daily_minor": money(), "internal_fee_minor": money(), "payments_enabled": boolean(), "currency": text()}, "version", "per_payment_minor", "daily_minor", "internal_fee_minor", "payments_enabled", "currency")
	entry := object(map[string]any{"id": integer(), "journal_id": text(), "reference": text(), "kind": text(), "delta_minor": signed(), "created_at": timestamp()}, "id", "journal_id", "reference", "kind", "delta_minor", "created_at")
	concrete := map[string]map[string]any{
		"GET /health/live":       object(map[string]any{"status": text(), "service": text()}, "status", "service"),
		"GET /health/ready":      status,
		"GET /v1/capabilities":   object(map[string]any{"environment": text(), "currency": text(), "external_payments_configured": boolean(), "notifications": text(), "synthetic_execution": boolean(), "growth_products_enabled": boolean(), "production_acceptance": boolean()}, "environment", "currency", "external_payments_configured", "notifications", "synthetic_execution", "growth_products_enabled", "production_acceptance"),
		"GET /v1/wallet/funding": funding,
		"GET /v1/statements":     object(map[string]any{"currency": text(), "from": timestamp(), "to_exclusive": timestamp(), "opening_minor": signed(), "closing_minor": signed(), "entries": array(entry), "generated_at": timestamp()}, "currency", "from", "to_exclusive", "opening_minor", "closing_minor", "entries", "generated_at"),
		"GET /v1/notifications":  object(map[string]any{"items": array(notice), "unread": integer()}, "items", "unread"),
		"GET /v1/preferences":    object(map[string]any{"optional_email": boolean(), "marketing_email": boolean()}, "optional_email", "marketing_email"),
		"GET /v1/beneficiaries":  array(object(map[string]any{"id": text(), "label": text(), "account_name": text()}, "id", "label", "account_name")),
		"GET /v1/kyc/cases":      kyc, "GET /v1/admin/kyc/cases": kyc,
		"GET /v1/payments/{id}/fulfilment":    object(map[string]any{"status": text(), "payment_status": text(), "value": text()}, "status", "payment_status"),
		"POST /v1/quotes/{id}/authorisations": object(map[string]any{"authorisation_token": text(), "quote_id": text(), "expires_at": timestamp()}, "authorisation_token", "quote_id", "expires_at"),
		"GET /v1/admin/policy":                policy,
		"GET /v1/admin/overview":              object(map[string]any{"customers": integer(), "pending_payments": integer(), "dead_jobs": integer(), "wallet_liability_minor": money(), "held_minor": money(), "currency": text()}, "customers", "pending_payments", "dead_jobs", "wallet_liability_minor", "held_minor", "currency"),
		"GET /v1/admin/proposals":             array(object(map[string]any{"id": text(), "maker_id": text(), "action": text(), "target": text(), "status": text(), "reason": text(), "expires_at": timestamp()}, "id", "maker_id", "action", "target", "status", "reason", "expires_at")),
		"GET /v1/admin/reconciliations":       array(object(map[string]any{"id": text(), "source": text(), "created_at": timestamp(), "rows": integer(), "exceptions": integer()}, "id", "source", "created_at", "rows", "exceptions")),
		"POST /v1/admin/reconciliations":      object(map[string]any{"id": text(), "results": array(object(map[string]any{"reference": text(), "matched": boolean(), "reason": text()}, "reference", "matched", "reason")), "scope": text()}, "id", "results", "scope"),
		"GET /v1/admin/audit":                 array(object(map[string]any{"id": text(), "actor": text(), "action": text(), "target": text(), "details": map[string]any{"type": "object"}, "created_at": timestamp()}, "id", "actor", "action", "target", "details", "created_at")),
	}
	for _, p := range []string{"/v1/banks/enquiries", "/v1/bills/validations"} {
		concrete["POST "+p] = object(map[string]any{"id": text(), "name": text(), "expires_at": timestamp(), "amount_minor": money()}, "id", "name", "expires_at", "amount_minor")
	}
	for _, p := range []string{"/v1/auth", "/v1/admin/auth"} {
		concrete["GET "+p+"/sessions"] = sessions
		concrete["GET "+p+"/csrf"] = object(map[string]any{"csrf_token": text()}, "csrf_token")
		concrete["POST "+p+"/mfa/enrol"] = object(map[string]any{"secret": text(), "otpauth_url": text()}, "secret", "otpauth_url")
	}
	for _, p := range []string{"/v1/support/cases", "/v1/admin/support/cases"} {
		concrete["GET "+p] = array(caseView)
		concrete["GET "+p+"/{id}/messages"] = array(message)
		concrete["POST "+p+"/{id}/messages"] = status
	}
	paths := spec["paths"].(map[string]any)
	for p, methods := range paths {
		for method, value := range methods.(map[string]any) {
			op := value.(map[string]any)
			responses := op["responses"].(map[string]any)
			for code, resp := range responses {
				if code == "default" {
					continue
				}
				response := resp.(map[string]any)
				if code == "204" {
					delete(response, "content")
					continue
				}
				content := response["content"].(map[string]any)
				body := content["application/json"].(map[string]any)
				envelope := body["schema"].(map[string]any)
				envelope["required"] = []string{"data", "request_id"}
				props := envelope["properties"].(map[string]any)
				model, exists := concrete[strings.ToUpper(method)+" "+p]
				if !strings.Contains(p,"/backups/") && !strings.HasPrefix(p,"/v1/backup-agent/") && !exists && method == "post" && (code == "201") && p != "/v1/quotes" {
					model = identity
					exists = true
				}
				if !strings.Contains(p,"/backups/") && !strings.HasPrefix(p,"/v1/backup-agent/") && !exists && (method == "post" || method == "patch") && p != "/v1/payments" && p != "/v1/me" && !strings.HasSuffix(p, "/login") && !strings.HasSuffix(p, "/refresh") && !strings.Contains(p, "/mfa/") {
					model = status
					exists = true
				}
				if exists {
					props["data"] = model
				}
				if p == "/v1/statements" {
					content["text/csv"] = map[string]any{"schema": text()}
					op["description"] = "Complete range; 10000-row bound fails explicitly rather than truncating. JSON has opening/closing balances; CSV contains the entries. End is exclusive."
				}
				if p == "/openapi.json" {
					body["schema"] = map[string]any{"type": "object", "required": []string{"openapi", "info", "paths"}}
				}
			}
			params, _ := op["parameters"].([]any)
			if method == "get" && (p == "/v1/payments" || p == "/v1/wallet/entries" || p == "/v1/notifications" || p == "/v1/admin/customers" || p == "/v1/admin/audit" || p == "/v1/admin/console/records/{resource}") {
				params = append(params, map[string]any{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 50}})
				cursor := text()
				if p == "/v1/wallet/entries" {
					cursor = map[string]any{"type": "integer", "minimum": 0}
				}
				params = append(params, map[string]any{"name": "before", "in": "query", "schema": cursor, "description": "Exclusive cursor from the last item id; ordering is descending id."})
			}
			if p == "/v1/admin/console/records/{resource}" {
				params = append(params, map[string]any{"name": "search", "in": "query", "schema": map[string]any{"type": "string", "maxLength": 100}, "description": "Literal substring over the authorised redacted projection"})
			}
			if p == "/v1/statements" {
				for _, name := range []string{"from", "to"} {
					params = append(params, map[string]any{"name": name, "in": "query", "required": true, "schema": timestamp()})
				}
				params = append(params, map[string]any{"name": "format", "in": "query", "schema": map[string]any{"type": "string", "enum": []string{"json", "csv"}, "default": "json"}})
			}
			if len(params) > 0 {
				// Registry defaults and concrete contracts may describe the same
				// parameter. Keep its position, but let the richer contract win.
				unique := make([]any, 0, len(params))
				indexes := make(map[string]int, len(params))
				for _, value := range params {
					parameter := value.(map[string]any)
					key := parameter["in"].(string) + ":" + parameter["name"].(string)
					if index, ok := indexes[key]; ok {
						unique[index] = parameter
					} else {
						indexes[key] = len(unique)
						unique = append(unique, parameter)
					}
				}
				op["parameters"] = unique
			}
		}
	}
	spec["x-production-accepted"] = false
	return spec
}
