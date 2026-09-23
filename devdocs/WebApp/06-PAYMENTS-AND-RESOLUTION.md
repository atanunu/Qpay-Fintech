# Payments and resolution

Resolve an exact opt-in handle or perform an owned bank enquiry. Saving a beneficiary is optional. Show amount, destination, fees, total debit and quote expiry before quote/session-bound PIN/MFA approval. Money is an integer minor-unit string.

GET /v1/payments/lookup recovers the original key/quote. Persist only scoped references, never PINs or approval tokens. Absence or timeout is not failure and must not create a second payment. Timelines distinguish held funds, financial outcome, fulfilment and notifications. Repeat prepares a fresh quote. Financial refunds and complete reconciliation remain gated.

[Scope](02-COMPETITOR-PARITY.md) · [Web register](../../WebApp/README.md) · [API contracts](../../APIbackend/docs/PARITY-API.md) · [Verification](../../WebApp/docs/VALIDATION.md)
