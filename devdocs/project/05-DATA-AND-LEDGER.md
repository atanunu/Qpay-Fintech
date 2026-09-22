# Data ownership and ledger requirements
Status: design requirements. Detailed ERDs, enum contracts and migrations remain to be approved and written.

## Entity catalogue
Identity: customers, contact_verifications, sessions, devices, authentication_factors, recovery_events, consents, staff, roles, permissions.
KYC: applications, checks, document_references, decisions, tier_policies, policy_versions.
Money: ledger_accounts, journals, postings, balance_projections, holds, payment_intents, authorisations, quotes, fee_components, idempotency_records, provider_attempts, provider_observations, inbox_events, outbox_events.
Products: virtual_accounts, funding_events, beneficiaries, transfers, bill_categories, billers, products, bill_validations, bill_orders, fulfilments, refunds, returns, disputes.
Operations: policy_proposals, approvals, audit_events, risk_cases, support_cases, reconciliation_runs, reconciliation_items, reconciliation_breaks, settlement_batches, provider_balances, notification_attempts, export_jobs.

This is not a complete physical schema. Per-module specifications must settle constraints, indexes, lifecycles, retention, access control, migration ownership and data lineage before code.

## Non-negotiable financial invariants
1. Store monetary amounts as integers in the currency's minor unit. Include currency everywhere. Use a lossless JSON representation and safe client parsing; never JavaScript floating-point arithmetic for ledger values. FX/rate calculations, when separately approved, need explicit decimal precision and rounding policy.
2. Every posted journal balances by currency. Postings are append-only; corrections create linked compensating entries, never edited historical balances.
3. Available funds, reservations and transaction limits must be checked and updated atomically. Define canonical lock ordering, isolation and bounded transaction retries. Cached balances are projections, not authority.
4. An idempotent intent is scoped to the authenticated actor and operation. Persist a request fingerprint; the same key with different financial details must be rejected. Keys and event-deduplication retention must outlast possible financial replay, independently of UI session expiry.
5. Transaction approval binds payer, payee, amount, currency, fees, quote version and expiry. A reused PIN or successful biometric prompt is not blanket permission for changed transaction details.
6. Record journal, hold and outbox changes atomically. Assume message delivery can repeat. Enforce one financial effect with durable uniqueness and state transitions rather than claiming the entire network is exactly-once.
7. A timeout or pending reply is not definitive failure. Preserve funds/holds according to the approved state machine until provider evidence and policy justify settlement or release.
8. Reversals, refunds and returned payments are separate operations linked to the original. Do not erase settled history or assume every rail supports cancellation.
9. Funding credits require authenticated, scoped evidence, amount/currency matching and duplicate protection. Route unmatched or conflicting receipts into investigation.
10. Reconcile ledger obligations with upstream records and bank/partner settlement. Differences remain visible in suspense with owner, reason, age and resolution evidence.

## State-machine specification
Define separate state machines for financial intent, external attempt, hold, settlement, bill fulfilment, notification, dispute and reconciliation break. Candidate intent states: created, awaiting_authorisation, accepted, processing, pending_review, succeeded, failed. These are proposals, not a published enum contract. Reversal status must not ambiguously overwrite the original success.

For every transition document initiator, guard, transaction boundary, journal/hold effects, allowed retries, evidence, emitted events and terminality. Include races between callbacks, polling and operator decisions. Staff decisions require current-state validation and independent approval where material.

## Reconciliation example
A successful upstream transfer without matching local posting is not fixed by editing balance. Investigate immutable intent/attempt data, identify whether a journal already exists, apply an approved duplicate-safe correction, preserve evidence and re-run reconciliation. A failed notification does not reverse a financially successful payment.

Use UTC storage and local-time presentation. Keep audit and financial history subject to approved retention/legal-hold rules; account closure is not an instruction to delete every regulated record.
