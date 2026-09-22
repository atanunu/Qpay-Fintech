# Identity and permission boundaries
Status: design baseline; enforce in APIbackend during implementation. Roles below are proposed job functions, not provisioned accounts. No default admin password or open staff registration.

| Capability | Customer | Support | KYC analyst | Finance proposer | Finance approver | Security administrator | Auditor |
|---|---|---|---|---|---|---|---|
| Own profile/history/payments | Own resources only | Masked case-scoped read | No | No | No | No | Policy-scoped read |
| View identity evidence | Own workflow | Masked status only | Assigned cases with reason | No | No | No routine access | Approved audit scope |
| Approve KYC tier | No | No | Permitted tier/policy | No | No | No | Read decisions |
| Investigate transaction | Own detail | Assigned case | Risk-relevant scope | Financial scope | Financial scope | Security metadata | Read only |
| Propose refund/correction | Request review | Open case only | No | Yes with reason/evidence | Separate proposer identity required | No | Read only |
| Approve correction | No | No | No | Cannot self-approve | Independent approval within limit | No | Read only |
| Change pricing/limits | No | No | No | Propose | Independent approval | No | Read versions |
| Freeze/restrict account | No | Escalate | Escalate | Policy-defined | Policy-defined | Security containment within policy | Read reasons |
| Grant privileged roles | No | No | No | No | No | Independent approval; no self-escalation | Read role history |
| View provider secrets | No | No | No | No | No | Rotation through secret store, not UI reveal | No |
| Export sensitive records | Own data policy | Approved case export | Approved case scope | Approved finance export | Approved finance export | Security incident scope | Approved audit export |

Deny by default. Evaluate resource ownership, organisation scope if introduced, role, monetary authority, KYC/risk restrictions, MFA freshness and transaction-bound authorization server-side. Hidden buttons are not authorisation. Record actor, subject, reason, old/new policy version, evidence and correlation ID without logging credentials or raw identity documents.

Every privileged mutation requires an audit event. Financial corrections use immutable compensating journals. Proposal and approval must be different identities, and material changes invalidate approval. Break-glass access is time-bound, independently reviewed and cannot silently bypass ledger invariants. Define concrete limits and incident ownership before rollout.

Required tests: cross-customer object access; staff/customer token audience confusion; self-approval; replayed/expired approval; changed transaction details; revoked role/session; bulk export scope; support impersonation denial; direct balance-edit prohibition. Retention, lawful access and reporting obligations require operating-model review.
