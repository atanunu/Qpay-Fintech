# Complete documentation programme and current inventory
This inventory distinguishes the delivered design baseline from detailed executable specifications and runtime evidence still to be produced. A future path is not counted as an existing implementation.

## Delivered foundation
Root README, AGENTS, CONTRIBUTING, SECURITY and CHANGELOG; four independent service READMEs and canonical feature registers; generated JSON summaries; service indexes and tracked source/test/migration/process workspaces; shared product, decisions, architecture, QPay integration, ledger, API, security, provider, UX, testing, operations, roadmap and risk plans. Accepted ADRs record B1/M1/A1/W1/E1 and mandatory documentation discipline.

Also supplied: implementation handover; permission matrix; configuration/secrets plan; release checklist; repository-settings plan; contracts and infrastructure workspaces; issue/PR/ADR/test/capture templates; CODEOWNERS; a read-only documentation workflow; regression tests; five rendered target-design diagrams; explicit empty runtime-capture manifests. See [shared index](../00-INDEX.md) and [documentation progress](DOCUMENTATION-PROGRESS.md).

## Required domain specifications in M0/M1
IdentityDev, KYCDev, WalletDev, FundingDev, TransferDev, BillPaymentDev, ReconciliationDev, TreasuryDev, RiskDev, NotificationDev, SupportDev, AdminDev and CustomerExperienceDev. BusinessDev, RecurringDev, MerchantDev and RegulatedProductsDev are expansion designs requiring product approval. Shared plans are not a substitute for reviewed physical schemas and executable contracts.

Each approved domain must be decomposed in the QPay style:
| File | Required detail |
|---|---|
| 00-INDEX.md | Purpose, boundaries, glossary, ownership and read/build order |
| 01-DATA-MODEL.md | ERD, entities, exact states, constraints, indexes, ownership and retention |
| 02-API.md | Executable schemas, auth, examples, errors, idempotency, callbacks and compatibility |
| 03-WORKFLOWS-AND-CONTROL.md | Customer/operator journeys, approval and failure handling |
| 04-PROVIDER-INTEGRATION.md | Capabilities, mapping, status queries, retries and qualification |
| 05-REPORTING-RECONCILIATION.md | Ledger effects, settlement, reports, breaks and audit |
| 06-NON-FUNCTIONAL.md | Measurable security, resilience, performance, accessibility and tests |
| 07-PENDING-WORK.md | Engineering/vendor/human tasks, accountable owners, dependencies and acceptance |
| 08-SECURITY-COMPLIANCE-READINESS.md | Threats, privacy, operating permissions and release evidence |

## Not yet delivered or accepted
Executable OpenAPI/event schemas and generated clients; physical database schemas/migrations; application manifests/runtime code; detailed provider qualification at a pinned QPay revision; native SDK proof; product tests and runtime screenshots; deployed runbooks and independent recovery evidence; regulatory/partner approval; production release. The repository-rule plan does not activate branch protection or create 114 issues/Projects cards. Exact tasks remain visible in service registers and release gates.
