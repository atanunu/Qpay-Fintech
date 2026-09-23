# Web and mobile integration

The core is intended for local client integration, not unrestricted production use. Read the API README and remaining-work list before assuming an endpoint exists.

## API address and transport
Use the same-site HTTPS production API origin once deployed. The supplied local Compose profile binds http://127.0.0.1:8080 for development. Exact WEB_ORIGINS are required. A physical phone needs an approved development HTTPS route; localhost on the phone is the phone, not the development computer.

Web login: POST /v1/auth/login with email,password,client=web,device, and optional mfa_code or recovery_code. Use fetch credentials=include. The response supplies CSRF material, not browser-readable access/refresh tokens. Subsequent mutating requests require X-CSRF-Token and the browser's approved Origin. After reload, GET /v1/auth/csrf with cookies and approved Origin restores the current CSRF value. Use POST /v1/auth/refresh with client=web to rotate; store the returned CSRF token. Coordinate refresh in one client operation: replaying an already consumed refresh token revokes its session family.

Mobile login uses client=mobile. Store access/refresh tokens using the platform's secure storage, not AsyncStorage/plain preferences. Inject the current access token via Authorization: Bearer. Refresh with client=mobile and refresh_token; replace both credentials atomically. No browser cookie/bearer interchange is permitted. Staff sessions use /v1/admin/auth and web transport, and must complete TOTP enrolment before operations are available.

## Money journey
1. Read /v1/wallet. Render available_minor, held_minor and balance_minor as exact strings.
2. Internal transfer uses a valid recipient account ID. Bank transfer first performs /v1/banks/enquiries, then saves /v1/beneficiaries. Bills select /v1/bills/products and call /v1/bills/validations.
3. POST /v1/quotes with kind, amount_minor string, currency=NGN, narration, and the appropriate recipient_id/beneficiary_id/validation_id. Display the returned recipient, amount, fee, total and expiry.
4. POST /v1/quotes/{id}/authorisations with PIN and MFA code when enabled. Approval is quote- and session-bound.
5. Persist a unique idempotency key before POST /v1/payments. Send quote_id and authorisation_token, with Idempotency-Key. Retrying the identical intent uses the same key, never a new payment.
6. Track /v1/payments/{id}. Accepted/pending is not success. Refresh or app restart should recover the original payment. Definitive failure and unresolved review have distinct user messages.
7. Fetch /receipt only for succeeded payments; /fulfilment returns purchased bill value only when ready. Never infer a bill token from payment success alone.

The helper clients/client.mjs deliberately does not auto-retry a money request. Its TypeScript declarations describe principal request/response models; /openapi.json is the route contract. Review generated schemas and all capability gates before enabling a screen. /v1/capabilities reports the environment; growth features remain disabled.

## Local synthetic accounts
Use qpfctl seed-local only with QPF_ENV=local. Supply QPF_CTL_EMAIL, QPF_CTL_NAME, and owner-only password/PIN files through QPF_CTL_PASSWORD_FILE and QPF_CTL_PIN_FILE. For Compose control commands, bind APIbackend/local-secrets and run the control container with the same numeric uid/gid as the file owner. Never chmod secrets world-readable. qpfctl show-local-code is a local-only CLI diagnostic, not an HTTP auth bypass.

## Other client journeys
Scoped notifications/read-state/preferences; encrypted support conversations; KYC case submission and status; account/device sessions; password/PIN changes; MFA; statement ranges (from/to RFC3339, exclusive end; format=json or csv). Payment, ledger, notification, customer and audit list routes support their documented cursors. Other bounded lists require further pagination work.

No wallet funding-account provisioning route, real refund execution, attachment upload, lending, merchant or payroll API exists yet. Do not create frontend mock success for these missing features.
