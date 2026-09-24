# Implemented interfaces

All routes below are under `/v1/admin`. Staff web login uses `{email,password,client:"web",device,mfa_code?,recovery_code?}`. Responses follow `{data,request_id}` or `{error:{code,message},request_id}`. Access/refresh tokens are cookies, never browser bearer response fields. Read `/auth/csrf` after reload; restore `/auth/me`, and rotate `/auth/refresh` only when access expired. No automatic mutation replay occurs. Authentication is separate from a recent ten-minute authorisation for sensitive operations.

| Route | Behaviour |
|---|---|
| POST /auth/login; GET /auth/csrf; POST /auth/refresh; GET /auth/me | Existing staff cookie lifecycle |
| POST /auth/mfa/enrol; POST /auth/mfa/confirm | Mandatory authenticator; manual key and one-display recovery codes |
| GET /auth/sessions; DELETE /auth/sessions/{id}; POST /auth/logout | Owned staff sessions |
| POST /auth/elevate | Current password + unused TOTP grants ten minutes of mutation authority |
| POST /console/invitations; POST /console/invitations/{id}/decision | Proposed invitation, independent approve/reject or revoke |
| POST /invitations/accept; POST /recovery/accept | One-use private grant; normal login + MFA required afterwards |
| GET /console/bootstrap; GET /console/metrics | Current capabilities and database-backed snapshot |
| GET /console/records/{resource}[/{id}] | Allowlisted redacted list/detail with audited access |
| POST /console/actions | Role-, reason-, version- and target-bound operational command |
| POST /console/exports | Reasoned CSV; max 5,000 matching rows, over-limit error rather than truncation |
| POST /console/documents/{id}; POST /console/identity/{id} | Reasoned private evidence access |
| POST /console/payments/{id}/fulfilment | Read delivered encrypted bill value with audit; never vend again |
| POST /console/payments/{id}/requery | Queue only an eligible dead job for original-reference recovery |
| POST /console/proposals/{id}/decision | Independent financial decision with recorded reason |
| GET /console/catalogue; POST /console/biller-observation | Normalised configured catalogue and time-limited observed state |
| GET /console/payment-control | Version for platform emergency controls |
| GET /policy; POST /proposals; POST /reconciliations | Existing financial policy, immutable proposal and normalised import |
| GET /support/cases/{id}/messages; POST /support/cases/{id}/messages | Role-scoped encrypted customer conversation |

Console commands: `note`, `work_create`, `work_update`, `support_assign`, `customer_sessions_revoke`, `schedule_stop`, `notification_suppress`, `emergency_stop`, `control_propose`, `control_decide`. Fields are strict JSON: action, target, resource, reason, value, title, assigned_to, severity and version. Unsupported fields/actions fail. Work resource is risk/incident/exception/refund/return. Controls cover staff_role, staff_status, staff_recovery, product and resume. No execution/paid/refunded command exists.

List parameters: `limit` 1–100, `before` exclusive reference, `search` literal substring up to 100 characters. Default console page is 50. Amounts remain integer minor-unit strings and signed ledger values; TypeScript uses BigInt for display and sorting. Resource names are a fixed server map, never interpolated into arbitrary SQL. CSV formula prefixes are neutralised. Import fields are exactly reference,amount_minor,currency,status; positive integer strings, NGN and succeeded/failed/pending. Independent source is bank or provider; duplicate identical imports recover the same report.

409 means stale/conflicting state; refresh the record, investigate and create a new proposal only when appropriate. 403 step_up_required directs to My security. A lost action response must be reconciled using its directory/audit record before manually resubmitting. Invitation and recovery secrets cannot be retrieved after display; revoke/reissue through independent review rather than claiming an email was delivered.
