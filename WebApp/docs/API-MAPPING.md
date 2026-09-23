# API contract and screen mapping

All requests are made to **APIbackend**, never directly to QPay or Novu. The backend candidate is a persistent Go/PostgreSQL core. Its provider and production acceptance are separate. This integration follows the actual `internal/httpapi/routes.go` and transport structs, not an invented endpoint list.

| Screen/journey | Actual API use | Remaining boundary |
|---|---|---|
| Registration | POST `/v1/auth/register`; POST `/v1/auth/challenges`; POST `/v1/auth/challenges/verify` | Real email delivery and full contact lifecycle remain backend/Novu work |
| Login/recovery | POST login/refresh/logout; GET auth/me and auth/csrf | Exact same-site deployment must qualify browser cookies |
| MFA/security | POST auth/mfa/enrol and confirm; POST auth/password and pin | Manual recovery and incident acceptance required; no QR/secret caching |
| Devices | GET auth/sessions; DELETE auth/sessions/{id} | Current-session revocation signs out; other sessions reload |
| Overview/wallet | GET wallet, wallet/funding, notifications and payments | Partner funding account creation is not available |
| Bank transfer | GET banks; POST banks/enquiries; POST beneficiaries; POST quotes | Current API requires saved verified beneficiary; no guessed account name |
| Internal transfer | POST quotes with exact recipient_id | Customer-safe phone/email/handle discovery is pending |
| Bill purchase | GET bills/products; POST bills/validations; POST quotes | Category/product normalisation and actual upstream qualification remain |
| Payment approval | POST quotes/{id}/authorisations, then POST payments with Idempotency-Key | No automatic retry of a money-changing request |
| Payment recovery | GET payment by ID, or bounded history search by quote_id | Owner-scoped lookup by idempotency key is recommended |
| Activity and receipts | GET payments with cursor; GET payments/{id}/receipt | Receipt only on confirmed success; JSON/download/print uses real result |
| Fulfilment | GET payments/{id}/fulfilment | A bill's financial success is distinct from token/value delivery |
| Statements | GET statements with WAT-derived RFC3339 range, JSON or CSV | No more than 366 days or 10,000 entries; async jobs pending |
| Profile/preferences | GET/PATCH me; GET/PATCH preferences | Contact changes, privacy exports and closure not implemented by API |
| Identity verification | GET/POST kyc/cases with private evidence_reference | Digital upload is a marked in-memory UI draft, not verification |
| Support/disputes | GET/POST support/cases; GET/POST case messages | Text-only; attachments, inbound delivery and financial refund execution pending |
| Notification centre | GET notifications; POST notifications/{id}/read | Uses the existing backend feed, not a competing financial authority or unqualified SDK |

## Transport rules
API responses use `{data,request_id}` and failures use `{error:{code,message},request_id}`. A stable support reference is displayed for server errors. Browser calls always use `credentials: include`; access/refresh cookies remain HttpOnly. The client rejects a browser login response containing mobile access/refresh tokens. CSRF is held in memory, restored through the approved origin and refreshed under an in-flight/Web Locks coordination boundary.

Only GET requests may automatically repeat after session recovery. Password changes, approvals and financial writes never do. A successful read does not approve a payment. Refresh races, cookie restrictions and session revocation must still pass on the actual deployment origins.

Money uses exact integer NGN minor-unit strings and BigInt. Inputs accept plain naira/kobo, not floats, exponents or rounded currency. Quote and wallet totals are validated before display. IDs are URL-encoded and object permission checks stay on the server.

## Persisted operation recovery
Immediately before payment submission, save only owner ID, quote ID, a cryptographically random idempotency key, time and phase to sessionStorage. Never store PINs, approval tokens, passwords, contact or account details. A storage failure prevents submission. On an unknown response, preserve references and query the original. A missing search result is not proof of failure. Completed/failed records clear the matching browser recovery reference. A different account cannot load another owner's reference.

The saved intent belongs to a browser tab. Cross-device recovery uses the authoritative account history. The current history search is capped at 1,000 entries and is explicitly documented as an API improvement. No arbitrary new payout is sent to resolve a missing response.

## Unsupported operations
`src/api/integration.ts` is the canonical UI integration-gap register. Each GAP links the relevant UI and API task IDs. There are no speculative HTTP calls for funding account creation, card collection, identity upload, contact changes, privacy exports, account closure or refund execution. Their buttons open marked review journeys or secure support, never fictional success.
