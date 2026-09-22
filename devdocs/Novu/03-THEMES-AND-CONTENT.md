# Built-in email themes and content system
Six version-controlled themes are implemented in [content.mjs](../../Novu/src/content.mjs), with the full workflow copy in [emails.psv](../../Novu/catalogue/emails.psv). They are email layouts/presets, not alternative themes for the Novu administration dashboard or an upstream fork.

| Theme | Intended content | Built-in presentation |
|---|---|---|
| Core | Account, service and everyday updates | Calm branded header, readable content and secure-app action |
| Security | Verification, recovery and credential/device changes | High-contrast security label and anti-phishing callout |
| Receipt | Verified credits, payments and refunds | Exact NGN amount and reference panel; no floating-point arithmetic |
| Support | Cases, evidence, restrictions and unresolved operations | Clear action-oriented copy and no confidential attachments |
| Operations | Staff/finance/risk/platform notices | Restricted-operations label, case/reference context and secure dashboard action |
| Campaign | Approved optional growth offers | Distinct promotional styling and explicit unsubscribe material; disabled for live use until approved |

These styles are initial implementation defaults, not a locked public brand identity. Theme selection comes from the reviewed catalogue, not a client payload. Shared brand settings include name, legal entity, approved postal address, support mailbox and HTTPS customer/staff/business origins. Live values are not known yet. Synthetic example.invalid branding fails production validation. Preserve applicable Novu branding/licensing obligations; custom Qpay templates are not permission to bypass upstream restrictions [S2 in research].

## Rendering and safe content
The renderer produces HTML and plain text for every workflow, using table layouts, inline CSS, system fonts and a maximum 600px content area. It has no JavaScript, remote images, external fonts or tracking pixels. Text interpolation is escaped. Payloads reject unknown fields, arbitrary URLs, unsupported currencies, malformed IDs, invalid dates and control characters. Amounts use integer strings and BigInt; NGN is the only receipt currency supported in this version.

Shared components are header, message label, heading, content, optional amount/reference panel, security warning, secure-app action and legal/support footer. Both HTML and text carry a usable action destination. Action URLs are built from approved origins and a restricted reference; that reference grants no access. The application must authenticate and authorise the referenced screen. Verification uses a Go-issued short-lived code and the application's challenge screen, not an email click that approves a payment. Email scanners must not cause GET requests to consume a challenge, authorise a payment or change credentials.

Optional/marketing content produces visible unsubscribe text and RFC-style one-click header material. A qualified provider adapter must actually transmit those headers and the text MIME part. They are not valid arbitrary properties on Novu's email-step output; the factory returns only subject/body and uses an explicit provider-binding seam [S6]. The unsubscribe handler is a separate pending Go feature; GET displays choices, authenticated/tamper-resistant one-click POST handles the permitted subscription change without requiring account login. It must never change essential-account policy or other customer settings.

## Accessibility and visual acceptance
Test 320px mobile layouts, 200% zoom, long names/references, dark-mode transformations, images disabled, text-only reading, keyboard link clarity and screen readers. Review contrast, heading hierarchy, touch targets, wrapping and footer readability. Capture Gmail, Outlook and Apple Mail desktop/mobile rendering using synthetic data before release. Browser previews are useful content evidence but do not prove email-client compatibility, delivered MIME structure or a live Novu deployment.

Generate all previews with `npm --prefix Novu run build`, then open `Novu/dist/index.html`. The build also exports per-workflow fixtures and schemas. dist/ is intentionally ignored to avoid treating regenerated preview output as canonical source. Published preview artefacts must identify source hashes, capture environment and synthetic fixture. The service runtime SCREENSHOTS.json remains uncaptured until actual Novu/application screens exist.

## Authoring and change controls
Create or edit the row, update schema/fixture rules when needed, render all affected templates, run tests, update service/root READMEs and submit a reviewed change. Security, financial, legal and marketing copy require the appropriate domain reviewer. No dashboard-only production edits: emergency changes are audited, reconciled back into Git, reviewed and included in the next release. Free-form HTML from customers, staff forms or an AI response cannot become executable workflow content. Localisation beyond en-NG requires a reviewed catalogue/schema version, not an unreviewed runtime translation.
