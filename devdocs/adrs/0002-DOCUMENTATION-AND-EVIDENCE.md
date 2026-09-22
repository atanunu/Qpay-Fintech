# ADR-0002: Documentation and truthful progress are mandatory
Status: Accepted. Decision date: 2026-09-22. Owner: repository owner.

## Decision
Root README and each of the four service READMEs are maintained in every coherent delivery. Service READMEs are canonical for stable API/MOB/ADM/WEB task IDs; JSON and root summaries are generated. Preserve the 114 existing task rows. A document, adapter, mock screen or passing docs job cannot promote a product task to Implemented.

Track source implementation, executed verification, provider acceptance and deployment separately. Before marking an item Implemented, link source and test evidence and satisfy its stated acceptance. Record skipped/unexecuted checks. Break broad expansion epics into children without double counting.

## Visual evidence
Use actual synthetic-data app captures for runtime screenshots, with commit, route/screen, environment, device/viewport, time and SHA-256. Design diagrams are labelled target architecture, not running systems. APIbackend has no invented dashboard. An absent app has no runtime screenshot and must say so.

## Enforcement
Run documentation validation on pushes and PRs with read-only permissions and immutable action references. Repository rules must separately require checks/review; do not claim administrative settings have been changed without verification. No automatic main-branch publication, provider secrets or deployment credentials belong in untrusted PR jobs.

## Safety
No force push, overwriting newer work, secret/customer-data publication or financial action follows from a documentation task. Preserve the upstream QPay repository unchanged unless separately requested.
