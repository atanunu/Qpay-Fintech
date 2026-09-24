# Admin v0.7 verification record

## Current verified development evidence

Application head: `f7e91005d83fcedc56ecec1da4fa6c47c251a52f`. Tested PR merge revision: `79ceb97aa548f06f9eab5f292f07491efae45dac`, based on `6977d16a12fa0b8034c407eb172e2a47a7a9aea7`. GitHub's temporary test-merge SHA is not a completed merge. Final merge/check receipts are maintained in PR #3.

| Verification | Recorded result |
|---|---|
| Strict TypeScript | Passed |
| Admin unit and contract tests | 63 passed |
| Synthetic Chromium review | 19 passed, 0 failed, skipped or flaky; retries disabled |
| Chromium to actual Go/PostgreSQL | 11 passed, 0 failed, skipped or flaky; retries disabled |
| Versioned notification metadata | 175 entries validated |
| API-connected and isolated review builds | Passed; review credential marker excluded from API release |
| Non-root static container, routing and security headers | Passed |
| Backend modules, vet, race/PostgreSQL, builds, contracts and Compose smoke | Passed |
| Existing WebApp review and real-backend regressions | Passed |
| Documentation and Novu content verification | Passed |
| Runtime screenshots | 41 PNGs committed, each with original source, time, viewport and SHA-256 |

Runs: admin `35961917855`, backend `35961917926`, WebApp `35961917828`, documentation/notifications `35961917885`. [Delivery record](../../devdocs/project/ADMIN-DELIVERY.md), [machine-readable evidence](DELIVERY-VERIFICATION.json), [gallery](SCREENSHOTS.md).

These results establish the tested development scope, not universal browser coverage, an independent security review, production performance, or complete feature-group acceptance. The separate proposed synthetic identity reveal/hide test file is not present in this revision and is not counted. Identity projection has six unit tests and private identity access is exercised by the passing real API suite.

## Release boundary

No live QPay or Novu activation, production deployment, real-money payment, real customer evidence, regulatory acceptance or independent approval is represented by this record. In-person review, full refunds/returns and three-way settlement, future-effective pricing, live notification feedback, production storage/scanning and security/load/restore qualification remain in [pending work](../../devdocs/AdminDashboard/07-PENDING-WORK.md).

## Historical attempts (superseded by the passing runs above)

The following records describe earlier intermediate revisions; their pending/failure statements are historical, not the current CI result.

Status: implementation candidate; normal-browser CI and final delivery revision still pending.

Locally executed: TypeScript passed; 45 frontend tests passed. New admin PostgreSQL tests and the existing service race suite passed. The complete HTTP race suite passed after correcting its new fixture to use the actual command contract. API, worker, CLI and migrator compiled. The CLI + HTTP fixture successfully created first/second administrators, independently approved staff invitations, enrolled MFA and submitted synthetic private identity evidence and a real local-ledger transfer.

The first aggregate local race run failed only on an incorrectly shaped new HTTP fixture (kind/value instead of resource); that fixture was corrected. The final all-package local race rerun passed: 95 top-level tests, 367 including subtests, zero failures or skips. The published backend verification also passed on abc1e494, including PostgreSQL 18.6 and the Compose smoke test. Local PostgreSQL is 16; CI uses the existing repository PostgreSQL 18.6 baseline. No result from an interrupted command is counted as passed.

Normal HTTP browser navigation is blocked by the local Chromium environment policy. Browser suites must pass in ordinary GitHub Actions Chromium before acceptance; no policy workaround is used. Local build and API fixture success are not browser evidence. CI run IDs, exact source revisions, zero-failure/skip/retry counts and screenshot provenance will be added after execution.

Still separate: in-person review, live QPay/Novu qualification, production storage/scanner, deployment/security/load and restore acceptance. No production transactions or real customer data were used.

## Continuation qualification, 24 September 2026

The complete local all-package Go race run passed again on recovered source ab6f549a5128da3f3bdd785bb6e13badb396d8bf. The subsequent frontend corrections pass TypeScript, 57 unit/contract tests, API-mode production compilation and the 175-entry canonical notification-content check.

Normal-browser run 35959797293 identified seven synthetic-review failures and five API-suite failures. Fixes address explicit layout labels, a mutable review-user snapshot that could dismiss MFA recovery codes, root text contrast in dark mode, neutral MIME-specific download names, and target-specific state cleanup. Tests now use the existing explicit Open-record links and the documented HTTP 201 reconciliation-import response. A non-secret fixture counter survives Playwright worker replacement, without permitting one-time MFA proof replay. The suite still uses zero automatic retries and retains all security/accessibility assertions.

These corrections require a new normal-browser run; prior failures are not counted as acceptance.
