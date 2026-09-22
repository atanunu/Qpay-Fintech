# Notification validation — 22 September 2026

Scope: built-in content and documentation, not a deployed notification system.

| Check | Result |
|---|---|
| `npm --prefix Novu test` | 207 offline Node tests pass; zero failures/skips. |
| `npm --prefix Novu run build` | 164 HTML/text messages, strict schemas, fixtures, catalogue and preview index render. |
| `npm --prefix Novu run check` | Catalogue digest/counts and all template renderability validate. |
| Repository documentation checks | 28 documentation regression tests pass; five services, 141 task rows and 411 local destination references validate. |
| Browser preview | Actual synthetic content rendered in Chromium; provenance records source hashes. |
| Real Novu Framework/server integration | Not executed; factory tests use a contract harness. |
| Provider, DNS, real MIME, live sync, callbacks | Not executed. No external message was sent. |
| Financial application tests/deployment | Not executed in this notification-content task. |

The target version/edition record is deliberately unqualified. Content and scenario coverage do not attest to licensing, consent, installed compatibility, payment execution, secure live delivery or production acceptance. CI also must be checked on the published commit, rather than inferred from this report.
