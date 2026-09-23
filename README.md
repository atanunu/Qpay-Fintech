# Qpay-Fintech

**Customer WebApp review candidate v0.5 — 23 September 2026.** The responsive React customer app now has API-connected journeys plus a separate synthetic review mode. The previously supplied persistent Go backend working source is reconciled for joint local testing. The API retains its documented incomplete funding, refund, reconciliation, provider and production gates. No real-money launch or live Novu/provider activation is claimed.

## Product and operating scope
Initial product-planning scope: Nigeria, NGN and adult individual customers targeting a partner-backed operating model. This is a planning baseline, not regulatory approval; legal entity, partner agreements and operating permissions still require verification. Bills and transfers must include the ledger, recovery, reconciliation, risk, support and operational controls that make them usable end to end. Growth and separately regulated products require explicit approval.

## Applications and approved stacks
| Application | Responsibility | Approved option | Canonical register |
|---|---|---|---|
| APIbackend | Customer domain, wallet subledger, payment orchestration and financial operations | B1: Go + chi, PostgreSQL | [APIbackend README](APIbackend/README.md) |
| MobileApp | Android and iOS customer application | M1: React Native + TypeScript + Expo development builds | [MobileApp README](MobileApp/README.md) |
| AdminDashboard | Staff operations, finance, support and independent approvals | A1: React + TypeScript + Vite + React Router | [AdminDashboard README](AdminDashboard/README.md) |
| WebApp | Responsive authenticated customer dashboard | W1: React + TypeScript + Vite + React Router | [WebApp README](WebApp/README.md) |
| Novu | Self-hosted notification setup, code-owned email themes and workflows | Self-hosted Novu; Node ESM content; qualified Framework host pending | [Novu README](Novu/README.md) |

The considered Flutter, native, Gin, Laravel, Next.js and SvelteKit alternatives remain as history in [decisions and stacks](devdocs/project/02-DECISIONS-AND-STACKS.md). Approved integration E1 consumes QPay as a server-side upstream; it does not share financial databases or assume upstream readiness.

## Architecture and integration
Read the [target architecture and diagram](devdocs/project/03-ARCHITECTURE.md), [QPay integration analysis](devdocs/project/04-QPAY-INTEGRATION.md), [ledger invariants](devdocs/project/05-DATA-AND-LEDGER.md) and [API contract plan](devdocs/project/06-API-CONTRACT-PLAN.md). One authority owns this product's customer ledger. Clients and admin views cannot edit balances directly. Transport timeout is not financial failure and must not trigger another payout.

## Repository layout
```text
APIbackend/       Go API, worker, scheduler, internal domains, migrations and tests
MobileApp/        React Native source, tests and independent documentation
AdminDashboard/   Bespoke staff web source, tests and independent documentation
WebApp/           Customer web source, tests and independent documentation
Novu/             Self-hosted notification specs, 164 email scenarios, six themes and tests
contracts/        OpenAPI, events and synthetic contract-fixture workspaces
devdocs/          Shared plans, service indexes, ADRs and delivery templates
docs/diagrams/    Canonical target-design source and rendered SVG images
infra/            Docker, observability and operational-runbook workspaces
tests/acceptance/ Cross-service acceptance workspace
scripts/docs/     Executable documentation checks and regression tests
.github/          Read-only CI, ownership and contribution templates
```
WebApp contains runnable customer UI, two explicit transports, tests and in-person review tools. APIbackend contains the persistent core and local test runtime, with its remaining work tracked. MobileApp and AdminDashboard remain planned scaffolds. Novu contains offline content and a partial live integration.

## Images and visual evidence

![Approved target architecture; not implemented](docs/diagrams/system.svg)
WebApp screenshots must come from the running synthetic review or isolated local API, never be labelled live-provider evidence. Its first local component-render captures and normal HTTP capture suite are being verified. Architecture diagrams are target-state source, not deployment evidence. Follow the [screen and provenance plan](devdocs/project/09-UX-AND-SCREEN-PLAN.md): capture actual running screens with synthetic data, source commit, route, device/viewport, timestamp and hash. Never present a design mockup or provider logo as integration proof.

[Built-in email-theme previews](Novu/README.md) show actual local template rendering with synthetic data, not live Novu or delivered-email screenshots. The notification architecture is an additive target boundary; the original system diagram remains a target view of the four product apps.

## Cross-service progress
The following derived summary counts service task rows. These are not unique end-user features or a production-ready percentage. Growth epics require decomposition and approval.

<!-- PROGRESS:START -->
| Service | Total tasks | Planned | In progress | Partial | Implemented | Blocked | Unverified | Deferred |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| [APIbackend](APIbackend/README.md) | 42 | 10 | 0 | 32 | 0 | 0 | 0 | 0 |
| [MobileApp](MobileApp/README.md) | 25 | 25 | 0 | 0 | 0 | 0 | 0 | 0 |
| [AdminDashboard](AdminDashboard/README.md) | 26 | 26 | 0 | 0 | 0 | 0 | 0 | 0 |
| [WebApp](WebApp/README.md) | 21 | 1 | 0 | 20 | 0 | 0 | 0 | 0 |
| [Novu](Novu/README.md) | 27 | 19 | 0 | 1 | 6 | 0 | 0 | 1 |
<!-- PROGRESS:END -->

See [documentation task progress](devdocs/project/DOCUMENTATION-PROGRESS.md) for completed foundation work and remaining specification gates.

Implementation baseline: persistent Go core; responsive React customer journeys and clearly marked API-pending drafts; explicit synthetic scenarios; 58 passing local WebApp unit/contract tests. Normal HTTP browser and real API/PostgreSQL verification are separate CI gates. Novu retains 164 content scenarios and six themes, with live bridge/provider acceptance still pending. See [validation report](devdocs/project/VALIDATION-REPORT.md) for the exact scope of documentation-only checks.

## Documentation and open decisions
Start with [devdocs/00-INDEX.md](devdocs/00-INDEX.md). The [full document inventory](devdocs/project/17-DOCUMENT-INVENTORY.md) distinguishes drafted planning documents from detailed specifications still to write. Open launch decisions include operating authority/custody, provider contracts, hostnames, production infrastructure, theme counts and release objectives. Stack and E1 integration are accepted in [ADR-0001](devdocs/adrs/0001-APPROVED-ARCHITECTURE.md).

## Roadmap and task tracking
[M0–M6 roadmap](devdocs/project/12-ROADMAP.md): decisions, foundations, an end-to-end transfer slice, bill operations, release acceptance, approved growth and separately gated products. The canonical detailed registers are the five service READMEs. Novu adds 27 tracked tasks; content-ready does not mean live-delivery-ready. Derived JSON files must remain in sync. A documentation workflow and issue/PR templates are included. Product issue allocation, Projects boards and branch-protection activation remain tracked setup tasks.

## Setup and configuration
The API and WebApp provide local review runtimes; Novu is not deployed. Start with [WebApp setup and test procedure](WebApp/README.md) and [backend operations](APIbackend/docs/OPERATIONS.md). Documentation tooling:

```sh
python3 scripts/docs/check.py --write
python3 scripts/docs/check.py --check
python3 scripts/docs/governance.py
python3 -m unittest discover -s scripts/docs -p 'test_*.py' -v
```

Offline notification content tooling (Node.js 22+, no external dependency install):

```sh
npm --prefix Novu test
npm --prefix Novu run build
npm --prefix Novu run check
```

Read the [self-hosted deployment/import plan](devdocs/Novu/05-DEPLOYMENT-AND-IMPORT.md), [complete email plan](devdocs/Novu/02-EMAIL-CATALOGUE.md), [theme rules](devdocs/Novu/03-THEMES-AND-CONTENT.md) and [notification validation](Novu/docs/VALIDATION.md). No Novu/provider secrets are required for offline checks.

The first documentation command updates generated summaries; the second checks their consistency. Read [tooling limitations](scripts/docs/README.md). Add actual application prerequisites, secure environment examples, migrations, fixtures and build commands with the first application implementation.

## Tests, CI and release readiness
Use the [acceptance plan](devdocs/project/10-TESTING-AND-ACCEPTANCE.md). Documentation generation is not a test of the financial engine. The documentation workflow is committed for push/PR checking. A separate read-only notification-content CI job renders and tests the built-in templates. The Go/PostgreSQL and WebApp unit/browser workflows are supplied. Their exact commit results, real Novu/provider acceptance and required-check repository rules remain separate gates. Every implementation report must state exact commit, commands, environment, passes, failures and skips. Provider/live financial and independent recovery acceptance are separate gates.

## Security and contributions
[Security policy](SECURITY.md) · [Contributing](CONTRIBUTING.md) · [Agent rules](AGENTS.md). Never publish secrets or real customer/KYC/payment data. Staff and customer sessions are independent. Read [security/compliance](devdocs/project/07-SECURITY-AND-COMPLIANCE.md) and [operations](devdocs/project/11-OPERATIONS-AND-DEPLOYMENT.md) before live activation.

## Limitations and launch blockers
Launch blockers: legal/custody approval, provider agreements, detailed executable contracts/schema, application implementation, independent security review, financial reconciliation qualification, off-host recovery, signed app distribution and operating support. No completed QPay features are inherited by copying their names. The 14 WebApp GAP items drive in-person review before upstream activation.

## Changelog
[Project changelog](CHANGELOG.md). 2026-09-23: WebApp and the backend review core brought together with explicit test modes, endpoint gaps and browser acceptance; 2026-09-22: self-hosted Novu accepted, email content/themes and traceable notification plan added;  recommended stack and E1 approved; documentation, ADRs, repository structure and validation workflow established. See the changelog and handover for the exact scope.

## In-person customer review
[WebApp](WebApp/README.md) · [Test procedure](WebApp/docs/IN-PERSON-TESTING.md) · [API mapping](WebApp/docs/API-MAPPING.md) · [Web verification](WebApp/docs/VALIDATION.md). Synthetic UI and actual Go/PostgreSQL tests are distinct; both remain separate from actual provider acceptance.
