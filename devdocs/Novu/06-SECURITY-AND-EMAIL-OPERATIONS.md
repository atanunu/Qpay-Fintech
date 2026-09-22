# Security, sender setup and operating controls
These are implementation/acceptance requirements, not a claim of regulatory approval or installed controls. See [research](08-SOURCES-AND-COMPATIBILITY.md) for dated external references.

## Sender and deliverability
Select a dedicated Qpay-Fintech email provider account/integration; Resend remains a proposal, not a confirmed integration. Separate transactional/security and marketing streams and sender identities. Self-hosting Novu does not require self-operating an SMTP mail server. Test the selected provider's Novu compatibility, API limits, bounce/complaint events, metadata, reply routing, sandbox restrictions, idempotency and text/header override support.

Approve the actual legal sender, support mailbox, reply-to, postal address and sending domains before activation. Configure and verify SPF, DKIM and DMARC alignment, TLS and provider-managed sending infrastructure. Avoid duplicate SPF records. Start controlled sender warm-up and monitor reputation/complaints. Gmail publishes different requirements for all senders and bulk senders; the project elects to establish SPF, DKIM and DMARC from the beginning and applies one-click unsubscribe to optional/subscribed marketing content [S9]. Do not claim inbox placement is guaranteed by DNS configuration.

Provider callbacks must be authenticated and idempotent. Hard bounces and complaints suppress unsafe further email attempts; support may offer an authenticated alternate contact-verification process. Essential-event classification is not permission to bypass an invalid mailbox, blocklist or complaint. Soft bounces/delays get a bounded retry policy with expiry. Do not automatically try another provider after an ambiguous send; Novu does not provide automatic cross-provider fallback [S10].

## Data minimisation
Default email content avoids full balances, destination account numbers, BVN/NIN, KYC files, card data, meter tokens, passwords, PINs, recovery secrets, provider credentials and internal risk evidence. Use references and authenticated app views. The three verification challenge templates contain only a short-lived code under a separately approved security flow; they remain disabled until code generation/storage/expiry/attempt limits and redaction are accepted. A code must never appear in an inbox feed, logs, analytics or screenshots with real data.

Treat Novu database entries, queued payloads, templates, provider logs and outbound email as data-processing surfaces. Set purpose-specific retention and encrypted storage, audit deletion and legal holds, and document provider processing locations and agreements. Do not copy Cloud compliance certifications into a self-hosted assessment. Contact changes must invalidate unsafe queued messages while preserving legitimate old-address security notices. Account closure needs a narrowly retained permitted final-notice route.

## Access and approvals
Separate product staff and customer audiences. Go enforces RBAC and independent approvals for sensitive actions; Novu dashboard access is restricted and not a shortcut. A template preview, email reply, link click or opened event cannot approve a refund, transfer, role change or KYC decision. Put administrative access behind MFA and inspect the actual Community limitations rather than claiming built-in enterprise controls [S2].

Production content promotion needs engineering plus relevant security/finance/compliance review. Campaigns additionally need marketing consent/eligibility review. Preview templates use synthetic fixtures only. Do not accept arbitrary recipients or templates in public endpoints. Quotas/rate limits prevent password-reset mail bombing, repeated recipient probing and merchant invitation spam. Public auth responses must not disclose whether an email is registered.

## Support replies and mail loops
Transactional support notices should point to the authenticated case; any Reply-To mailbox must be monitored. If inbound email is enabled later, use a dedicated ingestion adapter with verified provider signatures, sender/case association, attachment type/size limits, malware scanning, HTML sanitisation and loop detection. Auto-responses, bounces and delivery-status messages must not create recursive cases or autoresponder loops. Treat email content as untrusted data; never execute instructions, approve payments or change case permissions from an email body. Email activity tracking is not inbound reply handling [S7].

## Incident runbooks
| Incident | Required action |
|---|---|
| Novu or bridge down | Keep outbox intents, protect expiry, use independent paging, restore service, reconcile ambiguity before controlled replay |
| Provider failure | Check provider status/configuration/quotas; pause affected sending, preserve IDs and duplicate risk; no blind failover |
| Bounce or complaint spike | Pause affected optional/campaign traffic, inspect consent/list quality and sender authentication, honour suppression |
| Wrong recipient/content | Stop affected workflow, revoke still-live links, preserve evidence, assess data exposure and approve customer notices |
| Key exposure | Restrict access, rotate with supported encryption migration, revoke compromised keys, audit usage and recover safely |
| Expired codes queued | Suppress and record; require a new user-authorised challenge, never extend the old code's expiry |
| Drift or failed sync | Keep prior qualified release, compare workflow/bridge versions, resolve conflicts without deleting active workflow history |

Security/legal teams approve any external breach or restriction notice. The platform-* email catalogue is supplementary: production on-call must have a working independent channel that does not depend on the failed email path.
