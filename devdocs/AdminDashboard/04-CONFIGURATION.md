# Configuration and initial staff setup

Frontend build variables (public, never secrets):

| Variable | Required meaning |
|---|---|
| VITE_ADMIN_MODE | api by default; review only for deliberate synthetic review |
| VITE_API_BASE_URL | Exact approved HTTPS API origin, without path, credentials, query or fragment |
| VITE_ALLOW_LOCAL_HTTP | true only for local localhost/loopback testing |
| VITE_REVIEW_ACK | synthetic-data-only, required by review mode |

Backend variables: `ADMIN_ORIGINS` exact staff origins; `WEB_ORIGINS` separate customer origins; all existing database, encryption, execution, notification and private-object settings remain backend-only. Example local values: ADMIN_ORIGINS=http://localhost:5174 and WEB_ORIGINS=http://localhost:5173. Do not mix localhost and 127.0.0.1 in browser endpoints. Never use wildcard origins or cross-site third-party cookie workarounds. Non-local missing staff origins deny access.

Start the documented APIbackend database/migration/API/worker stack. Migration 3 is additive. Run first-admin and second-admin bootstrap with `qpfctl bootstrap-admin` and `qpfctl bootstrap-checker`, respectively. The second command works only while exactly one staff account exists. Supply `QPF_CTL_EMAIL`, `QPF_CTL_NAME` and `QPF_CTL_PASSWORD_FILE` pointing to a newly generated owner-only file. Do not pass a password on the command line or commit a shared initial password. Sign in and enrol both authenticators; retain recovery codes privately. Use the independently approved invitation interface thereafter.

The acceptance fixture `AdminDashboard/e2e/api-fixture.py` demonstrates these exact CLI + HTTP workflows against a fresh disposable local database. It does not modify a production account or add a bypass endpoint. It writes ephemeral mode-0600 fixture data under .test-fixtures, which is excluded from source and artifacts.

Review fixtures are in-memory only. Reload clears their mutations. Review passwords/code examples are not accepted by the Go service and are tested for exclusion from the API build. The normal static deployment Dockerfile always builds api mode. Compile-time API origin and nginx connect-src must match; replace the deliberate invalid CSP example before real test deployment.
