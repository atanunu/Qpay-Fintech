# Operations, deployment and troubleshooting

## Deployment
Build APIbackend binaries and run the additive migrations with a migration principal before serving the changed routes. Do not edit migration 1 or 2. Build AdminDashboard API mode from its exact lock, set the approved API origin, review nginx CSP, terminate HTTPS and use a separate staff hostname. The static server listens unprivileged on 8080; no database credentials are present. Pin reviewed image digests and record build/source hashes in the deployment record. Prevent development/review hosts from indexing.

Start APIbackend API, worker and scheduler as separately supervised processes. Use least-privilege DB runtime roles and private scanning/storage acceptance before real customer evidence. Do not enable QPAY_CONTRACT_ACCEPTED, ACCOUNT_POLICY_ACCEPTED, notification allowlists or provider credentials through the admin interface. Those require independent approved deployment configuration.

## Rollback
Stop new payment acceptance when justified, preserving workers that query existing obligations. Roll back frontend assets first if the API remains compatible. Keep additive schema 3 and audit records; do not drop financial or evidence tables to undo a deployment. An older backend cannot safely operate unknown new roles/controls without a documented compatibility check, so prefer a forward fix or a tested compatible previous binary. Restore a backup only after reconciling all external obligations, never as a way to reverse a payment.

## Access incidents
Use My security for fresh proof and owned session revocation. A security administrator proposes staff suspension/role/recovery; another independently approves. A recovery grant is displayed once for private handoff. Never paste tokens, identity documents or passwords into audit notes. Lost all administrators is a separately audited infrastructure/security recovery incident, not public self-registration. Keep two independently controlled active security administrators before operations.

## Operational incidents
Investigate a payment's original reference, observations, journal and hold before recovery. Requery accepts only eligible dead jobs; conflict responses are not an invitation to resubmit. Suspense/treasury/refund views do not invent bank evidence. Create owned incident/risk/financial investigation cases with severity, due dates and immutable notes. Emergency resumption is independently approved and version checked.

## Troubleshooting
Origin denied: verify ADMIN_ORIGINS, the exact staff hostname/port and VITE_API_BASE_URL. CSRF rejected: restore the session; do not disable CSRF or convert web cookies into bearer credentials. Step-up required: reauthenticate using a fresh unused TOTP. 401: sign in again or reload for controlled access-token refresh. MFA code reused: use a fresh time step or an unused recovery code. 409: reload the current version; do not silently overwrite another operator's changes. 503: inspect configured capability and acceptance status; no synthetic success fallback exists. Export too large: narrow the selection; no silent truncation. Unknown notification state: correlate existing provider/Novu evidence, never blind resend.

## Monitoring and acceptance
Observe API request IDs and redacted error types, denied staff actions, age/count of pending approvals, dead payment jobs, aged unmatched reconciliation, held funds, outstanding support and failed notification intents. Assign alert owners; actual monitoring integrations and off-host backup/restore drills require deployment qualification. Logs must not contain credential/evidence bodies. Verify access revocation, containment/resumption and disaster recovery in a disposable environment before live activation.
