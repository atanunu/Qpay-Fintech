# Self-hosted setup, import and release procedure
The permanent directory is **Novu/**. It contains our content package, workflow factory, tests and deployment contract. Do not copy the full upstream repository into this product or modify it to bypass licensed features.

## Readiness status
No target host, edition licence, installed version, provider credentials or Novu environment has been supplied. [Compatibility manifest](../../Novu/deploy/compatibility.json) is intentionally unqualified with empty image pins and an empty release allowlist. No production Compose stack, running bridge, verified SDK lockfile or sync is claimed. This is a content/documentation delivery, not deployment authority.

## Clean installation checklist
1. Select and inspect an upstream self-hosted release. Record edition/licence, source commit, image digests, supported MongoDB/Redis versions, Node/Framework/CLI versions, migration notes, CVE review and resource estimates. Pin immutable digests and npm lockfiles. Do not deploy `latest`, `next`, unreviewed curl-to-shell installers or mixed release images.
2. Provision separate test and production installations: private networks, persistent data services, storage if needed, TLS endpoints, monitored backups, DNS and least-privilege service identities. Use production-appropriate database placement rather than the quick-start topology [S1].
3. Bootstrap a designated operator in a restricted session, disable open registration, protect the dashboard behind identity-aware access/MFA, and verify direct-origin paths are not bypasses. Set unique JWT/encryption/API credentials. Preserve and securely back up the provider-encryption key; rotating it without a supported migration can make stored provider credentials unusable.
4. Review telemetry and all egress; configure the narrow bridge allow rule required by the pinned version. Check signed bridge requests, API/socket URLs, CORS, TLS termination and reverse-proxy timeouts. Do not expose databases, Redis, worker internals or public trigger/admin endpoints.
5. Implement and lock the bridge host around the supplied workflow factory, including SDK signature handling, production NODE_ENV, body limits, liveness/readiness and private Go eligibility calls. Verify discovery and signed execution with the actual server/SDK pair.
6. Configure the dedicated provider integration, approved sender/reply-to, SPF/DKIM/DMARC, callback adapter, suppression, metadata correlation and separate test sink. Supply secrets outside Git. Provider setup is not included in workflow sync.
7. Use a private runner able to reach Novu and the bridge. Build/test the reviewed revision, deploy the bridge by immutable release, synchronise only the approved allowlist, compare discovered IDs/schema/release hashes and record the result.
8. Trigger synthetic end-to-end test messages, inspect delivered MIME, text/HTML, unsubscribe headers, callbacks and application permissions. Verify a failed provider send leaves financial outcomes unchanged. Obtain explicit production acceptance before enabling real recipients.

## Sync command contract
After the pinned CLI is installed and the target is qualified, run its documented sync operation. This illustrative shell uses an installed executable, not npx latest:
```sh
set +x
./node_modules/.bin/novu sync   --api-url "$NOVU_API_URL"   --bridge-url "$NOVU_BRIDGE_URL"   --secret-key "$NOVU_SECRET_KEY"
```
Run in an isolated, short-lived runner with protected secrets; the CLI key flag may be visible to the local process owner, so do not use a shared untrusted host or echo command arguments/logs. Check the selected CLI's supported safer credential mechanisms before adopting one. Never hardcode the Cloud API. Self-hosted sync is documented upstream [S1, S3].

A Git push does not deploy Novu or the bridge. A successful sync does not prove subscriber migration, provider configuration or delivery. Retain workflows and bridge source in Git after sync: runtime execution depends on the deployed code bridge. The export is not a universal Dashboard JSON import.

## Portable artefact contents
Release bundle: catalogue and content code, six themes/brand configuration, workflow factory, per-workflow schema/fixture exports, release allowlist, compatibility record, source hashes, tests and migration/runbooks. The offline build generates these review materials without credentials. Host/provider keys are referenced by name only and are injected at deployment.

Subscribers, contact mappings, preferences, historical deliveries, suppression records, integration credentials, environment IDs and dashboard settings are separate state. Back up/migrate them using supported APIs or storage procedures with counts, checksums and access review. An existing Dashboard workflow cannot automatically be overwritten by a Framework workflow with the same identifier; allocate a new ID and migrate triggers/preferences deliberately. Never delete an active workflow just to free its ID [S3].

## Promotion, drift and rollback
One reviewed content release moves through test and production; no test secrets or subscribers move with it. Compare the allowlist, workflow/schema hashes, bridge digest, provider identifiers and environment identity before sync. Disable dashboard-only live content edits operationally; record emergency exceptions and reconcile them to Git. Missing workflow IDs are not an instruction to delete production history.

Rollback deploys the last qualified compatible bridge/content release and re-syncs deliberately. Keep older workflow versions while queued events reference them. Never blindly roll back a database after new notifications/subscriptions exist. Pause dispatch first, preserve ambiguity and reconcile outstanding sends to avoid duplicates. Delete or migrate queues only under an explicit recovery plan.

## Recovery and upgrades
Back up MongoDB, required Redis durable state, provider-encryption/JWT/API keys, configuration, bridge images, brand assets and Go outbox/subscriber/suppression mappings. Encrypt backups and keep independently restorable off-host copies. Restore into an isolated network with outbound delivery disabled, validate identity/versions, reconcile in-flight messages, then enable a controlled canary. Measure and approve RPO/RTO; do not invent successful targets.

Upgrade first in non-production with the exact migration sequence and a verified restore point. Re-run bridge signature/discovery, text/header mapping, inbox authentication and delivery callbacks. Monitor queue age, provider acceptance/delivery latency, bounce/complaint rates, database/Redis capacity, worker errors, certificate/secret expiry and backup freshness. Independent monitoring must still notify operators when Novu, the bridge or the email provider is down.
