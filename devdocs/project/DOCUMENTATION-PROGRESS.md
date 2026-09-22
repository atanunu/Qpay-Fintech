# Documentation and repository foundation progress
As of 2026-09-22. These are documentation tasks, not additional product features and not part of the 114 application-task denominator.

| ID | Item | State | Evidence / next action |
|---|---|---|---|
| DOC-001 | Approved four-app stack and E1 boundary | Done | ADR-0001 |
| DOC-002 | Mandatory root/service README standard | Done | ADR-0002 and documentation policy |
| DOC-003 | Four service registers and derived progress | Done | 114 Planned application tasks; generator and regression tests |
| DOC-004 | Shared product/security/financial/operations plans | Done for design baseline | Project documents; executable specifications remain below |
| DOC-005 | Application/contracts/infra/test directory structure | Done for scaffold | Tracked workspace READMEs; no runtime code implied |
| DOC-006 | Five rendered target diagrams and capture policy | Done | Deterministic SVG/source validation; runtime capture manifests empty |
| DOC-007 | Documentation CI and contribution templates | Source complete | Verify exact remote workflow run after publishing |
| DOC-008 | Required branch rules and independent reviewers | Pending | Repository administrator must configure and verify settings |
| DOC-009 | Full domain physical schemas and executable contracts | Pending | Complete module-specific specs and reviewed contract tests before each feature |
| DOC-010 | QPay capability audit and native SDK proof | Pending | Pinned source/API audit and real Android/iOS proof |
| DOC-011 | Product screenshots and executed application evidence | Pending | Capture/test actual apps when implemented; never substitute designs |
| DOC-012 | Provider/security/financial/release acceptance | Pending | Separate evidence and human approval; no live activation |

"Done for design baseline" means the relevant planning document or scaffold exists and was checked. It does not mean its proposed runtime controls, integrations or tests have been implemented. Source completion of CI is separate from a verified GitHub run and from enforced branch rules.

## Self-hosted notification addition
Added accepted ADR-0003, `devdocs/Novu/` index plus eight specifications, 164 email scenarios, six local themes and a canonical 27-task Novu register. Original 114 application statuses are unchanged. Local content is implemented; actual orchestration hosting, delivery, provider and release gates remain open.

See the [Novu specification index](../Novu/00-INDEX.md) and [service task register](../../Novu/README.md).
