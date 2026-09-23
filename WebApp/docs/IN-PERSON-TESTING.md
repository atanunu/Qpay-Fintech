# In-person test procedure

## Two explicitly different modes
**Synthetic UI review:** run `npm run dev:review`, or open the generated standalone `qpay-offline-review.html`. Use Fill sample sign-in. Initial email is review@qpay.example.invalid, fixed synthetic password Review-only-2026, code/PIN 123456, and internal recipient usr_review_tunde_02. The password stays fixed in the UI harness even during password-change/recovery rehearsals. This mode demonstrates UI states, not real authentication/provider security.

Synthetic records and the selected scenario are kept in the current browser tab; no real API or provider is called. Use only made-up identities/documents. Reset removes the synthetic operation state and signs out. Appearance is the only localStorage preference. An offline HTML copy is not the live service and must not be distributed as one.

**API-connected review:** run the included APIbackend in its explicit local profile and `npm run dev` with `VITE_API_MODE=api` and `VITE_API_BASE_URL=http://localhost:8080`. Open `http://localhost:5173`, not a different hostname. Use fresh synthetic accounts seeded by the controlled backend CLI. This mode uses the real Go service, PostgreSQL, session cookies, CSRF, ledger, holds and workers. Its local bank/bill adapter remains synthetic. No real QPay or Novu provider should be enabled for this session.

The web app does not switch modes following a timeout or 503. Configure a separate explicit review build instead. Financial submits are allowed only against reported local/staging environments; production reports are blocked by this release, with independent backend controls still mandatory.

## Session workflow
Start in the Review workspace. Name the session with non-sensitive text. The 24 UAT rows start as Not tested. Follow registration/login, account readiness, bank/internal transfer, bill validation/value, interruption recovery, receipts/statements, security/devices, support and notification journeys. Exercise success, pending, definitive failure, response loss, expiry, insufficient-funds, restriction and outage cases. Record expected versus observed behaviour and the environment used.

Review gaps are a separate category: actual partner funding, card/USSD collection, verified SMS, automatic identity-provider verification and full all-record privacy export remain unavailable. Digital document submission, dual-mailbox email changes and guarded closure now have actual backend operations and must be tested as such. A support dispute must not claim a refund was executed. Classify a row as blocked when the API or operating decision is missing rather than marking the screen's appearance as end-to-end success.

Export the JSON notes before ending the tab. The export includes mode, capability report, manual results and the 14 integration gaps. Do not put PINs, passwords, recovery codes, customer documents, bank numbers or other personal data into notes, traces or screenshots. Clear test state between reviewers and revoke test sessions when finished.

## Acceptance sequence
1. Browser-only UI/keyboard/mobile review using synthetic fixtures.
2. Browser → real local API → PostgreSQL/synthetic worker acceptance.
3. Review the gap register, update backend contracts and close each agreed item with tests.
4. Qualify a selected QPay sandbox contract and approved provider path independently.
5. Security, legal, treasury, recovery and operational approval before production money movement.

Do not count steps 1 or 2 as evidence that step 4 or 5 passed. No external payment, email or deployment authority is implied by this test plan.

## New parity journeys
Saved bills and reminders are server-backed in API mode. Internal schedules require explicit caps and new approval after pause; external autopay is unavailable. Request participants see only their own share, and cancellation is rechecked when paying. Monthly insights use complete posted-ledger queries. Test passkeys on the real configured origin; synthetic mode never pretends to perform cryptographic ceremonies. Only local test documents are allowed in local_unscanned mode. Cosmetic masking is visual convenience, not data access control.
