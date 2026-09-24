# Admin v0.7 verification record

Status: implementation candidate; normal-browser CI and final delivery revision still pending.

Locally executed: TypeScript passed; 45 frontend tests passed. New admin PostgreSQL tests and the existing service race suite passed. The complete HTTP race suite passed after correcting its new fixture to use the actual command contract. API, worker, CLI and migrator compiled. The CLI + HTTP fixture successfully created first/second administrators, independently approved staff invitations, enrolled MFA and submitted synthetic private identity evidence and a real local-ledger transfer.

The first aggregate local race run failed only on an incorrectly shaped new HTTP fixture (kind/value instead of resource); that fixture was corrected. The final all-package local race rerun passed: 95 top-level tests, 367 including subtests, zero failures or skips. The published backend verification also passed on abc1e494, including PostgreSQL 18.6 and the Compose smoke test. Local PostgreSQL is 16; CI uses the existing repository PostgreSQL 18.6 baseline. No result from an interrupted command is counted as passed.

Normal HTTP browser navigation is blocked by the local Chromium environment policy. Browser suites must pass in ordinary GitHub Actions Chromium before acceptance; no policy workaround is used. Local build and API fixture success are not browser evidence. CI run IDs, exact source revisions, zero-failure/skip/retry counts and screenshot provenance will be added after execution.

Still separate: in-person review, live QPay/Novu qualification, production storage/scanner, deployment/security/load and restore acceptance. No production transactions or real customer data were used.

## Continuation qualification, 24 September 2026

The complete local all-package Go race run passed again on recovered source ab6f549a5128da3f3bdd785bb6e13badb396d8bf. The subsequent frontend corrections pass TypeScript, 57 unit/contract tests, API-mode production compilation and the 175-entry canonical notification-content check.

Normal-browser run 35959797293 identified seven synthetic-review failures and five API-suite failures. Fixes address explicit layout labels, a mutable review-user snapshot that could dismiss MFA recovery codes, root text contrast in dark mode, neutral MIME-specific download names, and target-specific state cleanup. Tests now use the existing explicit Open-record links and the documented HTTP 201 reconciliation-import response. A non-secret fixture counter survives Playwright worker replacement, without permitting one-time MFA proof replay. The suite still uses zero automatic retries and retains all security/accessibility assertions.

These corrections require a new normal-browser run; prior failures are not counted as acceptance.
