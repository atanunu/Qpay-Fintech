# WebApp verification record

Initial local execution: TypeScript strict typecheck, API/review/offline build paths, and 58 Vitest unit/contract/review-adapter tests. Local Chromium rendered the actual React components using a MemoryRouter and injected in-memory browser-storage harness because navigation is restricted in the working environment. Desktop/mobile screens and transfer stages produced zero uncaught page errors and zero WCAG-tagged axe violations in that harness after corrections.

That local rendering is **not** real API-cookie or provider acceptance. The committed Playwright suites separately exercise a normal HTTP origin and a real Go/PostgreSQL local API in GitHub CI. Final run/commit IDs and result totals must be attached after execution; they are not inferred from the local tests.

Test layers: exact money/date validation, response-schema guards, cookie/CSRF transport, read-only refresh recovery, no write replay, idempotency/reference recovery, synthetic scenarios; browser auth/payment/bills/support/settings/statement journeys; automated accessibility and responsive captures; API-connected browser tests with real PostgreSQL and synthetic execution. No external banking or email service is called.

Remaining acceptance: in-person reviewer sign-off; all GAP decisions; actual same-site deployment and additional supported browsers/devices; real Novu delivery; real QPay sandbox/provider evidence; production financial, legal, security and independent recovery approval.
