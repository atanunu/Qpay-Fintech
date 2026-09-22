# API and contract plan
Status: proposed resource families; endpoints are not implemented and no provider compatibility is certified.

## Contract deliverables
Create contracts/openapi/customer.yaml and admin.yaml, versioned event schemas, error-code registry, pagination/filtering conventions, auth and idempotency policies, examples and consumer contract tests. Generate TypeScript clients reproducibly for the approved stack. Dart is not selected for this baseline. Source paths here are planned destinations, not files already shipped in this pack.

## Resource families
| Area | Proposed resources and commands |
|---|---|
| Identity | registrations, contact-verifications, sessions, devices, factors, recovery requests |
| KYC | applications, upload authorisations, status, evidence requests |
| Wallet | accounts, balances, limits, statements, funding instructions |
| Transfers | banks, account-enquiries, beneficiaries, quotes, authorisations, transfers, status, receipts |
| Bills | categories, billers, products, validations, quotes, orders, fulfilments, token retrieval |
| Service | notifications, support cases, disputes, privacy requests |
| Admin | scoped customer/transaction queries, proposals/approvals, risk cases, reconciliation, audit and operational controls |
| Integration | authenticated provider callbacks, upstream requests, internal job contracts and optional future partner webhooks |

Prefer explicit commands for irreversible actions. A GET must not initiate financial movement. Accept/pending responses identify the durable operation and status endpoint; they are not successful receipts. Expose safe request IDs and stable machine error codes without stack traces or provider secrets.

## Authentication
Customer and staff token audiences are distinct. Browser cookie sessions require secure cookie, CSRF and origin policies appropriate to the selected deployment. Mobile sessions require secure token storage, rotation, expiry and revocation. A backend-for-frontend may broker sessions but cannot bypass API authorisation. Server-to-server keys must be scoped, rotated and never placed in mobile or browser bundles.

## Financial request controls
Document lossless money encoding; idempotency fingerprint conflict handling; stale quote rejection; transaction-bound step-up; resource ownership; pagination bounds; request/body limits; replay prevention; version and compatibility rules. Apply rate limiting across instances to sensitive actions and enumeration endpoints. Account/name enquiries and identity checks also need abuse budgets.

## Example lifecycle, not live API syntax
Customer requests quote; engine snapshots beneficiary/amount/fees; customer authorises that quote; engine persists intent, reservation and outbox; worker submits to the selected upstream; verified observations drive the state machine; customer queries the durable intent. Webhooks, polling and reconciliation converge on the same state rather than each independently debiting funds.

Document backward compatibility for older mobile versions. API deprecation must have a supported-client window, feature gates and server-side minimum-version policy that does not strand access to funds or support.
