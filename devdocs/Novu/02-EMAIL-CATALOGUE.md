# End-to-end email catalogue and policy
The canonical editable source is [emails.psv](../../Novu/catalogue/emails.psv): **175 built-in scenarios: 145 core, 20 growth and 10 separately regulated**. The full catalogue is shipped, not an empty placeholder list. Every row has audience, rollout scope, category, theme, a precise guard name, subject and body copy. The renderer exports one HTML preview, plain-text preview, JSON schema and synthetic payload per workflow.

Workflow ID: `qpf-email-<key>-v1`. Event type: `qpf.notification.<key>.v1`. Do not rename IDs for a subject/theme-only edit. A breaking contract creates an explicit migration; do not repurpose an old ID. `guard` is a requirement for a persisted business predicate, not proof from a client-supplied state string. The Go publisher and live policy adapter must implement and test each predicate before that workflow is activated.

## Coverage
| Group | Required email outcomes included |
|---|---|
| Identity/account | Contact verification, welcome, onboarding reminder, password reset/change, phone/email change with old/new snapshots, closure requested/cancelled/completed |
| Security | New devices, suspicious activity notice, session revocation, MFA changes, recovery lifecycle, PIN-change notice, restrictions/restoration |
| KYC | Submission, additional information, approval, unsuccessful review, expiring/expired documents and tier/limit updates |
| Funding/wallet | Funding account provision/change, verified credits, unresolved incoming funds, information requests, return lifecycle, compensating correction, holds and optional balance alerts |
| Transfers | Accepted, pending, action required, confirmed completion, definitive failure, supported cancellation, return pending/posted, internal recipient credit, beneficiaries and requested receipt copies |
| Bills | Order/pending, airtime, data, prepaid/postpaid electricity, television, internet, approved other billers, delayed/recovered fulfilment, failure, refund pending/posted and opt-in reminders |
| Records/privacy | Requested statements, generation failure, optional summaries, privacy requests, secure export readiness and approved responses |
| Support/disputes | Case creation, reply, evidence/action request, escalation, resolution/reopening, dispute submission, evidence deadlines and decisions |
| Service/legal | Approved maintenance/disruption/restoration, fee/terms/privacy notices, consent changes and required app updates |
| Staff/support | Invitations, role/access/auth changes, independent approval requests/deadlines/results, KYC queue, assignment, SLA and dispute deadline |
| Finance/treasury | Float thresholds, reconciliation exceptions/reporting, aged pending payments, unmatched funding, settlement lifecycle, refund proposals, corrections and exports |
| Risk/platform | Restricted cases, privileged actions, incident notices, notification backlog/provider failures, bounce/complaint spikes, dead letters, bridge health, backups/restores, certificates, sync and drift |
| Growth (off) | Recurring mandates/payments, business/KYB, bulk/payroll, merchant collection/settlement/invoices/refunds, payment requests, cashback, referrals, campaigns and budgets |
| Regulated (off) | Cards/disputes, lending/repayment, savings/investment statements, FX/remittance, insurance, agents and escrow notices |

## Recipients and event ownership
`customer` resolves only the account owner's approved contact. `customer-old-contact` freezes the previously verified contact/version; it must not be silently rewritten to the new address. `customer-new-contact` is restricted to active verification challenges or an already committed/verified replacement. Store mapping and verification reason server-side, never use arbitrary email addresses in the event payload.

`staff-*` audiences resolve active staff with the required finance, security, support, platform or administration permission. Invitations are bound to the specifically authorised invitee. Do not broadcast staff messages to all staff or customers. `business` requires current organisation membership, scoped permissions and approved business-product activation. One intent per authorised recipient; do not reveal other recipients in To/CC lists.

Identity owns identity/security events; KYC owns verification decisions; ledger/funding/transfer/bills own their confirmed financial/fulfilment events; support owns case publication; compliance owns approved legal/customer incident copy; finance owns reconciliation/treasury; platform engineering owns infrastructure notices. Each module must add an event producer and acceptance test with its feature implementation.

## Common delivery defaults (proposed policy, not an SLA)
| Category | Release/delivery rule |
|---|---|
| Essential | Purpose-limited account/security/service notice; product/legal classification must be reviewed. Not blanket permission to ignore a provider suppression. |
| Transactional | Send only for the owner's actual operation; apply the agreed receipt preferences without mixing promotions. |
| Optional | Current opt-in/explicit request and category unsubscribe required; recheck before dispatch. |
| Marketing | Off by default; independently approved campaign, eligibility, opt-in, frequency cap, one-click unsubscribe and sender stream. |
| Operational | Restricted staff/on-call routing, fingerprinted deduplication, escalation and independent incident fallback. |

Content contract TTL ceilings: verification codes 10 minutes; operational events 1 hour; other events 24 hours. Go may choose shorter expiry. Expired messages must be suppressed, not renewed by a retry. These TTLs are maximum dispatch windows, not delivery guarantees or legal-notice response deadlines. A still-relevant legal/statement notice that expires requires a new authorised notification intent with the original document/effective dates preserved. Underlying financial replay records have a separately approved, longer retention period.

Emit at most once per recipient/event/version by default. Pending alerts are emitted after the module's approved age threshold, not on every poll. Default reminder policy is one notice per scheduled occurrence; repeat intervals and operational escalations require separately identified occurrences. Final outcomes suppress unsent pending notices. Do not digest security challenges, final financial receipts, urgent evidence deadlines or critical incidents. Optional monthly summaries may be digested. Quiet hours are user-configurable for optional content; they must not postpone an essential deadline without an approved alternate route.

## Financial wording and fulfilment
Timeout is never mapped to definitive failure. A bank-transfer completion email requires confirmed outcome and ledger effects; accepted/submitted means only that the instruction was accepted. A returned transfer or refund email requires its own verified posted credit. A bill payment and delivery of value are separate predicates. Tokens and full meter/account details remain behind authenticated access; default email says the token is ready, not the token itself. A recovered token belongs to the original order and must not trigger a new vend.

Never include raw AML/sanctions reasoning, suspicious-activity reporting, internal watchlist data or confidential investigations in customer copy. Customer-facing restrictions and incident notices use approved neutral wording. Staff emails contain case references, not investigation dossiers. Legal notices must link to the approved, versioned notice containing exact dates; no invented deadlines are placed in templates.

## Completeness rule
A catalogue row is not a live feature. Before activating it, add its domain producer, exact persisted guard implementation, recipient/consent policy, provider mapping, tests, operational owner and release evidence. New functionality cannot pass acceptance without its success, pending, error/action, correction/refund and operator notifications, or an explicitly reviewed explanation for why a state needs no email.
