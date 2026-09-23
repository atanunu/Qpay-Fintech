# Security, privacy and accessible UX

Anonymous and signed-in layouts remain separate. Browser authentication uses HttpOnly cookies, approved origins and CSRF. WebAuthn checks RP/origin, user verification, expiry and single use; login does not approve payments. Preserve MFA and revocation.

Private JPEG/PNG/PDF uploads are owner/purpose-bound, size limited, encrypted and authenticated on download. Non-local operation requires scanning; local unscanned fixtures prove no production scanning. Submitted identity is immutable. Email changes require both mailboxes; closure checks balances, holds, payments and cases. SMS and complete asynchronous privacy export remain gated.

No service worker/private offline cache or error-triggered fixture fallback. Support labels, keyboard focus, reduced motion, readable themes and narrow layouts. Cosmetic masking is not access control. Automated accessibility checks do not replace manual acceptance.

[Scope](02-COMPETITOR-PARITY.md) · [Web register](../../WebApp/README.md) · [API contracts](../../APIbackend/docs/PARITY-API.md) · [Verification](../../WebApp/docs/VALIDATION.md)
