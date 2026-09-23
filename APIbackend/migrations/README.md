# Checksum migrations

Run `go run ./cmd/migrate` explicitly before API, worker or scheduler start. Version 1 remains `internal/service/schema.sql` with its original checksum. Version 2 is the additive `internal/service/parity_v2.sql`. Both apply transactionally under an advisory lock; drift fails closed. API startup checks migration two rather than auto-migrating.

Version two adds private customer-control, planning, request, identity, document, funding and passkey records. Immutable snapshots and mandate terms have database triggers. Never alter an applied migration in place; use a subsequent version. A rollback is a reviewed application/data procedure, not dropping tables with customer records. Test backup restoration, key recovery and forward-fix behaviour in isolated staging before deployment. Role-separated production privileges remain a release gate.

[Runtime operations](../docs/OPERATIONS.md) · [Parity API](../docs/PARITY-API.md).
