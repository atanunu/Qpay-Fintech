# GitHub repository setup and enforcement
Target: atanunu/Qpay-Fintech. Default branch: main. The repository was public and contained only its initial README when inspected on 2026-09-22. Do not change visibility, collaborators or secrets as a documentation side effect.

## Supplied in source
Documentation integrity workflow on main pushes, pull requests and manual dispatch; read-only contents permission; immutable checkout action; no persistent checkout credential; job timeout and cancellation; CODEOWNERS; feature/bug/PR templates; Dependabot for Actions; documentation regression tests and deterministic generated registers.

## Administrative setup still requiring verification
Require the `documentation-integrity` status check on main, disallow force pushes/deletion, require an appropriate independent reviewer and resolved conversations, and apply policy to privileged contributors as appropriate. The only current listed owner is atanunu; requiring independent review must account for an actual second authorised reviewer rather than creating an impossible rule. Add application checks as real manifests/tests exist. Enable appropriate security/private reporting settings and define a monitored private contact.

Workflow files and CODEOWNERS do not alone enforce protected branches. Record the ruleset ID, check context, reviewer configuration and a safe test PR result when settings are actually configured. This baseline does not claim settings activation. Do not manufacture a required green application check for a nonexistent app.

## Issue and milestone setup
Use M0-M6 from the roadmap and stable API/MOB/ADM/WEB IDs. Create implementation issues as tasks are assigned; keep Unassigned explicit until an owner accepts responsibility. Link PRs and commit/test evidence back to the canonical README. Do not count a board card as implementation, or duplicate an epic and its children in progress totals. Documentation delivery does not claim that 114 issues or a Projects board have been created.

## Publishing safety
Review current main immediately before publishing; use a parent-aware non-force update. A conflict requires reconciliation, not replacement. Verify the remote tree, commit and actual Actions runs. Read-only documentation checks do not publish generated changes, deploy services or access production data.
