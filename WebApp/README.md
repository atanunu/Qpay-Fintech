# WebApp — Qpay customer web application

**Approved stack:** React + TypeScript + Vite + React Router (W1). Bespoke, responsive light/dark customer interface powered by `APIbackend/`. Authentication layouts are separate from the signed-in portal.

**v0.5 review candidate:** customer screens and API bindings are implemented; 58 local unit/contract tests pass. API-connected browser acceptance and the final GitHub revision are being verified. This is an in-person test release, not a real-money launch. UI-ready drafts are labelled and cannot silently execute unsupported operations.

## Purpose and boundaries
Provide a complete customer review experience now, wire the API features that exist, and expose actionable integration gaps before QPay provider activation. The Go service owns identity, permissions, balances, ledger, execution and business outcomes. The browser does not hold provider secrets or directly call QPay/Novu. [Shared project](../README.md) · [Implementation plan](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md).

## Architecture and visuals
`Customer interface → validated cookie/CSRF transport → APIbackend → PostgreSQL / selected execution adapter`.

The separate synthetic review build uses local fixtures only; it is never an error fallback. Actual screenshots belong in docs/screenshots with revision, route/state, time, viewport, fixture and image-hash provenance. No real customer or credential screenshots are permitted. [Visual evidence and validation](docs/VALIDATION.md).

## Setup, configuration and commands
Node.js 22.12+ and the committed package lock are required. From the repository root:
```sh
cd WebApp
npm ci
npm run dev:review
```
Open `http://localhost:5173`. Use **Fill sample sign-in**. Synthetic password: `Review-only-2026`; code/PIN: `123456`. All data and money in this mode are fictitious. Review data is scoped to the browser tab; reset clears the sample operation state.

API-connected local testing:
```sh
# From the repository root, in a separate terminal:
python3 APIbackend/scripts/local_config.py
docker compose --env-file APIbackend/.env.local -f APIbackend/compose.local.yml up --build -d api worker scheduler

# Then start the browser application:
cd WebApp
VITE_API_MODE=api VITE_API_BASE_URL=http://localhost:8080 VITE_ENABLE_REVIEW_TOOLS=true npm run dev
```
Use controlled synthetic backend accounts; the frontend does not invent customer credentials or expose a public funding/verification bypass. Keep the same `localhost` hostname on both sides for Strict cookies. Actual API startup and browser results must be checked in the verification report; a command in a README is not a passing run.

```sh
npm run typecheck
npm test
npm run build
npm run build:review
npm run build:offline
npm run test:e2e
```
`build` is API-connected with an explicitly configured HTTPS API origin; its safe example default is unavailable, not a demo fallback. `build:review` serves synthetic UI. `build:offline` also produces a standalone `dist/qpay-offline-review.html` using hash routing. `test:e2e:api` additionally requires the isolated local API fixture. [In-person procedure](docs/IN-PERSON-TESTING.md) · [Deployment](deploy/README.md).

## Source and API map
| Area | Source and responsibility |
|---|---|
| Routing/bootstrap | src/App.tsx, main.tsx; protected routes and separate shells |
| Transport/session | src/api/client.ts, config.ts, types.ts; state.tsx |
| Financial safety | api/money.ts, id.ts, recovery.ts; quote/approval pages |
| Authentication | pages/auth.tsx; verification, recovery and secure sign-in |
| Payments and bills | pages/payments.tsx, activity.tsx; discovery, quote, approval, status, receipt and value |
| Accounts and support | pages/account.tsx, overview.tsx, records.tsx, support.tsx |
| Review and gaps | pages/review.tsx; api/integration.ts; review/transport.ts |
| Tests | tests/ unit/contract suites; e2e/ normal HTTP and real API browser suites |

[Full endpoint mapping](docs/API-MAPPING.md) specifies concrete request shapes and missing capabilities. Fourteen GAP items map to API tasks. No speculative endpoints are called for unsupported funding, KYC upload, contact changes, closure or refunds.

## Feature and task register
Implementation, browser verification, upstream acceptance and production release are separate. The preserved WEB IDs remain canonical; generated JSON follows this table. At this candidate stage final browser/GitHub evidence remains pending; API gaps are not concealed as completed features.

<!-- FEATURES:START -->
| ID | Capability / task | Milestone | Priority | Status | Specification | Acceptance and required verification | Source / tests / issue |
|---|---|---|---|---|---|---|---|
| WEB-001 | Independent web and authentication shells | M1 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Login recovery and verification exclude authenticated navigation | src/components/layout.tsx; pages/auth.tsx; independent shells and route guarding; browser verification pending |
| WEB-002 | Registration verification and login | M1 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Validate safe errors expiry resend and authenticated entry | pages/auth.tsx; client.ts; registered API contracts and review journeys; real email transport pending |
| WEB-003 | Session MFA and device controls | M1 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Secure session CSRF origin and revocation policies pass | state.tsx; cookie/CSRF transport; security/MFA/devices forms; deployment/session acceptance pending |
| WEB-004 | Customer KYC workflow | M2 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Upload and verification flow handles failures and review without data leaks | pages/account.tsx; manual evidence case API plus clearly unsubmitted digital draft; GAP-03 |
| WEB-005 | Wallet and funding instructions | M2 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Show authoritative scoped available held and funding states | pages/overview.tsx; authoritative wallet and funding history; funding-account interfaces pending GAP-01/02 |
| WEB-006 | Beneficiaries and name enquiry | M2 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Customer ownership and current verified recipient details are enforced | pages/payments.tsx and account.tsx; enquiry, explicit save, ownership-backed API; browser verification pending |
| WEB-007 | Transfer quote and authorisation | M2 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Display exact fees and reject changed expired or unapproved intents | pages/payments.tsx; exact quote review, expiry, PIN and optional MFA; unit guards pass; browser acceptance pending |
| WEB-008 | Transfer pending and refresh recovery | M2 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Browser refresh and duplicate click recover existing operation safely | api/recovery.ts; reference-only persistence and read-only original lookup; interrupted review scenario; GAP-09 |
| WEB-009 | Bill discovery and validation | M3 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Render approved categories products and category-specific validation | pages/payments.tsx; six display categories from available products, service validation; provider catalogue acceptance pending |
| WEB-010 | Bill vend and token recovery | M3 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Show pending and fulfilment honestly and recover delayed token | pages/payments.tsx and activity.tsx; payment/value separation, original fulfilment retrieval; browser acceptance pending |
| WEB-011 | Receipts and controlled downloads | M3 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Only authoritative completed evidence generates a success receipt | pages/activity.tsx; success-only receipt API, JSON download and print; final browser acceptance pending |
| WEB-012 | History search and transaction details | M2 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Filters pagination and timelines remain scoped and accurate | pages/activity.tsx; loaded-record filters, cursor paging and scoped details; additional backend filters tracked GAP-10 |
| WEB-013 | Statement and export jobs | M3 | P1 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Exports are complete scoped and delivered through expiring authorisation | pages/records.tsx; bounded authoritative JSON/CSV statements; asynchronous jobs pending GAP-10 |
| WEB-014 | In-app notification centre | M2 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Read status and deep links remain secure and separate from payment execution | pages/records.tsx; backend-owned non-secret inbox, read state and protected reference routes; live delivery separate GAP-11 |
| WEB-015 | Support complaints and disputes | M3 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Linked cases support escalation and audited resolution | pages/support.tsx; case creation/replies/dispute references; attachments, refund execution and escalation pending GAP-07/08 |
| WEB-016 | Privacy security and account settings | M3 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Settings reflect actual backend policy and retention constraints | pages/account.tsx; name/preferences APIs; contact/privacy/closure drafts visibly unsubmitted GAP-04/05/06 |
| WEB-017 | Browser security and cache policy | M1 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | CSP CSRF origin and private caching rules prevent cross-session exposure | api/client.ts; cookies, CSRF, origin validation; deploy/nginx.conf; production ingress and cross-browser acceptance pending |
| WEB-018 | Responsive and accessible UX | M1 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Keyboard assistive and narrow-screen journeys cover errors and pending states | styles/app.css; responsive light/dark screens; local Chromium synthetic render has zero axe violations; HTTP browser suite pending |
| WEB-019 | Contract component and browser tests | M1 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Real integration paths use maintained generated contracts and deterministic tests | tests/core.test.ts and review.test.ts: 58 pass; e2e/review.spec.ts and api.spec.ts authored; remote execution pending |
| WEB-020 | Screenshot capture and deployment | M4 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Publish synthetic runtime evidence with source hash and clean deployment proof | Playwright capture suite and deployment templates present; verified revision captures and actual deployment acceptance pending |
| WEB-021 | Approved business and growth journeys | M5 | P1 | Planned | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Split selected products into end-to-end web and operations tasks | src/api/integration.ts GAP-13 decision register; unapproved expansion is not presented as executable services |
<!-- FEATURES:END -->

## Done, pending and blocked
Built: login/register/verification/recovery; wallet overview; bank/internal transfer; bill catalogue/validation/approval; original-operation recovery; activity/receipts; beneficiaries; statements; notification centre; profile/MFA/devices/preferences; manual KYC plus digital draft; support/complaint/dispute conversations; review checklist/scenarios and gap export; responsive light/dark UI.

Backend-dependent gaps: verified funding-account provisioning, card/USSD collection, digital KYC uploads, contact changes, privacy export and closure, support attachments, financial refunds, larger exports and production notifications. Their interfaces state what has and has not been submitted. Unapproved expansion remains planned. See [mapping and gaps](docs/API-MAPPING.md).

## Tests and evidence
58 local Vitest cases pass, including exact amount/date handling, API schema checks, browser credential isolation, no automatic write retry and reference recovery. The local Chromium component-render harness passed the tested desktop/mobile accessibility checks with synthetic fixtures. Normal HTTP browser and real Go/PostgreSQL browser suites are included for CI; report their actual results independently. [Validation](docs/VALIDATION.md).

## Security and permissions
Only server-authorised data is displayed in API mode. PINs, passwords, MFA setup/recovery secrets and approval tokens are never stored in browser storage or analytics. Recovery stores scoped references only. No service worker, offline payment execution, untrusted HTML or browser provider keys. Financial submit is explicitly gated and blocked for a production capability report in this release. Read [security boundaries](docs/SECURITY-AND-OPERATIONS.md).

## Operations, deployment and troubleshooting
Use explicit API or review configuration, never a timeout-triggered fallback. On a lost response, query the saved original operation; absence is not failure. On a 401, recover reads but do not auto-replay writes. On stale quotes or policy changes, request and approve a new quote. For unavailable funding/KYC/closure capabilities, use the gap register rather than editing balances or fabricating completion. Static hosting requires same-site HTTPS origins, narrow CSP, SPA fallback, private no-store/noindex and verified ingress. Production hosting, providers, finance and legal approval remain separate.

## Roadmap, limitations and maintenance
Use the 13 manual acceptance journeys in the [in-person plan](docs/IN-PERSON-TESTING.md), then close agreed API gaps before upstream QPay acceptance. Source, tests, endpoint contracts, task register, root summary and changed actual screenshots are a single delivery. Preserve stable task IDs and do not confuse UI completion with regulatory or provider approval. The synthetic adapter is deliberately not a banking engine or a substitute for real security tests.

## Changelog
2026-09-23: built the complete customer review interface and available API bindings, added synthetic scenarios, safe interrupted-payment recovery, strict transport/money checks, review export, browser/contract tests and API-gap documentation. Reconciled the supplied backend working source for joint local verification; production acceptance remains open.
