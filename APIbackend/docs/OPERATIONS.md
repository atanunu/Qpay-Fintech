# Runtime and operations

## Process model
api handles HTTP; worker consumes financial and notification work; scheduler expires credentials/intents and cleans rate records; migrate applies explicit checksum-tracked schema changes; qpfctl performs limited offline administration. No startup migration shortcut or customer-accessible wallet credit endpoint exists.

## Local setup
From the repository root run the README commands. The generated .env.local is fresh and owner-only. Compose's PostgreSQL profile and synthetic balances are local development tools. Stop without losing data using `docker compose --env-file APIbackend/.env.local -f APIbackend/compose.local.yml down`. Adding `-v` deliberately destroys local database volumes; never apply that command to a production environment.

The optional smoke test `python3 APIbackend/scripts/smoke_local.py` expects the local stack already running. It creates synthetic users through the controlled CLI and checks internal transfers, simulated bank/bill execution, idempotency, receipts, fulfilment and exact balances. It does not call a real payment provider.

## Configuration
QPF_ENV is explicit local/staging/production. DATABASE_URL requires verify-full TLS outside local. AUTH_PEPPER and DATA_KEYS are independent base64 32-byte keys; DATA_KEY_ID selects the active encryption key while historical keys allow controlled recovery. WEB_ORIGINS is an exact origin allowlist. EXECUTION_MODE=off/local/qpay is explicit; local is rejected outside local. QPay mode needs its own scoped credentials, approved sender, environment and explicit contract acceptance. ACCOUNT_POLICY_ACCEPTED does not grant a licence; it is a deployment gate after policy review.

NOTIFICATION_MODE=off/local/novu is explicit. Novu mode requires self-hosted HTTPS API, secret, signed policy key and release allowlist. A submitted notification is not delivered; unknown sends are not blindly retried. Real delivery callback integration remains pending.

Never commit secrets, real customer/KYC data or production screenshots. Terminate HTTPS at a trusted proxy, restrict service ingress and validate forwarded-IP/rate policies. Current local database ownership and image tags need least-privilege role/digest/CVE qualification before production.

## Investigation
Readiness failure: inspect database TLS/configuration and apply reviewed migrations. Never erase migration checksums to conceal drift. Pending payment: inspect original payment/reference, immutable observations and job state; do not create replacement funds movement. Provider conflict: retain hold and escalate. Expired notification: suppress; issue a new authorised verification challenge instead of extending an old one. Refresh reuse: reauthenticate and investigate the revoked session family.

## Required release operations
Off-host backups must include PostgreSQL, encryption keys, configuration and unresolved operation evidence. Restore into an isolated environment with outbound payments/email disabled. Reconcile ambiguous external effects before restarting dispatch. Record measured RPO/RTO and prove rollback/migrations on a restored copy. Full restore, load, monitoring and provider incident exercises have not been performed in this delivery.
