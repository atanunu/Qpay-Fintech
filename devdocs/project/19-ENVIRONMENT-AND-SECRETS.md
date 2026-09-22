# Configuration, secrets and local-environment contract
Status: configuration plan, not deployable environment files. Actual names and validators must be implemented with each application and documented in its README. No credentials are supplied.

| Scope | Planned configuration | Rule |
|---|---|---|
| APIbackend | APP_ENV, HTTP_LISTEN_ADDR, PUBLIC_ORIGIN, STAFF_ORIGIN, DATABASE_URL | Validate at boot; distinguish browser origins; database reachable only by approved backend processes |
| Backend workers | QPAY_BASE_URL, QPAY_CREDENTIAL_REF, QPAY_CALLBACK_KEY_REF, JOB_LEASE_DURATION | Resolve scoped credentials from a secret manager; credentials are references here, never committed values |
| Crypto/storage | KEY_PROVIDER, KEY_ID, OBJECT_STORE_ENDPOINT, PRIVATE_BUCKET | Separate encryption/signing purpose; keys recoverable under controlled off-host procedures; no public KYC bucket |
| Observability | OTEL_ENDPOINT, LOG_LEVEL, ALERT_TARGET_REF | Redact tokens, OTPs, raw PII and bank-account detail; low-cardinality metrics |
| Browser apps | VITE_API_BASE_URL, VITE_APP_ENV | Public build-time configuration only; no secret belongs in a VITE-prefixed variable |
| Mobile app | EXPO_PUBLIC_API_BASE_URL, EXPO_PUBLIC_APP_ENV | Public bundle values only; signing credentials and provider secrets remain outside the bundle |
| CI | Build/test-only fixture configuration | No production provider/database credentials in PR workflows |

Use synthetic .env.example files only when runtime validators exist. Commit lockfiles and supported toolchain versions with actual manifests, not speculative package declarations. Require TLS and approved origin/cookie rules outside isolated development. Never treat a tunnel or private hostname as a substitute for application authentication.

Keep sandbox and production partner accounts, keys, callbacks, databases, object storage and notification destinations separate. Production activation requires an explicit environment gate and independent approval. No default credential, debug bypass or simulator fallback in production. Fail closed for required financial configuration; permit presentation-only degraded modes only when clearly labelled.

Browser authentication must use a deliberately reviewed cookie/session or token design with CSRF protection where applicable. Mobile secrets require platform-backed storage and revocation; no cleartext refresh tokens or financial authorization material in general app state. Staff/customer sessions cannot be reused across audiences.
