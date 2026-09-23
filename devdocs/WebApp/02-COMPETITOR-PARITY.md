# Everyday fintech parity scope

The 23 September 2026 approval covers the twelve everyday-customer improvement packages, not automatic activation of every financial product mentioned in the comparison. Outcomes drive the implementation: usable onboarding, funding visibility, practical recipients, recoverable payments, household bills, controlled scheduling, requests, security, insights, support, account maintenance and accessible desktop/mobile navigation.

## Delivery matrix
| Package | Delivered implementation | Remaining acceptance or expansion |
|---|---|---|
| Onboarding | Encrypted resumable draft, private checked documents, immutable submission and independent manual identity approval | Automatic verification provider, liveness and camera handoff |
| Funding | Persistent provisioning state, original-reference recovery, independently verified credit interface, local-only test adapter, missing-funding support | Actual QPay funding adapter/partner acceptance; hosted card/USSD collection |
| Transfers | Exact opt-in handles, one-off verified bank destinations, favourites, fresh repeat quotes, personal limits | Privacy-reviewed verified phone discovery |
| Resolution | Owner-scoped original-key/quote lookup, timeline and held funds, linked support | Provider refunds/returns and complete treasury/reconciliation |
| Bills | Encrypted named bill accounts, favourites, token archive, timestamped availability/watch | Qualified provider catalogue and real health observations |
| Scheduling | Reminder calendar plus explicitly bounded internal mandates, atomic occurrence effects, pause/cancel/history | External bank/bill autopay; new approval is required after pause |
| Requests | Equal/custom exact shares, participant privacy, partial payment, cancel/decline/nudge, authenticated QR/link | Public guest collection and scheme-interoperable QR |
| Security | WebAuthn passkeys, existing MFA, personal limits, freeze, session revocation | Actual deployment/browser recovery qualification |
| Insights | Complete monthly ledger aggregates, annotations/exclusions, category budgets | Goal-pocket ledger and linked external accounts |
| Support | Issue categories, encrypted private attachments, escalation, immutable case events | Full SLA/assignment automation and inbound email |
| Account maintenance | Dual-mailbox change, step-up profile export, zero-balance closure | Verified SMS/phone change and complete asynchronous privacy export |
| UX | Expanded responsive navigation, search, compact layout, cosmetic balance masking, keyboard labels | Manual accessibility and broader browser/device testing |

## Separately gated products
Cards, interest-bearing savings, loans, contribution groups, business banking, international payments, investments and insurance remain unimplemented and unavailable. No advertised rate, free-payment promise, licence or competitor success percentage is copied. Non-interest money goals also remain a tracked expansion, not part of the available wallet totals.

## Traceability
Preserve WEB-001–021, API-001–042 and GAP-01–14. New tasks WEB-022–034 and API-043–058 describe the added component scope. Their statuses are not a platform production-readiness percentage. Source, tests, release conditions and provider gates must accompany future changes.
## Verification and release boundary
Implementation and automated verification are distinct from in-person acceptance, actual partner acceptance and production enablement. Use synthetic data only. Supported backend operations never fall back to fabricated success. The customer API remains the authority; browser visibility is not an authorisation check. Full v0.6 browser/CI evidence belongs in [WebApp validation](../../WebApp/docs/VALIDATION.md).
