# Web deployment and test-only safety boundary

Use the ordinary API-connected build for a served web app. Use the separately named offline HTML only for synthetic design review. Never publish the review build as a real financial service.

The supplied Dockerfile builds static assets and serves them with a non-root Nginx image. Its current tags are review defaults, not immutable production pins. Qualify image digests, dependency advisories, TLS, health checks, resource limits, trusted proxy settings and backup/release procedures before production acceptance.

Set `VITE_API_BASE_URL` at build time to the reviewed HTTPS API **origin**, without a path, query or credentials. Update the exact `connect-src` origin in nginx.conf to match; do not use `*`. Put the web and API hosts on a reviewed same-site HTTPS domain so the backend's Strict cookies work. Public VITE values are visible to the browser: they must never contain QPay/Novu master keys, passwords or provider secrets.

The Nginx template includes SPA deep-link fallback, no-store, noindex, CSP, no-sniff, referrer and framing controls. TLS termination and HSTS belong to the actual approved ingress configuration, not an unverified local example. No service worker or decrypted offline account cache is installed. Files in a local build are not a deployment.

In this release the application refuses financial submission when the API reports a production environment. Backend deployment must independently keep production execution disabled. Client-side controls alone cannot secure a payment API.
