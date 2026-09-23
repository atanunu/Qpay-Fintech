# Backend tests

Tests live beside their packages under internal/. Run go test -race with TEST_DATABASE_URL and QPF_REQUIRE_POSTGRES=true. See [validation](../docs/VALIDATION.md). Synthetic provider fixtures are not live financial acceptance. Local helper tests: node --test clients/client.test.mjs.
