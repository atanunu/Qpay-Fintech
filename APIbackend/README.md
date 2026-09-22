# APIbackend

**Status: approved documentation and scaffold.** No runnable application, real product screenshots, implementation tests, provider acceptance or deployment is delivered here. The approved stack is recorded below.

[Project](../README.md) · [Documentation](../devdocs/00-INDEX.md) · [Service specification](../devdocs/APIbackend/00-INDEX.md)


**Approved stack:** Go + chi + PostgreSQL (B1). See [ADR-0001](../devdocs/adrs/0001-APPROVED-ARCHITECTURE.md). Alternative stacks are decision history. Product features remain Planned.

## Purpose and boundaries
Go financial and product engine. Approved stack: **B1: Go + chi and PostgreSQL; approved 2026-09-22**. Financial authority, execution ownership and data boundaries are defined in [Architecture](../devdocs/project/03-ARCHITECTURE.md). Role owner: APIbackend engineering; named assignee is unassigned. Financial/security decisions require independent domain review.

## Architecture and visuals

![Approved target workflow; not a runtime screenshot](../docs/diagrams/apibackend.svg)
Use the [target architecture](../devdocs/project/03-ARCHITECTURE.md) and [UX/screen inventory](../devdocs/project/09-UX-AND-SCREEN-PLAN.md). These are designs, not screenshots of implemented services. Runtime gallery: not captured. APIbackend uses API/sequence evidence rather than a fictional GUI. Screen provenance is mandatory when executable UI exists.

## Setup, configuration and commands
No application install/build/start command exists yet. Do not invent one or present a mock as an installed product. With the first application implementation, add prerequisites, pinned versions, environment-variable names with safe examples, migrations, local fixtures, development/build/test commands and troubleshooting. Never include live keys or customer data. Documentation-only commands exist in [scripts/docs](../scripts/docs/README.md).

## Source and API map
Application source and executable contracts are not yet created. See [API contract plan](../devdocs/project/06-API-CONTRACT-PLAN.md). Publish actual source paths, entry points, module boundaries and generated-client ownership when implemented. Browser/native secrets and direct database financial mutations are forbidden.

## Feature and task register
Core scope is approved for planning; every entry below remains unimplemented. Growth rows are epics needing decomposition and explicit approval. Service role ownership applies to all rows; dependencies follow [milestones](../devdocs/project/12-ROADMAP.md) and must be expanded in issues before sprint commitment. Source/test/issue evidence is intentionally absent. Verification: Not run. Provider acceptance: Not assessed. Release: Not deployed.

<!-- FEATURES:START -->
| ID | Capability / task | Milestone | Priority | Status | Specification | Acceptance and required verification | Source / tests / issue |
|---|---|---|---|---|---|---|---|
| API-001 | Customer registration and verified contacts | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Reject duplicate and enumeration abuse without leaking identity | Not implemented; tests not run; issue not created |
| API-002 | Customer sessions and devices | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Rotate and revoke sessions with cross-device recovery tests | Not implemented; tests not run; issue not created |
| API-003 | Separate staff identity and permissions | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Reject customer tokens and unauthorised operator actions | Not implemented; tests not run; issue not created |
| API-004 | KYC cases and policy-based tiers | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Document decisions and enforce approved policy on every money action | Not implemented; tests not run; issue not created |
| API-005 | Partner account provisioning | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Recover duplicate or ambiguous provisioning without creating unrelated accounts | Not implemented; tests not run; issue not created |
| API-006 | PostgreSQL schema and migrations | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Clean setup and compatible migrations pass against real PostgreSQL | Not implemented; tests not run; issue not created |
| API-007 | Balanced append-only ledger | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Every journal balances and corrections preserve original postings | Not implemented; tests not run; issue not created |
| API-008 | Atomic available funds and holds | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Concurrent debit tests cannot overspend or double-release holds | Not implemented; tests not run; issue not created |
| API-009 | Lossless money and currency encoding | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Round-trip large minor-unit amounts across all clients without precision loss | Not implemented; tests not run; issue not created |
| API-010 | Versioned quotes fees and limits | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Reject changed or expired quotes and audit effective policy versions | Not implemented; tests not run; issue not created |
| API-011 | Durable idempotent financial intents | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Same key replays safely and changed payload conflicts | Not implemented; tests not run; issue not created |
| API-012 | Transactional outbox and workers | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Recover worker crashes without losing work or duplicating financial effect | Not implemented; tests not run; issue not created |
| API-013 | Authenticated durable callback inbox | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Reject forgery and deduplicate scoped late or reordered events | Not implemented; tests not run; issue not created |
| API-014 | Bank and virtual-account funding | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Verified credits reconcile and unmatched receipts enter investigation | Not implemented; tests not run; issue not created |
| API-015 | Bank directory and account-name enquiry | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Use authoritative enquiry and control enumeration and stale results | Not implemented; tests not run; issue not created |
| API-016 | Persistent scoped beneficiaries | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Cross-customer access fails and edited beneficiaries require valid reauthorisation | Not implemented; tests not run; issue not created |
| API-017 | Internal wallet transfers | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Both sides post atomically with correct authorisation and limits | Not implemented; tests not run; issue not created |
| API-018 | External bank transfers | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Binding quote and durable intent produce a queryable supported result | Not implemented; tests not run; issue not created |
| API-019 | Ambiguous-operation recovery | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Timeout followed by late success never triggers a second payout | Not implemented; tests not run; issue not created |
| API-020 | Canonical biller and product catalogue | M3 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Version and map upstream products without duplicate or unavailable offerings | Not implemented; tests not run; issue not created |
| API-021 | Bill validation and authoritative quote | M3 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Require applicable customer validation and reject stale product pricing | Not implemented; tests not run; issue not created |
| API-022 | Bill vend and token recovery | M3 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Track financial outcome separately from token delivery and recover delayed fulfilment | Not implemented; tests not run; issue not created |
| API-023 | Refunds returns and disputes | M3 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Apply supported linked compensating operations without erasing history | Not implemented; tests not run; issue not created |
| API-024 | Three-way reconciliation | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Match product ledger upstream and settlement evidence and expose aged breaks | Not implemented; tests not run; issue not created |
| API-025 | Provider float and settlement | M4 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Detect low float and reconcile settlement and fee obligations | Not implemented; tests not run; issue not created |
| API-026 | Risk rules and account restrictions | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Enforce server-side restrictions with reason codes and audit | Not implemented; tests not run; issue not created |
| API-027 | Audit and independent approval engine | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Reject self-approval and preserve immutable evidence for privileged changes | Not implemented; tests not run; issue not created |
| API-028 | History statements and receipts | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Export scoped authoritative transactions without false success receipts | Not implemented; tests not run; issue not created |
| API-029 | Notification delivery lifecycle | M2 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Retry and track channel failures without altering financial outcome | Not implemented; tests not run; issue not created |
| API-030 | Support and complaints API | M3 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Link cases to scoped operations with escalation and resolution history | Not implemented; tests not run; issue not created |
| API-031 | Private documents and retention | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Expire signed access and enforce approved retention and legal holds | Not implemented; tests not run; issue not created |
| API-032 | OpenAPI schemas and generated clients | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Validate contracts and detect breaking client drift | Not implemented; tests not run; issue not created |
| API-033 | Metrics logs and traces | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Correlate intent and provider events while redacting sensitive data | Not implemented; tests not run; issue not created |
| API-034 | Distributed abuse controls | M1 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Limits remain effective across instances and identity enquiry channels | Not implemented; tests not run; issue not created |
| API-035 | Key rotation and off-host recovery | M4 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Restore keys and data independently and reconcile pending operations | Not implemented; tests not run; issue not created |
| API-036 | Load concurrency and release evidence | M4 | P0 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Execute financial failure suites and meet approved operating objectives | Not implemented; tests not run; issue not created |
| API-037 | Recurring mandates and schedules | M5 | P1 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Decompose epic and prove explicit consent cancellation and duplicate-safe execution | Not implemented; tests not run; issue not created |
| API-038 | Business KYB bulk payout and payroll | M5 | P1 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Decompose epic with organisation boundaries approvals and per-item outcomes | Not implemented; tests not run; issue not created |
| API-039 | Merchant QR links and collections | M5 | P1 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Decompose epic with settlement refunds expiry and merchant ownership | Not implemented; tests not run; issue not created |
| API-040 | Rewards referrals and budgeting | M5 | P2 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Decompose epic with reward liability fraud rules and customer disclosure | Not implemented; tests not run; issue not created |
| API-041 | Partner APIs and outbound webhooks | M5 | P2 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Decompose epic with scoped credentials signed events and developer acceptance | Not implemented; tests not run; issue not created |
| API-042 | Regulated expansion product families | M6 | P2 | Planned | [Specification](../devdocs/APIbackend/00-INDEX.md) | Split each product into separately approved legal financial and technical work | Not implemented; tests not run; issue not created |
<!-- FEATURES:END -->

## Done, pending and blocked
Documentation: this planning README and task register are drafted. Product implementation: zero tasks completed. Pending: 42 planned rows. Decisions: stack and E1 integration approved; contracts, operating authority and provider acceptance remain unresolved. Do not confuse these programme gates with a running system failure. Link named blockers and dependencies as implementation begins.

## Tests and evidence
Product tests: not written or run. [Acceptance plan](../devdocs/project/10-TESTING-AND-ACCEPTANCE.md) defines required suites. Record commit, exact commands, environment, totals and skips; source-discovered tests are not executed results. No financial or security certification is implied by documentation checks.

## Security and permissions
Adopt the [security plan](../devdocs/project/07-SECURITY-AND-COMPLIANCE.md). Separate customer and staff authentication. Enforce API authorisation, object ownership, sensitive-data controls and audited privileged operations. Document a detailed permission matrix before feature implementation.

## Operations, deployment and troubleshooting
Not deployed. Use the [operations plan](../devdocs/project/11-OPERATIONS-AND-DEPLOYMENT.md) to create service-specific release, rollback, logging, incident, key-recovery and data-recovery procedures. Do not modify production to create documentation evidence.

## Roadmap, limitations and maintenance
See [roadmap](../devdocs/project/12-ROADMAP.md). This README is the canonical detailed register; generated FEATURES.json is derived. Root and service statuses, tests, API docs and changed UI captures must accompany every feature delivery. See the [mandatory standard](../devdocs/project/13-DOCUMENTATION-STANDARD.md).

## Changelog
2026-09-22: service boundary, approved stack, acceptance tasks and mandatory documentation obligations recorded. Approved stack recorded and documentation/scaffold prepared for GitHub; no product implementation claimed.
