# Shared API and event contracts
Status: contract workspace reserved; executable OpenAPI/event schemas are not yet implemented. The [API plan](../devdocs/project/06-API-CONTRACT-PLAN.md) is the design source until reviewed machine-readable contracts exist.

Use openapi/ for versioned customer and independently scoped staff APIs; events/ for versioned inbox/outbox and partner events; fixtures/ for synthetic compatibility cases. APIbackend owns execution semantics. Generate typed clients after contract validation, never hand-maintain a second money model in a frontend. No provider secrets or real payloads here.

Contract acceptance must cover authentication/audience, object scope, decimal-string minor units and currency, transaction-bound authorisation, request fingerprint/idempotency, asynchronous outcomes, original-reference query, structured errors, pagination, callback authenticity/replay, supported field evolution and deprecation. Tests must check schema compatibility as well as financial behaviour.
