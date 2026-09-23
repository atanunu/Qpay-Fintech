# GitHub delivery — 23 September 2026

Recovered 100 complete file records with their SHA-256 checks, including all application records before the truncated documentation tail. Seven missing referenced documents were rewritten and progress regenerated. This is a recovered/corrected delivery, not a claimed byte-identical archive of the vanished workspace.

Fresh local evidence: full Go 1.27.1 race/database tests, vet and build passed against isolated PostgreSQL 16.15; 71 frontend unit/contract tests and TypeScript/builds passed; 218 Novu tests/check/build passed; 29 documentation regression tests passed. Duplicate OpenAPI parameters were corrected without weakening the existing assertion.

Local browser navigation was blocked by managed Chromium (ERR_BLOCKED_BY_ADMINISTRATOR). GitHub normal-browser and actual Go/PostgreSQL integration results must be recorded separately. Manual UAT remains not-tested. No provider activation, deployment or real-money operation is included. The service remaining-work registers still apply.

[Recovery manifest](DELIVERY-SOURCE-MANIFEST.json) · [Web register](../../WebApp/README.md) · [API register](../../APIbackend/README.md)

Browser delivery correction: native fetch receiver fixed in both clients; customer capability display now uses the API payments field and fails closed on unknown values; bank quote submission waits for its selected beneficiary; recovery tests wait for route changes. Added regression tests. Local frontend tests: 79 passed; shared client tests: 7 passed. GitHub rerun and manual UAT remain distinct acceptance gates.
