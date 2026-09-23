# Remaining backend work and honest readiness

This release is a substantial persistent core, not completion of all 42 programme tasks.

## Launch-blocking engineering
Partner account provisioning, owned funding instructions and verified callback/requery credit pipeline; authenticated durable provider callback inbox; complete refund/return workflows and independent compensating operations; comprehensive three-way reconciliation, suspense resolution, treasury/float/settlement; owned private document storage and scanning with KYC provider integration; transaction-specific provider fee/status/fulfilment qualification; production notification bridge, correlation/bounce/complaint callbacks and unsubscribe; complete staff invitation/recovery/contact-change/account-closure flows; automated risk controls; role-separated production database privileges; observability and independent recovery/load/security acceptance.

KYC currently accepts an evidence reference for manual review. This is not proof of identity, document ownership, liveness or regulatory eligibility. Account policy caps are not a representation of approved regulatory tiers. Reconciliation compares supplied rows and does not prove completeness of a bank statement or settlement period. CreditFunding is a tested internal primitive; a customer cannot currently obtain a real funding account through this API.

## Provider gates
QPay adapter source follows inspected interfaces, with HTTP fixtures and safe unresolved-state behaviour. No real QPay sandbox/live acceptance has been completed. Unknown operations retain their original reference; list bounds or unsupported states may require manual investigation. No provider should be activated solely by toggling an environment variable.

## Client and delivery work
Finish complete per-view schemas/typed SDK acceptance, all list pagination and async exports; integrate actual web/mobile UI and native secure-storage/deep-link tests; receive and inspect real MIME, provider callbacks and notification permissions. Existing support lacks attachments, assignment/SLA automation and inbound email processing.

## Expansion
Recurring mandates, business KYB/payroll/bulk payouts, merchant collections/QR/payment links, rewards/referrals/budgets, partner APIs and separately regulated products remain planned. They must not be advertised as live features.

## Verification/publication
The corrected suite and container smoke test must pass on the exact final revision. The working snapshot does not claim a passing final CI run or a completed push to main. Preserve the previous main until verification and documentation are reconciled.
