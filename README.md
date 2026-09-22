# Qpay-Fintech

**Approved documentation and scaffold v0.2 — 22 September 2026.** Four-application fintech product plan for bill payments, funds transfers and approved expansion. The stack and integration direction are approved. This is a documentation/scaffold baseline, not a runnable product or a production-ready release. All application task rows are Planned. No app build, provider acceptance, deployment or live payment is claimed. Repository publication and CI evidence are recorded separately from product readiness.

## Product and operating scope
Initial product-planning scope: Nigeria, NGN and adult individual customers targeting a partner-backed operating model. This is a planning baseline, not regulatory approval; legal entity, partner agreements and operating permissions still require verification. Bills and transfers must include the ledger, recovery, reconciliation, risk, support and operational controls that make them usable end to end. Growth and separately regulated products require explicit approval.

## Applications and approved stacks
| Application | Responsibility | Approved option | Canonical register |
|---|---|---|---|
| APIbackend | Customer domain, wallet subledger, payment orchestration and financial operations | B1: Go + chi, PostgreSQL | [APIbackend README](APIbackend/README.md) |
| MobileApp | Android and iOS customer application | M1: React Native + TypeScript + Expo development builds | [MobileApp README](MobileApp/README.md) |
| AdminDashboard | Staff operations, finance, support and independent approvals | A1: React + TypeScript + Vite + React Router | [AdminDashboard README](AdminDashboard/README.md) |
| WebApp | Responsive authenticated customer dashboard | W1: React + TypeScript + Vite + React Router | [WebApp README](WebApp/README.md) |

The considered Flutter, native, Gin, Laravel, Next.js and SvelteKit alternatives remain as history in [decisions and stacks](devdocs/project/02-DECISIONS-AND-STACKS.md). Approved integration E1 consumes QPay as a server-side upstream; it does not share financial databases or assume upstream readiness.

## Architecture and integration
Read the [target architecture and diagram](devdocs/project/03-ARCHITECTURE.md), [QPay integration analysis](devdocs/project/04-QPAY-INTEGRATION.md), [ledger invariants](devdocs/project/05-DATA-AND-LEDGER.md) and [API contract plan](devdocs/project/06-API-CONTRACT-PLAN.md). One authority owns this product's customer ledger. Clients and admin views cannot edit balances directly. Transport timeout is not financial failure and must not trigger another payout.

## Repository layout
```text
APIbackend/       Go API, worker, scheduler, internal domains, migrations and tests
MobileApp/        React Native source, tests and independent documentation
AdminDashboard/   Bespoke staff web source, tests and independent documentation
WebApp/           Customer web source, tests and independent documentation
contracts/        OpenAPI, events and synthetic contract-fixture workspaces
devdocs/          Shared plans, service indexes, ADRs and delivery templates
docs/diagrams/    Canonical target-design source and rendered SVG images
infra/            Docker, observability and operational-runbook workspaces
tests/acceptance/ Cross-service acceptance workspace
scripts/docs/     Executable documentation checks and regression tests
.github/          Read-only CI, ownership and contribution templates
```
These are tracked documentation/scaffold workspaces, not runnable services.

## Images and visual evidence

![Approved target architecture; not implemented](docs/diagrams/system.svg)
No real product screenshots exist because these applications are not implemented. Architecture diagrams are target-state source, not deployment evidence. Follow the [screen and provenance plan](devdocs/project/09-UX-AND-SCREEN-PLAN.md): capture actual running screens with synthetic data, source commit, route, device/viewport, timestamp and hash. Never present a design mockup or provider logo as integration proof.

## Cross-service progress
The following derived summary counts service task rows. These are not unique end-user features or a production-ready percentage. Growth epics require decomposition and approval.

<!-- PROGRESS:START -->
| Service | Total tasks | Planned | In progress | Partial | Implemented | Blocked | Unverified | Deferred |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| [APIbackend](APIbackend/README.md) | 42 | 42 | 0 | 0 | 0 | 0 | 0 | 0 |
| [MobileApp](MobileApp/README.md) | 25 | 25 | 0 | 0 | 0 | 0 | 0 | 0 |
| [AdminDashboard](AdminDashboard/README.md) | 26 | 26 | 0 | 0 | 0 | 0 | 0 | 0 |
| [WebApp](WebApp/README.md) | 21 | 21 | 0 | 0 | 0 | 0 | 0 | 0 |
<!-- PROGRESS:END -->

See [documentation task progress](devdocs/project/DOCUMENTATION-PROGRESS.md) for completed foundation work and remaining specification gates.

Documentation baseline: shared plans, accepted ADRs, service README registers, repository structure, governance, templates and executable consistency checks. Functional features implemented: none. Product tests run: none. Provider acceptance: not assessed. Deployment: not performed. See [validation report](devdocs/project/VALIDATION-REPORT.md) for the exact scope of documentation-only checks.

## Documentation and open decisions
Start with [devdocs/00-INDEX.md](devdocs/00-INDEX.md). The [full document inventory](devdocs/project/17-DOCUMENT-INVENTORY.md) distinguishes drafted planning documents from detailed specifications still to write. Open launch decisions include operating authority/custody, provider contracts, hostnames, production infrastructure, theme counts and release objectives. Stack and E1 integration are accepted in [ADR-0001](devdocs/adrs/0001-APPROVED-ARCHITECTURE.md).

## Roadmap and task tracking
[M0–M6 roadmap](devdocs/project/12-ROADMAP.md): decisions, foundations, an end-to-end transfer slice, bill operations, release acceptance, approved growth and separately gated products. The canonical detailed registers are the four service READMEs. Derived JSON files must remain in sync. A documentation workflow and issue/PR templates are included. Product issue allocation, Projects boards and branch-protection activation remain tracked setup tasks.

## Setup and configuration
No runtime setup exists yet; do not treat these folders as runnable applications. Documentation tooling only:

```sh
python3 scripts/docs/check.py --write
python3 scripts/docs/check.py --check
python3 scripts/docs/governance.py
python3 -m unittest discover -s scripts/docs -p 'test_*.py' -v
```

The first updates generated summaries; the second checks their consistency. Read [tooling limitations](scripts/docs/README.md). Add actual application prerequisites, secure environment examples, migrations, fixtures and build commands with the first application implementation.

## Tests, CI and release readiness
Use the [acceptance plan](devdocs/project/10-TESTING-AND-ACCEPTANCE.md). Documentation generation is not a test of the financial engine. The documentation workflow is committed for push/PR checking. Application CI, provider acceptance and required-check repository rules remain separate gates. Every implementation report must state exact commit, commands, environment, passes, failures and skips. Provider/live financial and independent recovery acceptance are separate gates.

## Security and contributions
[Security policy](SECURITY.md) · [Contributing](CONTRIBUTING.md) · [Agent rules](AGENTS.md). Never publish secrets or real customer/KYC/payment data. Staff and customer sessions are independent. Read [security/compliance](devdocs/project/07-SECURITY-AND-COMPLIANCE.md) and [operations](devdocs/project/11-OPERATIONS-AND-DEPLOYMENT.md) before live activation.

## Limitations and launch blockers
Launch blockers: legal/custody approval, provider agreements, detailed executable contracts/schema, application implementation, independent security review, financial reconciliation qualification, off-host recovery, signed app distribution and operating support. No completed QPay features are inherited by copying their names.

## Changelog
[Project changelog](CHANGELOG.md). 2026-09-22: recommended stack and E1 approved; documentation, ADRs, repository structure and validation workflow established. See the changelog and handover for the exact scope.
