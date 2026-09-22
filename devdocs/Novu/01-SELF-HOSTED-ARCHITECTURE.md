# Self-hosted notification architecture
Status: accepted hosting/directory decision and documented target; no installed Novu environment has been verified.

## Authority and responsibility
APIbackend owns financial outcomes, authorised recipients, communication consent, outbox records, replay control and delivery evidence. Novu owns notification orchestration; the Novu/ content package owns templates, schemas and the workflow factory. Neither may post journals, release holds, approve payments, decide KYC outcomes or change an account restriction. AdminDashboard exposes permissioned operations; web/mobile consume permitted inbox information. A delivery failure cannot undo a successful financial operation.

Target flow:
```text
Product transaction + notification intent (one PostgreSQL commit)
  -> Go dispatcher + authorised recipient snapshot
  -> private self-hosted Novu API -> Novu worker
  -> authenticated Novu/ bridge -> registered delivery provider
  -> recipient mailbox
Provider delivery callbacks -> Go delivery inbox -> authorised staff view
Independent monitor/pager -> separate incident channel (not this same email path)
```

## Deployment inventory
| Component | Required ownership/boundary |
|---|---|
| Novu API | Internal trigger/admin ingress; expose only the routes required by qualified inbox clients |
| Novu worker | Queue execution; tightly scoped outbound provider and bridge access |
| Novu WebSocket service | Authenticated inbox real-time transport when that channel is implemented |
| Novu dashboard | Private operator tooling behind identity-aware access; not the customer or product admin dashboard |
| Qpay notification bridge | Our code-owned workflows, signed requests, fixed schemas, policy lookup and content rendering |
| MongoDB | Novu-owned records; separate credentials, backups and lifecycle from the financial PostgreSQL database |
| Redis queue/cache | Novu-owned runtime infrastructure; qualified version, persistence/eviction policy and recovery tests |
| Object storage, if required | Private storage for the selected upstream release; do not assume local emulators are production storage |
| Delivery provider | Dedicated transactional sender; self-hosted Novu does not operate the receiving mailbox or eliminate provider processing |

Pin the exact component list and versions after reviewing the chosen upstream release. Do not copy moving upstream compose examples directly into production. Official self-hosting guidance and framework support are recorded in [research](08-SOURCES-AND-COMPATIBILITY.md) [S1–S4].

## Community baseline and compensating design
Community is the planning baseline; Enterprise self-hosted is a separate commercial choice, not silently purchased. The published matrix limits custom environments, staff administration/MFA/RBAC and email activity tracking. Multiple layouts and code-first workflows are listed as available. Never assume Cloud parity or bypass an edition restriction [S2, S7].

Use separate non-production and production installations and credentials rather than inventing custom environments in Community. Within each installation, explicitly select the supported environment. Do not mix real subscriber data into staging. Put the Novu dashboard behind identity-aware MFA and a restricted break-glass operator process; product staff use AdminDashboard roles and audited approvals rather than shared Novu credentials. Confirm applicable team-member and branding/licensing conditions before access is granted. Do not strip required upstream branding.

Since Community email activity tracking is not included, plan a provider-to-Go callback adapter and a product delivery register. This is our own integration, not an undocumented toggle in Novu. Prove provider message IDs can be correlated back to notification intents before acceptance. Optional Enterprise capabilities may be assessed separately [S7].

## Network and environment rules
Use dedicated TLS hostnames, private service DNS and least-privilege network policies. Public hostnames are not yet selected; example.invalid values are deliberately non-live. Do not connect clients to Cloud API defaults. Configure self-hosted API/socket URLs explicitly and authenticate subscriber access. Do not place API keys in public client bundles.

Novu API/worker must reach the bridge. For releases using outbound SSRF protection, narrowly allow only the internal bridge hostname using the documented mechanism; do not permit entire private address ranges. Preserve HMAC verification and test tampered, expired and replayed requests. No production development tunnel or development-mode signature bypass [S1, S4].

Set the supported telemetry opt-out and separately inspect egress. The telemetry documentation describes a keep-alive beacon even with telemetry disabled; do not claim an air-gapped installation based only on an environment variable [S8]. Monitor authorised egress and record any permitted licensing/health traffic.

## Failure isolation
If Novu or its bridge is unavailable, keep the financial operation and outbox intact, display authoritative status in the app, apply bounded retries, and alert through an independent path. Authentication challenge delivery has stricter expiry and must fail closed rather than accept an expired challenge. No component may invent a fresh payment to fix a missing notification.
