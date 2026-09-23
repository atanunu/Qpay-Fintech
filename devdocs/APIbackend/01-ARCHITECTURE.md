# Executable backend architecture — v0.4
Implementation specification, 22 September 2026. Go + chi fronts a modular PostgreSQL-backed service. HTTP and payment-provider I/O are outside database transaction retries. API, worker and scheduler have separate entry points; no in-memory production database and no shared QPay wallet tables.

## Ownership
`internal/service` owns identity, ledger, quotes, authorisations, payments, KYC review, support and durable notifications. `internal/httpapi` projects scoped request/response contracts. `internal/upstream` talks to approved QPay APIs; its local simulator is explicit and prohibited in production. `internal/security` owns Argon2id, opaque token hashes, encrypted secrets, TOTP and authenticated callbacks. `internal/app` validates configuration and starts processes.

PostgreSQL transactions atomically commit journals/postings, account projections, holds, payment state and notification intent. Database triggers reject unbalanced journals and historical mutation. Lock customer and affected accounts in stable order. Quotes snapshot complete payment details; transaction approval is bound to quote and session; execution requires a scoped durable idempotency key.

External payment jobs mark submission intent before network I/O. After any uncertain result or restart, query the original reference; do not issue a replacement payout. Definitive success posts the reserved debit; definitive failure releases its hold. Conflicting financial evidence stays under investigation. Bill delivery status is distinct from payment status. Provider unavailability must not masquerade as successful payment.

## Security boundaries
Customer/staff audiences are distinct. Staff require MFA to operate; bootstrap creates an MFA-enrolment-only session until confirmation. Browser cookie sessions require matching CSRF and exact permitted origins; mobile uses bearer tokens. Access and refresh tokens are random, stored hashed, rotated and revocable. Customer verification and tier approval are required for spending. Secrets and sensitive destination/fulfilment fields are encrypted using deployment-owned keys. Restriction and permission checks are server-side.

## Release honesty
Feature completion follows executed evidence, not file presence. No live provider, legal acceptance, production release, mobile signing or independent security assessment follows from this implementation task. Unsupported growth/regulated functionality stays explicitly disabled. Actual provider integrations, client SDK acceptance, load/recovery and current dependency security qualification are separate release gates.
