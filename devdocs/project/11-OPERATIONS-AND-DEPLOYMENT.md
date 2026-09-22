# Operations and deployment plan
Status: no deployment performed, infrastructure or hostnames selected, or production changes authorised.

## Environments
Isolate local/test, integration/sandbox, staging and production. Use synthetic fixtures and environment-scoped keys/accounts. Docker and private service DNS are proposed; Cloudflare Tunnel may be selected after topology and threat review. Public edge protection must not be the only authentication layer. Keep databases and privileged APIs on private networks where feasible.

Deploy API, workers, scheduler, admin and web separately with compatibility-aware releases. Use health, readiness, queue lag and dependency signals. A successful process health endpoint does not mean payments can safely execute. Block live execution without required provider/ledger/security configuration.

## Persistence and disaster recovery
Back up databases, private documents, configuration versions and encryption-key recovery material through approved independent controls. Define RPO/RTO and demonstrate restoration to a replacement environment. Reconcile money and pending operations after restoration; never blindly replay external payment submissions. A backup on the same host is not sufficient disaster recovery evidence.

## Observability
Structured redacted logs, request/intent correlation, traces, event lag, pending-age alerts, reconciliation breaks, ledger balance invariant checks, provider error trends, float thresholds and notification failures. Define alert owner, escalation and runbook. Provide separate business/financial and infrastructure metrics. Record metric definitions, not invented dashboard totals.

## Required runbooks
Provider outage; ambiguous transfer; delayed bill token; duplicate funding; mismatched settlement; low provider float; suspected account takeover; key rotation/revocation; lost staff factor; controlled emergency access; incident/customer communications; database failover; restore; queue backlog; deployment rollback; mobile API incompatibility and account access recovery.

Migration rollout uses backward-compatible expand/contract changes where needed and reviewed rollback/roll-forward plans. Do not delete ledger history in a rollback. Test old/new clients against supported API versions. Signing keys, package identifiers, store accounts, distribution approvals and support contacts remain to be established.

Branch protection, required CI checks, deployment environments and secret permissions must be installed and verified in GitHub; merely documenting them does not enforce them. No such settings are changed by this pack.
