# Testing and definition of done
Status: product tests are not implemented or executed. Documentation checks have their own limited report.

## Required test layers
| Layer | Required evidence |
|---|---|
| Domain/unit | Ledger balance, fees/rounding, state transitions, permissions, KYC/limit policy and quote binding |
| Database/concurrency | Real PostgreSQL migrations, simultaneous spends, unique constraints, isolation retries, callback/poll races and crash recovery |
| Contracts | Customer/admin OpenAPI, upstream schemas, callback signatures, stable errors and generated-client compatibility |
| Integration | Durable outbox/inbox, provider simulation, original-reference recovery, notification retry and reconciliation |
| Browser E2E | Customer and staff paths, tenant/resource isolation, transaction restart recovery, exports and accessibility |
| Native E2E | Android/iOS devices, SDKs, biometrics, secure storage, deep links, push, network interruption and upgrades |
| Security | Threat model, negative authorisation tests, dependency/secrets scanning, independent penetration test remediation |
| Financial acceptance | Verified funding, payout, bill fulfilment, reversals/returns, correct ledger and partner/bank reconciliation |
| Operations | Load and soak, alert tests, off-host restoration with keys, job recovery and reconciliation after restore |

## Mandatory failure scenarios
Duplicate request with identical body; same idempotency key with changed payee/amount; concurrent debits against the same funds; crash after intent commit; crash after upstream acceptance before response persistence; callback before synchronous reply; duplicate, late, out-of-order or forged callbacks; provider timeout followed by success; definite pre-send failure; failed vend versus delayed token; missing funding notification; mismatched currency/amount; expired quote; session revocation; attempted self-approval; repeated export download; app restart and older-client compatibility.

## Definition of implemented
Source and migrations/config exist; success and failure paths work; scoped security and audit are enforced; required automated tests have executed; API/docs/README statuses and real changed UI images are updated; limitations are explicit. A stub, adapter class, screenshot, mock response, empty panel or discovered test name does not satisfy this definition.

## Definition of release accepted
Implemented is necessary but insufficient: provider/financial qualification, legal/commercial approval, deployed security, performance objectives, monitoring/support, independent recovery, mobile signing/store readiness and rollout/rollback approval must also have evidence. These are separate states, not one green percentage.

Each report records exact source commit, command, environment, test counts, pass/fail, skips and reason, artifacts, reviewer and remaining gates. A skipped database suite is not a database pass. Do not run destructive tests against production or submit live money merely to produce a green test badge.
