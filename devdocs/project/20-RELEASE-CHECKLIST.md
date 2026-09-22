# Release acceptance checklist
Status: not passed. Documentation/scaffold delivery is not a live product release. Every item needs an accountable owner, immutable evidence reference, source revision, environment, reviewer and acceptance date. A checkbox here is not permission to move money.

## Product and operating authority
- [ ] Legal entity, terms, privacy/consent notices and customer eligibility approved.
- [ ] Custody/account model, partner permissions, contracts, complaint responsibilities and loss allocation accepted.
- [ ] Customer fees, limits, disclosures and support commitments approved.

## Financial and integration correctness
- [ ] Balanced append-only ledger and atomic reservations proven against real PostgreSQL.
- [ ] Duplicate, concurrent and crash-recovery tests executed for every money path.
- [ ] Provider timeout then late success, duplicate/reordered callback and original-reference query qualified.
- [ ] Funding, transfer, bill fulfilment, return/refund and settlement states tested separately.
- [ ] Three-way reconciliation and exception ageing accepted by finance.
- [ ] Selected QPay runtime capabilities qualified at a pinned revision and environment; adapters alone are insufficient.

## Security and privacy
- [ ] Staff MFA, scoped authorization, independent approvals and secure recovery qualified.
- [ ] Mobile/web/API threat model and independent security findings remediated or explicitly accepted.
- [ ] Data retention, restricted exports, private documents, legal holds and key rotation accepted.
- [ ] Dependency/secrets checks, signed builds and production configuration validated.

## Operations and distribution
- [ ] Load/latency/pending-age objectives measured; alert ownership and on-call/support escalation rehearsed.
- [ ] Independent restoration of data, keys and unresolved operations meets approved RPO/RTO.
- [ ] Migration compatibility, rollback and stop-traffic procedures rehearsed without erasing financial history.
- [ ] Android/iOS signed builds and store/provider SDK acceptance completed; responsive web accessibility verified.
- [ ] Production rollout explicitly authorised. No implicit activation from a documentation merge.

## Documentation and repository
- [ ] Root and service READMEs reflect real source and test evidence; screenshots use synthetic data with provenance.
- [ ] Documentation/app checks pass on the exact release revision; branch rules and reviewers verified.
- [ ] Open blockers, support runbooks, release notes, rollback owner and provider incident contacts recorded.
