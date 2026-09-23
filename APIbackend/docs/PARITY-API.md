# Customer parity API — version 0.6

## Contract and authority
Routes are implemented in `internal/httpapi/parity_routes.go`; the running `/openapi.json` and `go run ./cmd/contracts` export the actual route registry. Customer browser requests use HttpOnly cookies, approved Origin and CSRF on writes. Native clients use transport-bound bearer sessions. All payload money uses integer NGN minor-unit strings. Private state is never trusted from the browser. Every write validates ownership and current session/eligibility as applicable; financial operations recheck inside the ledger transaction.

## Endpoint groups
| Group | Endpoints | Required behaviour |
|---|---|---|
| Capability/controls | GET me/capabilities; GET/PATCH me/controls; POST me/freeze | Current limits and exact opt-in handle; password/MFA for sensitive change |
| Sessions/passkeys | POST auth/logout-all; GET auth/history; POST auth/passkeys/begin and finish; GET passkeys; POST passkeys/register/begin and finish; POST passkeys/{id}/revoke | Browser-bound one-use WebAuthn ceremony; UV/RP/origin checks and existing TOTP |
| Recipients | GET recipients/lookup?handle=; PATCH beneficiaries/{id}/favourite | Exact opt-in recipient only; scoped favourites |
| Payment recovery | GET payments/lookup?idempotency_key=&quote_id=; GET payments/{id}/timeline and draft | Original-operation recovery; absence is not failure; repeat prepares a new quote |
| One-off bank quote | POST quotes with kind=bank, enquiry_id, amount_minor, currency, narration | Exactly one owned enquiry or beneficiary; fresh name verification |
| Bills | GET/POST saved-bills; PATCH/DELETE saved-bills/{id}; GET bills/token-archive?before= | Encrypted customer references, favourites, original-value retrieval |
| Availability | GET bills/availability; POST bills/watch | Timestamped unknown/available/unavailable observations; watching boolean |
| Reminders | GET/POST reminders; PATCH reminders/{id} | Due date, optional end, once/weekly/monthly cadence; versioned pause/resume/cancel; never debit |
| Mandates | GET/POST mandates; PATCH mandates/{id}; GET mandates/{id}/history | Internal recipients only; explicit PIN/MFA, count and debit caps; immutable terms |
| Requests | GET/POST money-requests; GET money-requests/{id}; POST money-requests/{id}/action and quote | Idempotent creation; exact shares; current cancellation/expiry/remaining balance checks |
| Insights | GET insights?month=YYYY-MM; GET insights/categories; POST budgets; DELETE budgets/{category}?month= | Posted-month full aggregates; category budget; amount strings |
| Annotations/search | GET/PATCH payments/{id}/annotation; GET search?q= | Own private note/category/exclusion; scoped bounded search |
| Funding | GET/POST funding/account | Persist-before-call provisioning; original-reference queries; actual partner is gated |
| Identity | GET/PATCH identity/draft; POST identity/submit | Optimistic version; owned accepted uploads; immutable legal-name snapshot |
| Documents | GET uploads?purpose=&case_id=; POST uploads multipart; DELETE uploads/{id}; GET uploads/{id}/download | One 5 MiB JPEG/PNG/PDF; ownership/scanner/encryption; attachment-only download |
| Contact/closure | POST me/email-change; POST me/email-change/{id}/verify; GET/POST me/closure; POST me/export | Reauthentication; dual email proof; zero balance and no unresolved obligations; explicit limited export |
| Support | POST support/cases/{id}/escalate; GET support/cases/{id}/timeline | Owned case; immutable event timeline; no refund implied |
| Staff | GET admin/identity/cases/{id}; POST admin/bills/availability | Current MFA staff permissions; audit |
| Internal | POST internal/funding/callback | Separate signed hint then independent partner credit verification |

All listed public paths begin `/v1/`; internal funding begins `/internal/`. Full field definitions, required headers and multipart schema are exported from the Go types. Most response collections currently have a bounded limit; the endpoint summary must not be read as proof of full asynchronous export support. Missing account/product capabilities return explicit errors, not synthetic fallback.

## Important request fields
Controls: handle, discoverable, per_payment_minor, daily_minor, password, optional mfa_code. Saved bill: label, product_id, customer_id, amount_minor, favourite. Reminder: title, amount_minor, optional bill_id, cadence, due_at, optional ends_at. State changes: status and version. Mandates use the immutable quote_id, title, cadence, first_at, ends_at, max_occurrences, max_debit_minor, max_total_minor, pin, optional mfa_code; consult generated schema for exact required fields. Requests require a unique Idempotency-Key, total amount, title, expiration and explicit user/amount shares; share payment always derives the authenticated payer’s own share.

## Runtime configuration
Passkeys: `PASSKEY_RP_ID`, current `WEB_ORIGINS`. Private objects: `PRIVATE_UPLOAD_MODE=off|local|s3`; local root; explicit `PRIVATE_S3_ENDPOINT/BUCKET/REGION/ACCESS_KEY/SECRET_KEY`, `CLAMD_SOCKET` for nonlocal scanning. Funding currently supports only `FUNDING_MODE=off|local`; implement a real adapter rather than invent another mode. Callback authentication uses separate `FUNDING_CALLBACK_KEY`. Outside local testing, internal schedules additionally require `SCHEDULED_INTERNAL_ACCEPTED=true` and the ordinary accepted account policy.

Local Compose uses a dedicated encrypted-object volume owned by the non-root app user. Offline seed/control commands disable upload-store initialisation when running as the host user to read owner-only fixture secrets. This is not a HTTP/authentication bypass. Never use synthetic funding accounts, scanner states or generated credentials outside local mode.

## Migration and notification linkage
Apply reviewed checksum migration two explicitly. Do not edit migration one or a previously applied version-two checksum. New events include calendar reminder due, request received/reminded/paid, mandate authorised/paused/executed, dual-mailbox proofs and passkey changes. Eleven stable workflow IDs are added; the canonical catalogue has 175 entries. The Go embedded catalogue is byte-checked against Novu. Content and producers do not prove a deployed Novu bridge or delivered emails.

## Remaining acceptance
[Remaining engineering and partner gates](REMAINING-WORK.md) · [Web feature scope](../../devdocs/WebApp/02-COMPETITOR-PARITY.md) · [Customer state rules](../../devdocs/WebApp/03-CUSTOMER-JOURNEYS-AND-STATES.md). No deployment, real transaction, provider account change or production workflow activation is included in this review delivery.
