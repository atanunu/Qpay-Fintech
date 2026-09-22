# Mandatory documentation and traceability policy
Applies at all times to root README and all four service READMEs. Adopted by user approval on 22 September 2026. Documentation CI is supplied in this repository. Required-check and review enforcement in GitHub repository settings is a separately tracked administrative task, not inferred from workflow presence.

## Canonical registers
Each service README is the canonical human-reviewed detailed feature/task register. Keep stable API/MOB/ADM/WEB IDs. Generate each service's docs/FEATURES.json and aggregate summaries from these registers; do not independently edit duplicated status data. Root categories and service task counts overlap and must never be added as distinct features. A percentage of rows is not production readiness.

Every task needs ID, capability, service ownership, priority, milestone, status, specification, measurable next acceptance, dependencies, named assignee or explicit unassigned state, issue/PR when created, implementation path, test path and executed evidence, images when applicable, and blocker/next action. At the documentation baseline, role ownership is proposed, issues and assignees are not yet created, and all application implementation/evidence states are explicitly absent. Detailed issue decomposition must populate dependencies and accountable assignees before sprint commitment.

## Status vocabulary
Planned; In progress; Partial; Implemented; Blocked; Unverified; Deferred. Implemented means source-backed and tested against stated acceptance, not released. Track verification separately as Not run/Passed/Failed, partner acceptance separately as Not assessed/Pending/Accepted, and release as Not deployed/Deployed/Accepted. An unverified historic implementation must not be silently marked done. Requirements and changed scope require approval, not merely a status edit.

## Mandatory service README content
Purpose and ownership; selected/proposed stack; architecture/workflow diagrams; real screenshot gallery with provenance; setup prerequisites and supported commands; safe environment-variable reference; source/API map; auth/roles/security; stable full feature register; done/pending/blocked work; acceptance and test reports including skips; operations/troubleshooting; migrations/deployment/rollback/backup; dependencies and limitations; roadmap; changelog. Planning-only readmes must say when setup/commands, product images or runtime evidence do not exist instead of inventing them.

## Mandatory root README content
Product and operating scope; all applications and their responsibilities/stacks; architecture and integration boundaries; links to each service README and documentation index; representative verified images; cross-service progress; open decisions and milestones; clean setup entry point once runnable; tests/CI/release status; security and contributor rules; changelog and honest launch blockers. Root and service changes belong in the same reviewed delivery whenever status changes.

## QPay-style module specifications
For each module create 00-INDEX.md, 01-DATA-MODEL.md, 02-API.md, 03-WORKFLOWS-AND-CONTROL.md, 04-PROVIDER-INTEGRATION.md, 05-REPORTING-RECONCILIATION.md, 06-NON-FUNCTIONAL.md, 07-PENDING-WORK.md and 08-SECURITY-COMPLIANCE-READINESS.md. Non-applicable sections need a reason, not deletion of an accounting/security consideration. See [document inventory](17-DOCUMENT-INVENTORY.md) for planned directories.

## Required CI and review gates to implement
Validate required READMEs/headings; stable IDs and allowed statuses; evidence paths and traceability; generated register freshness; source inventory; local links and anchors; real Mermaid parse/render; screenshot manifests/hash/provenance; OpenAPI schema and client generation; code/test/docs change coupling or explicit reviewed no-impact declaration; required critical tests; secrets/dependency scans. A negative regression suite must prove each checker fails for representative invalid inputs.

Use branch rules to make relevant checks required, plus review of status/evidence changes. A generator cannot decide actual financial readiness. PR checks need read-only minimum permissions; publication from trusted main can write only allowlisted outputs. Do not give untrusted PRs production credentials or an automatic broad write token.

## Delivery rule
Code, tests, migrations/config, API/UX documentation, task state, root summaries and changed real screenshots are a single coherent delivery. Verify the remote commit and CI before saying pushed/passed. Never force-push, discard concurrent work, restore old ZIPs over newer main, or assume a local file exists on GitHub. A proposal in this pack is not authorisation to change production.

## Baseline renderer
The supplied diagrams are deterministic SVGs generated from `docs/diagrams/diagrams.json`; CI verifies source/output equality and structural validity. Mermaid blocks are rejected until an actual parsing/rendering toolchain is added with tests. This avoids claiming that detecting a code fence validates a diagram.

## Fifth-service enforcement
Apply this entire README, evidence and task-ID standard to `Novu/`. Use NOT- IDs, separate content previews from live runtime captures, and report implementation, Framework sync, provider delivery and production acceptance independently.

See the [Novu specification index](../Novu/00-INDEX.md) and [service task register](../../Novu/README.md).
