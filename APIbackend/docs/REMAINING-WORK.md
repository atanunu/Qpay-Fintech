# Remaining work and release gates — v0.6

## Delivered components
Migration two supplies owned private uploads, a scanner/store adapter, resumable/immutable manual identity review, dual-mailbox changes, guarded account closure, passkeys, saved bills, budgets/insights, requests, reminders and bounded internal mandates. Do not continue reporting these as missing source. Their actual deployment qualification remains separate.

## Financial and partner engineering still incomplete
The funding provisioning/verified-credit interface and persistent lifecycle exist, but only a local synthetic adapter is available. Implement and qualify the actual QPay partner adapter before real funding. Complete provider callbacks, refund/return compensation, three-way reconciliation, suspense management, treasury/float/settlement, and supported provider fees/states/value retrieval. Missing provider outcomes retain original references; they are not permission to resubmit.

## Identity, privacy and service operations
Automatic identity-provider verification/liveness and phone-assisted camera handoff are not implemented. Private S3/ClamAV adapters need actual deployment, ACL/public-access-block, key custody, encrypted backup, scanner update, retention and orphan-cleanup acceptance. Verified SMS/phone changes are unavailable. Current profile export is deliberately limited and is not a complete all-record privacy-access response. Larger financial exports need asynchronous jobs and complete pagination.

Support attachments/escalation/events exist; complete staff assignment, SLA automation, retention and inbound email do not. Staff invitation/recovery hardening, automated risk rules, production database role separation, metrics, independent recovery, load tests and external security review remain gates.

## Notification and growth gates
The catalogue now includes customer-parity events and backend producers. Self-hosted Novu HTTP bridge, exact version compatibility, email-client rendering, real delivery, complaints/bounces and optional unsubscribe acceptance remain unfinished. External bank/bill autopay, money pockets, rewards, cards, lending, business products, international payments and other specialised financial programmes are not enabled.

## Publication evidence
Local tests, exact remote source, CI results, manual in-person acceptance and production readiness must each be recorded separately. Review-code publication does not authorise real transactions.
