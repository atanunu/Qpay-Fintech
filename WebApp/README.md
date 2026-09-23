# WebApp — Qpay customer web application

**Approved stack:** React + TypeScript + Vite + React Router (W1). Bespoke, responsive light/dark customer interface powered by `APIbackend/`. Authentication layouts are separate from the signed-in portal.

**v0.6 parity review candidate:** customer security, identity, funding visibility, household bills, reminders, internal schedules, requests and insights now have backend implementations and dedicated UI. Local unit checks pass; the expanded normal-browser suites require final CI verification. In-person acceptance and real provider activation remain separate.

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

[Full endpoint mapping](docs/API-MAPPING.md) specifies concrete request shapes and missing capabilities. The preserved fourteen GAP items distinguish implemented backend components from remaining partner and programme requirements. Added endpoints and limitations are documented in [parity API contracts](../APIbackend/docs/PARITY-API.md).

## Feature and task register
Implementation, browser verification, upstream acceptance and production release are separate. The preserved WEB IDs remain canonical; generated JSON follows this table. At this candidate stage final browser/GitHub evidence remains pending; API gaps are not concealed as completed features.

<!-- FEATURES:START -->
| ID | Capability / task | Milestone | Priority | Status | Specification | Acceptance and required verification | Source / tests / issue |
|---|---|---|---|---|---|---|---|
| WEB-001 | Independent web and authentication shells | M1 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Login recovery and verification exclude authenticated navigation | src/components/layout.tsx; pages/auth.tsx; independent shells and route guarding; browser verification pending |
| WEB-002 | Registration verification and login | M1 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Validate safe errors expiry resend and authenticated entry | pages/auth.tsx; client.ts; registered API contracts and review journeys; real email transport pending |
| WEB-003 | Session MFA and device controls | M1 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Secure session CSRF origin and revocation policies pass | state.tsx; cookie/CSRF transport; security/MFA/devices forms; deployment/session acceptance pending |
| WEB-004 | Customer KYC workflow | M2 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Upload and verification flow handles failures and review without data leaks | pages/funding_identity.tsx; private documents and resumable immutable submission; parity API tests; automatic identity provider remains GAP-03 |
| WEB-005 | Wallet and funding instructions | M2 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Show authoritative scoped available held and funding states | pages/overview.tsx; authoritative wallet and persistent funding provisioning; local-only TEST adapter; real partner remains GAP-01/02 |
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
| WEB-019 | Contract component and browser tests | M1 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Real integration paths use maintained generated contracts and deterministic tests | tests/core.test.ts, review.test.ts, parity.test.ts: 71 local passes; expanded normal browser and API verification pending |
| WEB-020 | Screenshot capture and deployment | M4 | P0 | Partial | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Publish synthetic runtime evidence with source hash and clean deployment proof | Playwright capture suite and deployment templates present; verified revision captures and actual deployment acceptance pending |
| WEB-021 | Approved business and growth journeys | M5 | P1 | Planned | [Implementation](../devdocs/WebApp/01-IMPLEMENTATION-AND-REVIEW.md) | Split selected products into end-to-end web and operations tasks | src/api/integration.ts GAP-13 decision register; unapproved expansion is not presented as executable services |
| WEB-022 | Account capabilities and effective limits | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | CapabilitiesCard and current policy state | src/components/parity.tsx; e2e/parity.api.spec.ts |
| WEB-023 | Exact handles and one-off bank recipients | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | Owned enquiry and fresh quote; no compulsory saving | src/pages/payments.tsx; core and API browser suites |
| WEB-024 | Original payment resolution and safe repeat preparation | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | Owner lookup; hold timeline; new quote for repeat | src/api/recovery.ts; components/payment_parity.tsx; tests/core.test.ts |
| WEB-025 | Saved household bills and protected token archive | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | Private saved accounts and original-value retrieval | src/pages/planner.tsx; e2e/parity.review.spec.ts; parity.api.spec.ts |
| WEB-026 | Payment reminders and bounded internal schedules | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | Explicit approval caps and per-occurrence traceability | src/pages/planner.tsx; APIbackend/internal/service/parity_test.go |
| WEB-027 | Requests split shares and partial payments | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | Exact sums; participant privacy; cancel rechecked at commit | src/pages/requests.tsx; tests/parity.test.ts; e2e/parity.api.spec.ts |
| WEB-028 | Complete monthly insights annotations and budgets | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | Posted-ledger aggregate; exclusions never change balances | src/pages/insights.tsx; components/payment_parity.tsx; parity tests |
| WEB-029 | Passkeys personal freeze and security controls | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | Actual WebAuthn and existing MFA; no payment auto-approval | src/pages/security_parity.tsx; e2e/parity.api.spec.ts |
| WEB-030 | Private digital identity draft and submission | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | Owned checked uploads and immutable submitted identity | src/pages/funding_identity.tsx; e2e/parity.api.spec.ts |
| WEB-031 | Funding lifecycle and missing-credit support | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | No invented bank details; independently verified provider contract | src/pages/funding_identity.tsx; backend parity tests; actual partner gated |
| WEB-032 | Private support evidence and escalation | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | Owned attachments and audited case events; not financial refund | src/pages/support.tsx; components/parity.tsx; API browser suite |
| WEB-033 | Dual-mailbox change scoped export and guarded closure | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | Step-up proofs; no funds or open cases at closure | src/pages/security_parity.tsx; backend parity tests; SMS and full export gated |
| WEB-034 | Responsive search masking and compact presentation | M3 | P0 | Partial | [Parity scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md) | Desktop/mobile navigation and labels; no private offline cache | src/components/layout.tsx; styles/app.css; e2e/review.spec.ts |
<!-- FEATURES:END -->

## Done, pending and blocked
The original core is extended with one-off bank quotes, exact opt-in handles, direct original-payment lookup, private identity and support documents, household bills, WAT calendars, bounded internal mandates, partial split-request payment, monthly budgets/insights, passkeys, personal freeze, dual-mailbox changes and guarded account closure. A separate synthetic review remains available; it cannot replace actual API errors.

Pending: final browser/CI confirmation, in-person sign-off, live funding partner, actual Novu delivery, full refunds/returns and treasury, automatic identity-provider/liveness/camera handoff, verified SMS, all-record asynchronous privacy export, complete support assignment/SLA/inbound replies and external autopay. Cards/credit/interest products are not offered. [Detailed scope](../devdocs/WebApp/02-COMPETITOR-PARITY.md).

## Tests and evidence
Local TypeScript checks and 71 frontend unit/review tests passed. Go-backed parity domain tests ran against isolated PostgreSQL; normal HTTP browser review and real API integration suites remain required final CI evidence. See [validation](docs/VALIDATION.md). The WebAuthn suite uses a browser virtual authenticator and actual Go verification. No live financial, email or KYC provider was contacted. Manual UAT remains not-tested.

Changed runtime screenshots must be published from the CI browser capture, with provenance, before calling the visual review complete.

## Security and permissions
Only server-authorised data is displayed in API mode. PINs, passwords, MFA setup/recovery secrets and approval tokens are never stored in browser storage or analytics. Recovery stores scoped references only. No service worker, offline payment execution, untrusted HTML or browser provider keys. Financial submit is explicitly gated and blocked for a production capability report in this release. Read [security boundaries](docs/SECURITY-AND-OPERATIONS.md).

## Operations, deployment and troubleshooting
Use explicit API or review configuration, never a timeout-triggered fallback. On a lost response, query the saved original operation; absence is not failure. On a 401, recover reads but do not auto-replay writes. On stale quotes or policy changes, request and approve a new quote. For unavailable funding/KYC/closure capabilities, use the gap register rather than editing balances or fabricating completion. Static hosting requires same-site HTTPS origins, narrow CSP, SPA fallback, private no-store/noindex and verified ingress. Production hosting, providers, finance and legal approval remain separate.

## Roadmap, limitations and maintenance
Use the 13 manual acceptance journeys in the [in-person plan](docs/IN-PERSON-TESTING.md), then close agreed API gaps before upstream QPay acceptance. Source, tests, endpoint contracts, task register, root summary and changed actual screenshots are a single delivery. Preserve stable task IDs and do not confuse UI completion with regulatory or provider approval. The synthetic adapter is deliberately not a banking engine or a substitute for real security tests.

## Changelog
2026-09-23 v0.6: approved everyday-customer parity implementation, additive backend migration/contracts, browser baseline fixes, expanded test harness and gated external product boundaries. See [changelog](CHANGELOG.md).

Delivery recovery: hash-verified application records restored, documentation tail rebuilt and OpenAPI duplication corrected. [Delivery evidence and remaining acceptance](../devdocs/project/GITHUB-DELIVERY.md).

Browser delivery correction: native fetch receiver fixed in both clients; customer capability display now uses the API payments field and fails closed on unknown values; bank quote submission waits for its selected beneficiary; recovery tests wait for route changes. Added regression tests. Local frontend tests: 79 passed; shared client tests: 7 passed. GitHub rerun and manual UAT remain distinct acceptance gates.

Form readiness: bill validation remains disabled until the requested product is loaded. The recovery browser case waits for the actual recovery screen before typing, not only its URL. GitHub run 35837962795 passed all 11 actual Go/PostgreSQL browser journeys and all 38 screenshot/accessibility captures; two review form races were corrected here and remain subject to a full rerun. No assertions or retries were removed.
