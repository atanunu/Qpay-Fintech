# MobileApp specification index
Status: planning foundation. Detailed service specifications below are planned, not completed.

[Service README](../../MobileApp/README.md) · [Shared index](../00-INDEX.md)

## Scope
Native customer experience for Android and iOS. Approved stack: M1: React Native + TypeScript + Expo development builds; approved 2026-09-22. Use the canonical feature register for all task states.

## Required detailed specifications
| File to write | Contents and acceptance |
|---|---|
| 01-ARCHITECTURE.md | Actual modules, source directories, trust boundaries, selected dependencies and deployment |
| 02-INTERFACES.md | API contracts, typed clients, events, validation, errors and compatibility |
| 03-WORKFLOWS.md | Success, error, pending and recovery journeys with authorisation and evidence |
| 04-CONFIGURATION.md | Safe environment names, secrets handling, defaults and clean setup |
| 05-TESTING.md | Executed test commands, coverage scope, fixtures and verification gaps |
| 06-OPERATIONS.md | Deployment, migrations where owned, rollback, monitoring and troubleshooting |
| 07-PENDING-WORK.md | Dependencies, assigned issues, blockers and precise next acceptance requirements |

Read shared [architecture](../project/03-ARCHITECTURE.md), [ledger requirements](../project/05-DATA-AND-LEDGER.md), [API plan](../project/06-API-CONTRACT-PLAN.md), [UX](../project/09-UX-AND-SCREEN-PLAN.md) and [testing](../project/10-TESTING-AND-ACCEPTANCE.md) before implementing. Do not derive live behaviour from one isolated document.
