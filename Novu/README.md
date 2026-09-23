# Novu — Qpay-Fintech notifications

**Approved stack:** self-hosted Novu, with repository-owned email content, templates, schemas and code-workflow definitions under `Novu/`. Portable content uses dependency-free Node.js 22+ ESM; the production TypeScript/Novu Framework host and exact server/SDK versions require qualification. The previously proposed `Notifications/` directory is superseded.

**Status — 22 September 2026:** 175 email scenarios and six built-in themes render locally. The workflow factory passes a contract harness, not a real Novu SDK integration test. **Zero live workflows are enabled; no Novu instance, provider, production bridge or email delivery has been activated.** Community is the planning edition baseline; do not assume Cloud or paid Enterprise capabilities.

## Purpose and boundaries
Own deterministic email content, payload schemas, workflow code, theme rendering, deployment/import documentation and notification-specific acceptance. Go owns business facts, identity, financial state, notification intent, recipient eligibility, consent, suppression and audit. No email, callback, link or delivery failure can approve, reverse or complete a payment. See [accepted ADR](../devdocs/adrs/0003-SELF-HOSTED-NOVU.md) and [specification index](../devdocs/Novu/00-INDEX.md).

## Architecture and visuals
```text
Go transaction + durable intent -> Go dispatch -> self-hosted Novu
                                                -> signed bridge -> email provider
Provider callbacks -> Go delivery records / suppression / staff investigation
Independent incident path -> on-call (must survive Novu failure)
```
Self-hosted infrastructure includes the qualified Novu API, worker, WebSocket service, dashboard, MongoDB, Redis, private bridge and any version-required storage. Use isolated installations when Community environment or administration features are insufficient. Do not expose databases or the staff dashboard publicly. [Architecture](../devdocs/Novu/01-SELF-HOSTED-ARCHITECTURE.md).

![Six rendered built-in email themes using synthetic fixtures; browser preview, not a Novu deployment](docs/email-themes.webp)
[Preview provenance](docs/PREVIEWS.json) records the actual local renderer and catalogue hashes. These are browser content previews, **not delivered-email or live Novu screenshots**. The runtime [screenshot manifest](docs/SCREENSHOTS.json) remains uncaptured.

## Setup, configuration and commands
Node.js 22+ is sufficient for offline content work. There are no external npm dependencies in this content package and no install step is needed. From the repository root:
```sh
npm --prefix Novu test
npm --prefix Novu run build
npm --prefix Novu run check
make docs-check
make docs-test
```
Build generates ignored `Novu/dist/`: one HTML, text, schema and synthetic fixture per workflow, a machine-readable catalogue and preview index. Open the generated index locally. Do not expose synthetic challenge codes as a production feature.

For real hosting, follow [deployment and import](../devdocs/Novu/05-DEPLOYMENT-AND-IMPORT.md), [deployment workspace](deploy/README.md) and [.env.example](.env.example). Secrets, production sender identity, exact image digests and target release are intentionally absent. There is **no executable production Compose stack or HTTP bridge yet**; the deployment plan explicitly tracks these next implementation tasks.

## Source and API map
| Path | Responsibility |
|---|---|
| [catalogue/emails.psv](catalogue/emails.psv) | Canonical 175-event catalogue: audience, scope, class, theme, guard and real copy. |
| [src/content.mjs](src/content.mjs) | Six themes, validation, payload schemas, synthetic fixtures, safe HTML and plain text. |
| [bridge/workflows.mjs](bridge/workflows.mjs) | Importable factory accepting the real Framework API and mandatory trusted adapter seams. |
| [bridge/README.md](bridge/README.md) | Production bindings, signatures, provider overrides and limitations. |
| [scripts/build.mjs](scripts/build.mjs) | Reproducible content/schema/fixture export and summary consistency check. |
| [tests/content.test.mjs](tests/content.test.mjs) | Offline renderer and workflow-contract tests. |
| [deploy/compatibility.json](deploy/compatibility.json) | Unqualified target-version register and initially empty workflow allowlist. |
| [shared contract index](../contracts/notifications/README.md) | Notification envelope and future Go API contract boundary. |

No customer or admin API endpoint is implemented in this directory. The canonical workflow ID is `qpf-email-<catalogue-key>-v1`. Schemas reject unknown fields; routes built by templates are proposed authenticated application routes, not currently implemented screens.

## Feature and task register
Status measures the named task, not end-to-end product readiness. Content implementation is separate from Framework, server, provider and release acceptance. Each financial feature milestone must also implement its notification producer, guard and delivery/recovery tests.

<!-- FEATURES:START -->
| ID | Capability | Milestone | Priority | Status | Specification | Acceptance | Evidence |
|---|---|---|---|---|---|---|---|
| NOT-001 | Self-hosted architecture and operating specifications | M0 | P0 | Implemented | [Spec](../devdocs/Novu/01-SELF-HOSTED-ARCHITECTURE.md) | Ownership, edition gaps, private hosting and operations documented | devdocs/Novu/01–08; accepted ADR-0003 |
| NOT-002 | End-to-end email catalogue and deterministic identifiers | M0 | P0 | Implemented | [Spec](../devdocs/Novu/02-EMAIL-CATALOGUE.md) | 175 unique copy-bearing scenarios and synthetic fixtures validate | catalogue/emails.psv; content tests; docs/CONTENT-SUMMARY.json |
| NOT-003 | Six built-in email themes and safe renderer | M0 | P0 | Implemented | [Spec](../devdocs/Novu/03-THEMES-AND-CONTENT.md) | Escaped HTML and plain text render with approved theme and bounded values | src/content.mjs; six-theme browser preview; renderer tests |
| NOT-004 | Per-workflow payload schemas and portable content bundle | M0 | P0 | Implemented | [Spec](../devdocs/Novu/04-CONTRACTS-AND-DELIVERY.md) | Export schemas, fixtures, HTML and text without credentials or network | scripts/build.mjs; build and --check executed |
| NOT-005 | Offline content and workflow-contract regression suite | M0 | P0 | Implemented | [Spec](../devdocs/Novu/07-TESTING-AND-ROADMAP.md) | Positive and adversarial catalogue, render and factory cases pass | tests/content.test.mjs; 207 Node tests passed |
| NOT-006 | Version-controlled Framework workflow factory | M1 | P0 | Partial | [Spec](../devdocs/Novu/04-CONTRACTS-AND-DELIVERY.md) | Factory must execute with the selected real SDK, bridge and provider mapping | bridge/workflows.mjs; contract harness passes; real SDK acceptance pending |
| NOT-007 | Release, edition, SDK and container compatibility lock | M0 | P0 | Planned | [Spec](../devdocs/Novu/05-DEPLOYMENT-AND-IMPORT.md) | Exact releases, image digests, licences and compatibility tests recorded | deploy/compatibility.json is unqualified |
| NOT-008 | Private self-hosted infrastructure and TLS | M1 | P0 | Planned | [Spec](../devdocs/Novu/01-SELF-HOSTED-ARCHITECTURE.md) | Qualified manifests provision isolated test and production dependencies | Hostnames, manifests, resources and deployment evidence pending |
| NOT-009 | Signed production bridge and policy/provider adapters | M1 | P0 | Planned | [Spec](../devdocs/Novu/04-CONTRACTS-AND-DELIVERY.md) | Real SDK host rejects forged requests and binds trusted policy/provider adapters | HTTP host, signatures and real provider overrides pending |
| NOT-010 | Clean-environment workflow sync and safe migration | M1 | P0 | Planned | [Spec](../devdocs/Novu/05-DEPLOYMENT-AND-IMPORT.md) | Reviewed allowlist syncs reproducibly; collisions and rollback tested | No live workflows synced; allowlist empty |
| NOT-011 | Go transactional outbox and duplicate-safe dispatch | M1 | P0 | Planned | [Spec](../devdocs/Novu/04-CONTRACTS-AND-DELIVERY.md) | Atomic intent, retries, unknown submissions, leases and final-state checks tested | Depends on API-012 and API-029; Go integration pending |
| NOT-012 | Authoritative subscriber, contact and preference policy | M1 | P0 | Planned | [Spec](../devdocs/Novu/02-EMAIL-CATALOGUE.md) | Recipient ownership, old/new contacts, tenancy and preferences enforced | Trusted subscriber provisioning and guard adapters pending |
| NOT-013 | Secure verification-code delivery qualification | M1 | P0 | Planned | [Spec](../devdocs/Novu/06-SECURITY-AND-EMAIL-OPERATIONS.md) | Go expiry/attempt limits, redaction and queue-expiry rules proven | Three code templates built; no live verification transport approved |
| NOT-014 | Sender identity, DNS and provider acceptance | M2 | P0 | Planned | [Spec](../devdocs/Novu/06-SECURITY-AND-EMAIL-OPERATIONS.md) | Verified sender, SPF/DKIM/DMARC/TLS and provider contract qualify | Provider selection and legal/sending identity pending |
| NOT-015 | Plain-text MIME and unsubscribe-header provider mapping | M2 | P0 | Planned | [Spec](../devdocs/Novu/03-THEMES-AND-CONTENT.md) | Actual received MIME and one-click headers meet selected provider contract | Text/header output built; provider overrides and endpoints pending |
| NOT-016 | Delivery callbacks, correlation and suppression | M2 | P0 | Planned | [Spec](../devdocs/Novu/04-CONTRACTS-AND-DELIVERY.md) | Signed callbacks handle duplicates, bounces, complaints and unmatched events | Independent Go delivery telemetry and suppression store pending |
| NOT-017 | Customer funding, transfers and bills end-to-end delivery | M2 | P0 | Planned | [Spec](../devdocs/Novu/02-EMAIL-CATALOGUE.md) | Real producers and UI state match authoritative final/pending states | Built-in copy is not a connected payment feature |
| NOT-018 | Customer web, mobile and staff inbox integration | M3 | P1 | Planned | [Spec](../devdocs/Novu/01-SELF-HOSTED-ARCHITECTURE.md) | Scoped inbox clients use self-hosted URLs and secure subscriber sessions | Email scope delivered here; inbox UI workflows and SDK acceptance pending |
| NOT-019 | Support replies, disputes and inbound email ingestion | M3 | P1 | Planned | [Spec](../devdocs/Novu/06-SECURITY-AND-EMAIL-OPERATIONS.md) | Conversation correlation, scanning, permission checks and loop prevention pass | Inbound receiver and support APIs pending |
| NOT-020 | Staff approvals and finance/risk operation workflows | M3 | P0 | Planned | [Spec](../devdocs/Novu/02-EMAIL-CATALOGUE.md) | Staff audiences and independent approvals have real producer/policy tests | Operation copy built; staff and financial integrations pending |
| NOT-021 | Independent monitoring, investigation and replay controls | M3 | P0 | Planned | [Spec](../devdocs/Novu/06-SECURITY-AND-EMAIL-OPERATIONS.md) | Out-of-band alerts, redacted timeline and audited safe replay exercised | No alert destination, monitoring or operator console deployed |
| NOT-022 | Backup, restore, rotation and upgrade qualification | M4 | P0 | Planned | [Spec](../devdocs/Novu/05-DEPLOYMENT-AND-IMPORT.md) | Independent restore recovers keys, workflow versions and unresolved sends | RPO/RTO, storage policies and recovery drill pending |
| NOT-023 | Email-client accessibility and visual acceptance | M4 | P1 | Planned | [Spec](../devdocs/Novu/03-THEMES-AND-CONTENT.md) | Gmail, Outlook, Apple Mail and mobile/dark-mode matrix reviewed | Browser content previews only; received-email matrix pending |
| NOT-024 | Notification production release acceptance | M4 | P0 | Planned | [Spec](../devdocs/Novu/07-TESTING-AND-ROADMAP.md) | All target-specific security, provider, delivery and recovery gates close | Live deployment and production acceptance have not occurred |
| NOT-025 | Approved growth notification rollout | M5 | P1 | Planned | [Spec](../devdocs/Novu/02-EMAIL-CATALOGUE.md) | Product approval, consent, pricing and every growth guard qualify | 20 growth scenarios built; excluded from default approved scope |
| NOT-026 | Separately regulated product notification rollout | M6 | P2 | Deferred | [Spec](../devdocs/Novu/02-EMAIL-CATALOGUE.md) | Product-specific legal and provider approval precedes integration | 10 regulated scenarios built for planning; no product approval assumed |
| NOT-027 | README, task-register and notification CI integration | M0 | P0 | Implemented | [Spec](../devdocs/Novu/07-TESTING-AND-ROADMAP.md) | Fifth service register, root summary, local tests and read-only CI tracked | scripts/docs; .github/workflows/documentation.yml; validation report |
<!-- FEATURES:END -->

## Done, pending and blocked
**Built locally:** 175 subject/body definitions (145 core, 20 growth, 10 regulated), six theme variants, per-workflow strict schemas, synthetic fixtures, HTML/text export, opt-out header values, factory contract seam and offline regression suite. Core does not mean automatically enabled: the production allowlist is empty. Growth and regulated rows do not grant product approval.

**Partial:** code-first factory integration. **Pending:** real Framework host/signatures, provider MIME mapping, infrastructure, actual sync, Go outbox and subscriber policy, callbacks, client inboxes, sender qualification, live screenshots, recovery and release. Exact hostnames, target Novu version, legal sender information and provider account selection are required for deployment, not for local content development.

## Tests and evidence
[Validation report](docs/VALIDATION.md) records executed commands and limitations. There are 207 passing Node content/contract tests. They do not run MongoDB, Redis, Novu, the real Framework SDK, SMTP, DNS or a financial engine. The repository CI validates documentation and renders/tests content without notification secrets or network delivery. Real received-email MIME, signature, callback, concurrency and email-client acceptance remain required.

## Security and permissions
Never pass balances, KYC records, passwords, transaction PINs or investigation details into general templates. Go must authenticate recipients and staff scopes; catalogue guards are policy requirements, not proof that business state is true. Keep code challenges disabled until logging/retention and queue-expiry handling qualify. Mandatory security notices do not bypass hard-bounce/complaint suppression. Links are not bearer authorisation for account or financial actions. Email scanner GET requests must not mutate state. See [security and operations](../devdocs/Novu/06-SECURITY-AND-EMAIL-OPERATIONS.md).

## Operations, deployment and troubleshooting
Production release means deploying a pinned, signed bridge plus syncing an approved workflow allowlist to the exact self-hosted API, separately configuring providers, and passing recipient/delivery checks. It is not a standalone JSON upload. Environment credentials, subscribers, preferences and history are not implicitly migrated by workflow sync. Store the encryption key with recoverable secrets; keep isolated backups and an independent incident channel. Community delivery observability must be provided by our own provider-to-Go callback path. No blind provider failover. [Runbooks and import plan](../devdocs/Novu/05-DEPLOYMENT-AND-IMPORT.md).

## Roadmap, limitations and maintenance
[Testing and roadmap](../devdocs/Novu/07-TESTING-AND-ROADMAP.md) maps M0–M6 and release gates. The [sources and compatibility record](../devdocs/Novu/08-SOURCES-AND-COMPATIBILITY.md) distinguishes official capabilities from our design defaults. Theme styles are implemented, but brand/legal copy and real email-client accessibility remain subject to review. English/en-NG and NGN are the initial content baseline; localisation requires its own fixtures and acceptance.

Source/catalogue changes require this README, the root README, relevant spec, tests and actual changed visual evidence in the same delivery. Generate `docs/FEATURES.json`; never edit it as the authority. Preserve IDs and existing event versions needed by queued operations. No dashboard-only edits or silent workflow deletion in production.

## Changelog
2026-09-22: accepted self-hosted Novu under `Novu/`; built content catalogue, themes, validation/export and partial workflow factory; documented installation, migration, delivery safety and all remaining production gates. See [changelog](CHANGELOG.md).

### Customer-parity additions — 2026-09-23
Eleven additive events cover reminders, request/share lifecycle, internal schedules, dual-mailbox verification and passkey changes. Existing workflow IDs are preserved. Go embeds the canonical catalogue with drift tests. Live deployment stays unqualified. [Producer mapping](../APIbackend/docs/PARITY-API.md). The displayed six-theme image retains its original capture provenance; it is not a preview of newly delivered emails.
