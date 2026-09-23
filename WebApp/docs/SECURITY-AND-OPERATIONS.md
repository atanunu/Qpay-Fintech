# Browser security and operating boundaries

No provider master keys, Novu secret keys, browser bearer credentials or transaction approval tokens are persisted. Web cookies, CSRF, approved origins and server ownership rules remain the primary API boundary. All rendered API text uses React escaping; no untrusted HTML renderer is installed. Quotes and balances are checked for supported currency, exact integer representation and coherent totals before use.

Payment forms require current API-verified details, a review step, a short-lived quote, PIN and optional authenticator code, followed by an explicit final action. Editing returns to the quote step. PIN/MFA values are cleared after the submission attempt. There is no automatic money-changing retry, offline queue or silent provider fallback. A pending/unknown operation is neither failed nor financially completed. Success receipts cannot be opened for failed or pending records. Token retrieval never re-vends a bill.

The network transport times out, rejects redirects to other origins and preserves a non-sensitive request reference. Credential operations that revoke sessions return the customer to sign-in. Customer and staff APIs remain separate. Errors do not send their original payloads to analytics/logging. Customer data is not embedded in third-party telemetry. Recovery records store references only; privacy-draft identity files are memory-only and never uploaded.

The UI review mode is compiled explicitly with an acknowledgement and a persistent synthetic banner. The API build has no runtime switch to a simulator. Production-capability reports block the payment controls in this test release, but this is not a substitute for backend security or a production kill switch. The application is not approved for actual money movement.

## Deployment
See [deployment template](../deploy/README.md). Use SPA fallback for direct routes, a narrowly configured CSP/connect-src, TLS on same-site web/API hosts, no-store for private pages, noindex for test hosts, and independent session/CSRF testing. Static assets do not need server-side credentials. Image digest pinning, dependency review, load/cross-browser testing, TLS and ingress acceptance remain release work.

## Incident handling
If the API is unavailable, show the connection failure rather than another user's stale data or synthetic balances. If submission response is lost, preserve the reference and query the original; do not offer blind resubmission. If a session is expired, recover reads only and require the user to reattempt writes deliberately. If a quote has expired or a policy changes, obtain and approve a new quote. If a bill value is delayed, query the original fulfilment and open support if needed.

For in-person tests, use synthetic accounts, fixtures and receipts. Store trace artefacts briefly and review them for test credentials before sharing. Do not capture MFA setup secrets/recovery codes or real documents. The screenshot inventory excludes those sensitive modal states. Rotating keys, custody, reconciliation and provider activation are backend/operator responsibilities, not browser settings.
