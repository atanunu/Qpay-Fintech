# Testing and evidence

The authoritative execution record is [AdminDashboard/docs/VALIDATION.md](../../AdminDashboard/docs/VALIDATION.md). Do not count a docs/workspace job as application verification.

Frontend: strict TypeScript; unit tests for transport, BigInt money, normalised import, navigation matrix, explicit review mode, self-approval and unsupported actions. The API bundle must exclude the synthetic credential marker.

Backend: real PostgreSQL migration 3, six-role resource matrix, revoked/expired/current sessions, CSRF/origin separation, denied customer tokens, unchanged encryption boundaries, independent invitation/recovery, token reuse, stale/immutable approvals, disabled-product execution and concurrent duplicate imports. Existing WebApp and money/race tests remain mandatory regressions.

Normal browser review: all management modules, three authentication layouts, light/dark/mobile/alternate layout, accessibility and overflow, search/export/dialog focus, notes and approvals, invitations, support, original requery, reconciliation, emergency controls and role-denied routes. Runtime captures are made only after actual page rendering. Static/synthetic screen rendering is not evidence of actual bank or email delivery.

Real API browser suite: staff cookies/restoration, MFA gate/enrolment, direct CSRF and role denials, private-document access, support assignment/versioning, reconciled import deduplication, invitation/checker acceptance, controlled exports, biller observations, customer restriction and logout. The fixture uses the actual CLI and staff APIs, not test-only authentication.

CI disables authentication traces, videos and automatic failure screenshots to avoid capturing ephemeral cookies, passwords, recovery tokens or document content. A separate review-only capture mode uses intentionally public synthetic fixture records; hashes and exact source revision accompany screenshots. No live financial/provider calls are made. In-person acceptance, penetration testing, production load/recovery and provider qualification are separate recorded release gates.

## Recorded qualification

The development candidate at f7e91005d83fcedc56ecec1da4fa6c47c251a52f passed normal GitHub Actions Chromium: 19 review journeys, 11 real Go/PostgreSQL journeys, 63 unit/contract tests, and container routing/security headers. Forty-one source-bound synthetic captures are committed in [the gallery](../../AdminDashboard/docs/SCREENSHOTS.md). [Exact revisions and scope](../project/ADMIN-DELIVERY.md). This finite Chromium suite is not all-browser, independent security or production acceptance.
