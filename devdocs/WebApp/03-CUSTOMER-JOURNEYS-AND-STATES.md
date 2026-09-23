# Customer journeys and state contracts

## Authentication and identity
Password and passkey login share the browser HttpOnly-cookie and CSRF session flow. Passkeys require user verification, the configured relying-party ID and allowed origins; existing TOTP remains required when enabled. Registration still needs email verification. Customer legal identity is submitted separately from an editable display name.

Identity draft states are resumable draft → submitted → independent staff outcome. Documents must belong to the customer and have an accepted scan state. Local fixtures explicitly say unscanned. A submission stores an immutable legal-name snapshot; later draft edits do not rewrite reviewed identity.

## Money movement
New intent: recipient selection → fresh name/customer verification → immutable quote and full debit → PIN/optional MFA → idempotent submission → original operation status. A repeated purchase creates a new quote, not another use of an old approval. Missing responses enter recovery; absence of a lookup match is not proof of failed execution.

External payment states remain accepted, submitted, pending, succeeded, failed or pending_review. Bill fulfilment is independent: a financially successful bill may still await purchased value. A success receipt only follows confirmed success. The timeline exposes relevant events and current hold status without provider secrets.

## Requests and schedules
A request participant can see only their share; the requester can see their own request’s shares. Quoting does not settle it. The same database transaction rechecks cancellation, expiry, remaining amount and intended recipient before updating request fulfilment and ledger postings.

Reminders do not debit. Internal mandates require explicit amount, per-occurrence/whole-plan caps, end date, count, cadence, fresh quote, PIN and optional MFA. Every occurrence rechecks current policy, account eligibility and funds; duplicates produce one effect. Insufficient funds, restrictions, stale execution or changed fees outside approval pause rather than silently retrying a new debit.

## Account maintenance
Email change requires password/optional MFA plus separate old/new-mailbox codes, with expiry and bounded attempts. Completion changes the address transactionally and revokes all sessions. Personal unfreeze never removes compliance restrictions. Closure requires a zero wallet balance, no held funds, no unresolved payments and no open cases; it retains records, cancels future schedules and revokes access.
## Verification and release boundary
Implementation and automated verification are distinct from in-person acceptance, actual partner acceptance and production enablement. Use synthetic data only. Supported backend operations never fall back to fabricated success. The customer API remains the authority; browser visibility is not an authorisation check. Full v0.6 browser/CI evidence belongs in [WebApp validation](../../WebApp/docs/VALIDATION.md).
