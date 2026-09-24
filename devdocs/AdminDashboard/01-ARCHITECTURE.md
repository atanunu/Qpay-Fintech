# Architecture and permissions

The React console is a static, staff-only application. It knows no database/provider credentials. `src/api.ts` sends only relative `/v1/admin/` paths to an explicitly approved HTTPS origin. A missing origin uses an invalid non-routable example, not the website's origin. The API authenticates cookie audience, transport, CSRF, MFA, current role/status and session, with transactional rechecks on domain mutations.

There are 26 allowlisted SQL resource projections: customers, staff, invitations, payments, KYC, financial proposals, operational controls, support, jobs, notification intents, audit, reconciliations, unmatched exceptions, journals, ledger accounts, funding, mandates, money requests, reminders, risk, incidents, refund/return investigations, privacy closures, product controls, biller observations and exports. No raw `SELECT *` from secret-bearing user/session/notification tables is exposed.

| Role | Primary scope | Denied examples |
|---|---|---|
| admin | All console resources; independently controlled security operations | Self approval, own access recovery, unverified financial execution |
| finance | Customer/payment metadata, ledger/books, reconciliation, financial exceptions and policy | Staff privilege, identity-document access, risk files |
| compliance | Customer/KYC, risk, request metadata, audit, restrictions and privacy | Treasury, staff privilege, support conversations |
| support | Customer/payment metadata, support, funding state, schedules, requests, reminders, bill availability | Staff, ledger accounts, reconciliation, identity documents, export |
| platform | Jobs, product controls, bill availability, incident and notification operations | Customer/payment directory, staff directory, bank custody, identity evidence |
| auditor | Redacted operational/financial/audit records and reasoned exports | Every mutation, risk files, support conversation, decrypted identity |

The complete executable matrix is `APIbackend/internal/service/admin_resources.go`. Browser navigation is only a reflection; crafted URLs or requests do not change server decisions. Approval queues are further filtered by the current reviewer's domain. Product/mapping identifiers are adapter-owned; disabling affects both future quotes and execution of existing authorisations. An idempotent replay of an already committed payment still returns the original outcome.

Mutation control snapshots are immutable at database level. No direct customer balance update exists. Encrypted internal notes are append-only. Separate staff and customer cookie names/allowlists reduce cross-application privilege leakage; host-only Secure cookies are used outside local mode. Browser private data and credentials are not cached in localStorage, IndexedDB or a service worker. Theme preferences alone may persist.

Private evidence requires an eligible role, explicit purpose, fresh staff proof and audit; read-only auditors get metadata, not private bodies. Related detail sets are bounded: sessions 100, provider observations 200, support events 500, schedule occurrences 120, notification events 200. These detail samples are not a complete legal export; use scoped operational exports or an approved backend investigation for larger histories.
