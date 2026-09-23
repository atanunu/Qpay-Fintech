# Capabilities, limits and account controls

GET /v1/me/capabilities reports current eligibility and effective limits. GET/PATCH /v1/me/controls manages permitted personal controls. Customers cannot raise limits above policy; the backend rechecks eligibility, session and limits at execution. Distinguish verification required, restricted, unavailable and supported states.

Sensitive changes require configured password/MFA proof. Exact handle discovery is opt-in. Personal freeze blocks new execution without discarding unresolved provider operations. Test stale quotes, lowered limits, revoked sessions and concurrent spending.

[Scope](02-COMPETITOR-PARITY.md) · [Web register](../../WebApp/README.md) · [API contracts](../../APIbackend/docs/PARITY-API.md) · [Verification](../../WebApp/docs/VALIDATION.md)
