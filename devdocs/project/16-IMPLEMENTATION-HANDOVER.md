# Implementation handover — approved foundation
Date: 2026-09-22. Read root README, AGENTS, ADR-0001/0002, the affected service README and the shared financial/security plans before coding.

## Approved and not to reopen silently
B1 Go/chi/PostgreSQL in APIbackend; M1 React Native/TypeScript/Expo development builds in MobileApp; A1 bespoke React/TypeScript/Vite/React Router in AdminDashboard; W1 the same browser stack in WebApp; E1 server-to-server QPay integration. APIbackend owns this product's wallet subledger. No shared mutable financial tables, frontend provider secrets or admin SQL balance edits. Initial planning scope is Nigeria/NGN/adult individual customers, bills/transfers and supporting operations.

## Current deliverable
Documentation and directory scaffold only. Four canonical service registers retain 114 Planned tasks. Governance tooling is executable; there are no product handlers, UI screens, migrations, native builds or live integrations. Diagram images describe the approved target, not a deployment. Detailed executable OpenAPI/schema and provider acceptance remain work items.

## Next implementation sequence
1. Inspect current main and open work; never overwrite concurrent changes. Assign owners and issues to M1 tasks and maintain stable IDs.
2. Verify supported Go/TypeScript/React/Expo dependencies and selected native KYC/payment SDKs. Pin versions and lockfiles; do not infer compatibility from framework approval.
3. Finalise executable contract and physical ledger schema with explicit authorization, money representation, unique constraints, transaction isolation and state transitions. Verify current QPay APIs at a pinned revision using the capability audit.
4. Build the API/worker/scheduler foundation, secure configuration, database migrations, session boundaries, audit and durable idempotency/outbox. Execute actual PostgreSQL concurrency/recovery tests.
5. Establish all three client application shells with authentication layouts separate from authenticated navigation. Keep browser/native release processes independent.
6. Deliver the transfer vertical slice across relevant clients, operator workflows, reconciliation, monitoring, tests and screenshots; then the bill-payment slice. Keep simulators labelled.

## Blocking launch decisions
Legal entity/custody/partner authority; provider contracts and qualification; production domains/infrastructure/data residency; key/backup ownership; operating SLO/RPO/RTO; support/compliance ownership; independent security review; native distribution acceptance. These do not prevent isolated implementation but do prevent real-money launch.

## Verify every delivery
Run `make docs-check` and `make docs-test`; add actual affected application commands once applications exist. Record command, revision, environment, count and skips. Verify remote commit and GitHub job results separately. A passing docs job is not evidence for financial readiness.
