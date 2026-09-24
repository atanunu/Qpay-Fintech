# AdminDashboard · Qpay operations

**Status: runnable v0.7 implementation candidate.** API-connected staff application, synthetic review transport, supporting PostgreSQL operations and tests are implemented. Browser/CI qualification is recorded separately in [Validation](docs/VALIDATION.md). No live provider activation or production deployment is implied.

[Project](../README.md) · [Specification](../devdocs/AdminDashboard/00-INDEX.md) · [Operations](../devdocs/AdminDashboard/06-OPERATIONS.md) · [API map](../devdocs/AdminDashboard/02-INTERFACES.md) · [Gallery](docs/SCREENSHOTS.md)

**Approved stack:** bespoke React + TypeScript + Vite + React Router (A1), without Filament. Exact dependency versions are in package-lock.json. APIbackend remains the sole authority for roles, money, customer data and operations.

## Purpose and boundaries
A staff-only console for comprehensive administration of the capabilities implemented by APIbackend: customer/KYC review, payment investigations, ledger reporting, support, approvals, staff access, product containment, notifications and incident operations. The API, not a hidden button, enforces every permission. No direct database connection, financial balance editor, provider key form, unsafe resubmit, or production switch is exposed. Six fixed backend roles are supported: administrator, finance, compliance, support, platform and read-only auditor.

## Architecture and visuals
![Staff trust boundary; architecture diagram rather than screenshot](../docs/diagrams/admindashboard.svg)

The [runtime gallery](docs/SCREENSHOTS.md) records actual synthetic browser captures separately from this architecture design. [SCREENSHOTS.json](docs/SCREENSHOTS.json) records source revision, timestamp, viewport and hashes. No mockup is presented as runtime evidence.

```text
Staff browser → AdminDashboard → /v1/admin/* APIbackend → PostgreSQL
                                                   → selected execution adapter
                                                   → notification intent / Novu
```

The review build uses a deliberately separate in-memory adapter, always labelled synthetic. Errors in the API build never activate that adapter. Authentication screens have no staff sidebar/header. Three authentication layouts, light/dark theme, compact density, alternate workspace layout, mobile navigation, keyboard focus and controlled downloads are implemented.

## Setup, configuration and commands
Node 22.12 or newer is required; CI uses the existing qualified Node 22 and Go 1.27.1 baseline. Install from the lock rather than resolving new packages.

```bash
cd AdminDashboard
npm ci
npm run dev:review
# Separate API-connected development:
cp .env.example .env.local
# Set VITE_API_BASE_URL and only for isolated local testing VITE_ALLOW_LOCAL_HTTP=true.
npm run dev
npm run typecheck
npm test
npm run build
npm run build:review
npm run test:e2e
npm run test:e2e:api
```

Review runs on port 5174. Its non-secret accounts and credentials are displayed only in that explicitly synthetic build. Reload resets review data. API mode requires real staff cookies and MFA; no review credentials are compiled into it. Run `python3 scripts/check_content.py` from AdminDashboard to validate versioned template metadata.

For local joint testing, start APIbackend from its documented local Compose setup and configure **WEB_ORIGINS=http://localhost:5173** separately from **ADMIN_ORIGINS=http://localhost:5174**. The browser API hostname must consistently be `localhost` for cookie and origin behaviour. Bootstrap the first and second security administrators through the offline CLI with owner-only secret files; see [configuration](../devdocs/AdminDashboard/04-CONFIGURATION.md). Everyone else joins through an independently approved invitation and mandatory MFA.

## Source and API map
| Source | Responsibility |
|---|---|
| src/main.tsx, auth.tsx | Routes, staff authentication, MFA, invitations and controlled recovery |
| src/layout.tsx, modules.ts | Permission-aware navigation and 26 operational module definitions |
| src/pages/records.tsx | Paginated lists, details, reasoned actions, approvals, case conversations and evidence |
| src/pages/dashboard.tsx, settings.tsx | Operational snapshot, policy, reports, security, integration boundaries and template metadata |
| src/pages/reconciliation.tsx | Strict import validation, preview and stored comparison results |
| src/api.ts | Cookie + CSRF transport, origin constraints, session restoration and no mutation retry |
| src/review.ts | Explicit synthetic in-memory review, never a failure fallback |
| ../APIbackend/internal/service/admin_*.go | Authorised projections, immutable controls, investigations, exports and staff lifecycle |
| ../APIbackend/internal/service/admin_v3.sql | Additive migration 3; earlier checksums unchanged |

[Actual interfaces](../devdocs/AdminDashboard/02-INTERFACES.md) and [permission matrix](../devdocs/AdminDashboard/01-ARCHITECTURE.md) define the trust boundary. Generate OpenAPI with APIbackend's route exporter; never edit generated copies independently.

## Feature and task register
All original ADM IDs and acceptance requirements are preserved. **Partial does not mean the UI is absent:** it distinguishes implemented operations from remaining full-epic requirements and qualification. Feature-specific limitations below remain visible in the application. Engineering ownership: AdminDashboard + APIbackend; named operator and release approvers must be assigned before activation.

<!-- FEATURES:START -->
| ID | Capability / task | Milestone | Priority | Status | Specification | Acceptance and required verification | Source / tests / issue |
|---|---|---|---|---|---|---|---|
| ADM-001 | Staff invitation authentication and MFA | M1 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Invite-only access with independent staff session and recovery policy | auth.tsx; admin_commands.go and admin_recovery.go; HTTP, role, invitation and recovery tests; [validation](docs/VALIDATION.md) |
| ADM-002 | Role and permission enforcement | M1 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Server denies forbidden operations even through direct API requests | layout.tsx; admin_resources.go; six-role by 26-resource matrix and direct HTTP denial tests; [validation](docs/VALIDATION.md) |
| ADM-003 | Real operational overview metrics | M2 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Every displayed metric has a defined authoritative query and error state | pages/dashboard.tsx; AdminMetrics; transactional database snapshot; [validation](docs/VALIDATION.md) |
| ADM-004 | Customer and KYC review | M2 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Scoped document access supports reasoned decisions and immutable audit | pages/records.tsx and settings.tsx; reasoned evidence access and independent identity decisions; [validation](docs/VALIDATION.md) |
| ADM-005 | Restrictions and session controls | M2 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Approved restrictions take effect immediately in backend authorisation | admin_commands.go; customer session revocation and existing restriction approval engine; [validation](docs/VALIDATION.md) |
| ADM-006 | Financial transaction timeline | M2 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Show intent attempts holds journals observations and settlement separately | Payment detail exposes observations holds journals jobs; external settlement evidence remains gated; [validation](docs/VALIDATION.md) |
| ADM-007 | Unresolved payment investigation | M2 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Re-query original operation without offering unsafe resubmit shortcuts | Reasoned original-reference requery; no new payout or vend action; [validation](docs/VALIDATION.md) |
| ADM-008 | Maker-checker approvals | M2 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Different eligible operator approves against current immutable proposal | Immutable versioned proposals and operational controls; distinct maker/checker and stale-target tests; [validation](docs/VALIDATION.md) |
| ADM-009 | Bill catalogue and product management | M3 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Audit versioned mappings and disable unavailable products safely | Normalised catalogue plus independently approved product enable/disable; provider mappings still adapter-owned; [validation](docs/VALIDATION.md) |
| ADM-010 | Bill fulfilment and token recovery | M3 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Distinguish failed delivery from a financially failed vend | Separate financial and fulfilment states; audited delivered-value access; no re-vend shortcut; [validation](docs/VALIDATION.md) |
| ADM-011 | Provider health and capability view | M2 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Distinguish configured adapter runtime support and financial acceptance | pages/settings.tsx; configured capability and acceptance boundaries; timestamped biller observations; [validation](docs/VALIDATION.md) |
| ADM-012 | Pricing limits and policy versions | M2 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Effective-date changes preserve existing quote history and approval | Versioned immediate-upon-approval financial policy; future-effective scheduling not implemented; [validation](docs/VALIDATION.md) |
| ADM-013 | Reconciliation imports and matching | M2 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Validate file scope duplicates completeness and matching outcomes | Normalised 2000-row import; idempotent report identity and duplicate detection; full statement completeness qualification pending; [validation](docs/VALIDATION.md) |
| ADM-014 | Suspense and aged exception resolution | M2 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Breaks retain owner evidence age and approved resolution | Owned encrypted investigation records and due dates; journal-affecting resolution remains gated; [validation](docs/VALIDATION.md) |
| ADM-015 | Treasury float and settlement overview | M4 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Display verified obligations and funding alerts without mixing revenue with custody | Separate wallet liabilities holds fee income and clearing; actual bank balance and usable provider float unavailable; [validation](docs/VALIDATION.md) |
| ADM-016 | Refund return and dispute operations | M3 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Use supported approved compensating flows and evidence | Linked refund and return investigations; actual compensating execution unavailable; [validation](docs/VALIDATION.md) |
| ADM-017 | Risk compliance and investigation cases | M2 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Enforce case permissions reasons and escalation requirements | Permissioned risk and incident records; assignment, state changes and immutable encrypted notes; [validation](docs/VALIDATION.md) |
| ADM-018 | Support and complaint lifecycle | M3 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Track assignment escalation customer updates and resolution evidence | Versioned support assignment, customer replies, escalation timeline and evidence; live inbound mail remains gated; [validation](docs/VALIDATION.md) |
| ADM-019 | Notification templates and delivery | M3 | P1 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Review changes and inspect delivery without exposing sensitive payloads | 175 versioned template metadata entries, delivery-intent metadata and safe suppression; live callbacks and template publishing remain gated; [validation](docs/VALIDATION.md) |
| ADM-020 | Audit search and controlled export | M1 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Privileged access itself is logged and exports are scoped | Audited list/detail reads and reasoned capped CSV export with formula protection; [validation](docs/VALIDATION.md) |
| ADM-021 | Incident and emergency controls | M4 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Restrict stop or resume controls with clear blast radius and approval | Versioned emergency containment, independent resumption and incident investigations; [validation](docs/VALIDATION.md) |
| ADM-022 | Finance and management reports | M3 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Reports reconcile to ledger and separate income fees float and liabilities | Ledger-backed metrics and bounded exports; regulatory and complete settlement reports remain gated; [validation](docs/VALIDATION.md) |
| ADM-023 | Accessible independent admin UI | M1 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Auth shell is separate and staff workflows support keyboard navigation | Three isolated auth layouts; responsive themed workspace and keyboard-friendly dialogues; [validation](docs/VALIDATION.md) |
| ADM-024 | Admin E2E and visual evidence | M1 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Test real permission boundaries and capture synthetic runtime screenshots | Normal browser review and real Go/PostgreSQL suites authored; remote verification pending; [validation](docs/VALIDATION.md) |
| ADM-025 | Deployment and operator runbooks | M4 | P0 | Partial | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Verify clean deployment rollback observability and incident procedures | Non-root static image, nginx security headers, setup and rollback runbooks; production drills pending; [validation](docs/VALIDATION.md) |
| ADM-026 | Approved business and growth controls | M5 | P1 | Deferred | [Specification](../devdocs/AdminDashboard/00-INDEX.md) | Decompose new products with staff finance risk and support ownership | Specialised products visible as unavailable boundaries, not falsely enabled business or growth controls; [validation](docs/VALIDATION.md) |
<!-- FEATURES:END -->

## Done, pending and blocked
Runnable staff application and supporting operational APIs are implemented. The [workflow specification](../devdocs/AdminDashboard/03-WORKFLOWS.md) explains which actions persist, require another operator, or only inspect evidence. The [pending-work register](../devdocs/AdminDashboard/07-PENDING-WORK.md) identifies external settlement, refunds, future-effective pricing, growth products, notification activation and production qualification; they are not replaced with invented successful actions.

## Tests and evidence
[Validation record](docs/VALIDATION.md) separates local frontend tests, actual PostgreSQL tests, normal browser journeys, API browser journeys, container checks and in-person acceptance. Local browser HTTP navigation is blocked by the environment policy; no policy bypass or mock browser result substitutes for CI. Browser traces/videos are disabled to avoid recording ephemeral staff credentials and evidence. Reviewed screenshots use synthetic records only.

## Security and permissions
Staff/customer origins and cookies are separate. Access/refresh cookies are HttpOnly and SameSite Strict; Secure + __Host prefix are enforced outside local mode. CSRF and origin are required for staff cookie mutations. A current enrolled MFA session is required for operational reads; mutations need a recent login or replay-protected password/TOTP elevation. Staff invitation, role/status/recovery, product controls and payment resumption require distinct eligible operators. Revoked sessions and stale target versions fail transactionally. Recovery does not restore a suspended account. Audit events never contain passwords, recovery tokens, decrypted documents or electricity tokens.

## Operations, deployment and troubleshooting
[Runbook](../devdocs/AdminDashboard/06-OPERATIONS.md) covers bootstrap, deployment, migration 3, configuration, rollback, queues, API failures, approvals and investigations. `Dockerfile` builds API mode only and serves the app as a non-root nginx user. Replace the nginx CSP example API origin with the same approved HTTPS API origin used at build time. Select reviewed immutable image digests before production. No frontend service worker or offline private-response cache is installed.

## Roadmap, limitations and maintenance
The console manages the backend's actual capabilities. It does not certify QPay settlement, ledger custody, a funding provider, malware-scanner production acceptance, live Novu delivery, regulatory reports or loan/card/savings products. CSV exports fail above 5,000 matching rows, imports cap at 2,000; list sort applies only to the current 50-row page in descending reference pagination. Related evidence has documented bounds. Read [pending work](../devdocs/AdminDashboard/07-PENDING-WORK.md) before expanding a module.

Every source change must update this README, its canonical task register, affected API contracts/tests, changed runtime captures and the root README. `make docs-check` and `make docs-test` are required. Do not mark a row Implemented merely because a screenshot exists.

## Changelog
- 2026-09-24 — v0.7 runnable admin console and additive backend operations; source, contract, security, workflow and test documentation delivered together. Verification evidence is maintained separately.

Admin browser qualification: shared required-field markers are decorative and no longer modify label text; exact-label and full-reload synthetic-session checks are included. PostgreSQL race, existing WebApp and documentation checks passed on `abc1e494`; admin browser acceptance remains pending.
