# Testing, delivery roadmap and release gates
## What this round implements
164 catalogue rows and copy bodies; six built-in theme presets/components; HTML/plain-text rendering; per-workflow payload schemas and synthetic fixtures; an importable workflow factory with explicit policy/provider binding seams; an offline build/export command; local tests. No real Novu server, Framework dependency, provider send, financial action or production deployment is exercised by those tests.

Run from the repository root:
```sh
npm --prefix Novu test
npm --prefix Novu run build
npm --prefix Novu run check
make docs-check
make docs-test
```
The package has no external runtime dependencies for content generation. The live bridge will add pinned Framework/CLI/HTTP dependencies and a lockfile only after version qualification. Do not interpret the absence of those packages as a production bridge implementation.

## Test layers
| Layer | Required coverage | Current evidence |
|---|---|---|
| Content | Every row renders HTML/text, schema/fixture parity, unique IDs, valid metadata, escaped interpolation, bounded output | Local automated suite |
| Validation | Wrong state/audience, missing/unknown fields, float/negative/unsupported currency, malformed dates/codes, unsafe origins | Local automated suite |
| Factory contract | Explicit allowlist, no default activation, scope gating, subscriber mismatch, expiry, policy failure/denial and provider-binding requirement | Test-double harness only; not SDK acceptance |
| Real Framework | Pinned dependencies compile; actual discovery, signature validation, skip semantics and provider output schema | Pending |
| End-to-end delivery | Real self-hosted instance, provider sandbox/approved canary, delivered MIME, text part, headers, reply-to and callback correlation | Pending |
| Go integration | Atomic outbox, duplicate/concurrent trigger, uncertain response, crash/restart, stale suppression, consent/contact change races | Pending |
| Security | Cross-subscriber/staff isolation, HMAC tamper/replay, auth challenge redaction, mail-bombing limits, SSRF and private access | Pending |
| Operations | Provider outage, bridge outage, restore drill, rotation/migration, independent paging, controlled rollback/replay | Pending |
| Client visuals | Actual Novu/admin/user/mobile inbox screenshots; Gmail/Outlook/Apple Mail desktop/mobile acceptance | Pending |

A browser preview is not a screenshot of an installed Novu workflow. A test double is not proof of the real Framework protocol. A queued/sent status is not mailbox delivery. Report these distinctions in every milestone.

## Build alongside each product milestone
M0: accept self-hosting, directory, catalogue and content policy; inventory licence/host/provider/version decisions. M1: qualify/pin upstream and SDK versions, implement signed bridge, Go event/recipient/outbox foundations, delivery callbacks and sender sandbox. M2: integrate identity/KYC/funding/transfers including pending/failure/return/operator email. M3: bills/fulfilment/refunds, support/disputes/statements, treasury/risk/service notices and inbound-support plan. M4: independent security and recovery assessment, client/MIME acceptance, measured operational objectives and controlled production release. M5: activate individually approved business/recurring/merchant/growth workflows. M6: activate specialised regulated products only after their separate approval.

## Mandatory release checklist
- [ ] Exact Novu edition/release, source commit and immutable image digests reviewed; Framework/CLI/server/data-store versions qualified.
- [ ] Signed private bridge deployed with real policy and provider adapters; no mock bindings or development bypass.
- [ ] Dedicated sender/DNS, text/HTML mapping, unsubscribe endpoint/headers, provider callbacks and suppression verified.
- [ ] Every enabled workflow has a producer, trusted guard, recipient policy, consent classification, owner and application acceptance test.
- [ ] Customer, staff and old/new contact isolation verified; challenge expiry/redaction approved.
- [ ] Clean sync on an isolated setup matches the exact allowlist; no workflow collisions or lost preferences.
- [ ] Duplicate, delayed and out-of-order delivery events, timeout/crash recovery and financial-state independence pass.
- [ ] Independent backup/restore, on-call alerting, rotation and rollback drills pass with recorded outcomes.
- [ ] Real synthetic client/email screenshots and delivery evidence linked to the tested release.
- [ ] Production legal/security/operations reviewers approve the measured release; no automatic real-recipient activation from a documentation push.

Use NOT- task IDs in the canonical Novu README. Track content implementation, real SDK verification, provider acceptance and production deployment separately. The 164 content scenarios are not 164 live features or a production-readiness percentage.
