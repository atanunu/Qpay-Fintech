# Dependency-led roadmap
Status: proposed milestones, not delivery-date or completion promises.

| Milestone | Work | Exit evidence |
|---|---|---|
| M0 decisions/specification | Approve market/operating model, stack, QPay boundary, SDK proof, domain model, contracts, UX and documentation policy | ADRs signed; major integration risks surfaced; CI enforcement work scoped |
| M1 foundations | Four app foundations, identity boundaries, ledger invariants, database/migrations, idempotency, audit, outbox, secure config, automated tests | Clean setup works; real database concurrency tests pass; documentation/checks run |
| M2 end-to-end transfers | KYC/account/funding, beneficiaries, fee quote, internal/bank transfers, mobile/web journeys, admin/risk, early reconciliation | One full vertical slice including timeouts, restart recovery and reconciled evidence in test/staging |
| M3 bills and service operations | Catalogue, validation, vend, fulfilment/token recovery, receipts, support, disputes, notifications and finance views | Actual bill lifecycle and failure handling qualified; no mock-only completion |
| M4 release acceptance | Provider and financial sign-off, legal approval, security remediation, operational load/restore, signed native builds, store/deployment readiness | All launch blockers closed with evidence; independent release approval |
| M5 approved expansion | Recurring mandates, KYB/payroll, merchant collections, budgets/referrals and partner APIs | Each epic separately decomposed, tested and commercially accepted |
| M6 separately gated products | Cards, credit, yield, FX/remittance, insurance, investments, agents and other licensed/partner-led products | Product-specific operating permissions and full acceptance |

M2/M3 can be built against documented simulators before live approval, but simulator completion is not partner acceptance. Business, compliance and vendor tasks progress alongside engineering and can still block release. No unapproved provider activation is implied by these milestones.

The initial 114 service-level task rows are a planning decomposition, not 114 unique end-user features or an exhaustive final backlog. Some growth rows are epics. Preserve IDs when splitting them into child tasks; link parent/child relationships and exclude overlap from aggregate counts.

## Cross-service delivery rule
A milestone is not complete because the API exists while mobile/admin/web remain mock screens. Each vertical slice must include relevant client flows, operator exception handling, accounting, monitoring, support, tests, documentation and visual evidence. Prioritise correctness over feature counts.
