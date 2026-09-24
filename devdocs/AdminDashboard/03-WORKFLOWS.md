# Operational workflows and failure states

## Staff access
Offline first and second administrators bootstrap independently, then sign in and enrol MFA before seeing operational records. Later invitations are proposed, approved by another eligible administrator, handed over through an approved private channel, consumed once and followed by mandatory MFA. Expired, revoked, reused or no-longer-authorised grants fail. Invitation handoff is not an assertion of mailbox verification. Lost password and all MFA proofs require a recovery proposal approved by two eligible administrators neither of whom is the subject; approval immediately revokes old sessions. Consumption resets credentials and MFA, not the role or suspension. Last active administrator cannot be removed. Every privilege change revokes sessions.

## Customer and identity management
Search only permitted metadata. Open scoped detail, review private evidence with a purpose and propose KYC approval/decline/more information or restriction/restoration. A separate eligible reviewer checks the immutable proposal against the current subject version. Any changed subject invalidates stale approval. Internal notes are encrypted and not customer replies. Revoke customer sessions only with reason and current staff proof.

## Payments and bill recovery
Inspect financial state, related holds, journals, upstream observations, job lease and fulfilment separately. Re-query only a dead recoverable original job. A submitted/unknown payment cannot be retried as a new operation or manually marked successful. Delivered electricity/token values require reasoned access, are absent from exported metadata, and never imply another vend. Open refund/return investigations only against existing successful payments. These cases do not post compensation or certify refunds.

## Products and policy
Read the configured catalogue; propose disable/enable of an exact product ID and obtain independent approval. Product version changes invalidate stale controls and new execution checks honour disablement. Biller observations expire to unknown after five minutes and do not promise individual transaction success. Immediate-upon-approval policy updates retain version history; future-effective scheduling remains pending. Emergency stop blocks new acceptance, not investigation of existing obligations. Resumption requires independent approval against the current disabled version.

## Cases, reconciliation and reports
Support can assign eligible staff, exchange encrypted customer messages, read escalation history and access owned evidence. Risk and incidents have separate role boundaries, due dates, ownership, immutable notes and version-checked transitions. Resolved cases cannot be silently rewritten. Financial exception/refund/return investigations cannot change a ledger journal.

Normalise independently acquired bank/provider observations before import. Validate all rows and preview. Duplicate complete imports have one stored result. Unmatched entries remain available for investigation; matching one CSV is not three-way settlement certification. Exports are separately authorised and reasoned, max 5,000 rows; narrow the filter or commission an asynchronous full export when exceeded. The treasury page distinguishes ledger clearing, fees, liabilities and holds from externally verified bank custody and usable provider float.

## Notification operations
Read workflow/reference/state metadata, not secret bodies or challenge codes. Only non-secret queued/dead notices can be suppressed. Leased/unknown/submitted work cannot be blindly resent. The 175 versioned template metadata entries are generated from Novu's catalogue; edit/publish through reviewed repository content procedures. Actual provider delivery, bounce/complaint correlation and live Framework sync remain acceptance gates.
