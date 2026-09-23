# Migrations

Canonical embedded schema is internal/service/schema.sql. Run cmd/migrate explicitly. Version/checksum drift fails rather than silently replacing a schema. Use a separate privileged migration role in a qualified production setup; API startup never migrates. [Operations](../docs/OPERATIONS.md).
