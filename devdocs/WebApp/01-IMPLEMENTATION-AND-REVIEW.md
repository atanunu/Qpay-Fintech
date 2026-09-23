# Customer WebApp implementation and review plan

The user approved building a complete customer interface now: wire available APIbackend capabilities, make missing-API journeys fully reviewable and explicit, then adjust contracts during in-person testing before connecting actual transaction execution through QPay.

Architecture: React + TypeScript + Vite + React Router. A single selected transport is either the real cookie-authenticated Go API or a separately compiled synthetic review adapter. No API failure activates a simulator. Financial authority stays in Go. No bank or Novu master credential is sent to the client.

Implemented surfaces: independent login/register/verification/recovery/help screens; overview; wallet/funding options; internal/bank transfer; quote/approval/recovery; bill discovery/validation/vend/value; activity/receipt; beneficiaries; statements; notifications; verification/manual evidence/digital draft; profile/security/MFA/devices/preferences/privacy; support/complaint/dispute conversations; review checklist and gap export. Unsupported funding, digital KYC, contact, privacy and closure actions remain visibly unsubmitted.

Use [API mapping](../../WebApp/docs/API-MAPPING.md), [in-person plan](../../WebApp/docs/IN-PERSON-TESTING.md), [security/operations](../../WebApp/docs/SECURITY-AND-OPERATIONS.md) and the [canonical register](../../WebApp/README.md). The 14 GAP items identify the needed backend contract or product decision. Keep each new decision in an ADR and update root/service status with actual evidence.

Verification must distinguish: local unit tests; synthetic-browser UI tests; real browser/Go/PostgreSQL tests with local execution; upstream QPay sandbox acceptance; and approved production execution. The first three do not imply the latter two. The existing backend working source is reconciled into the test branch so the web app and API can be evaluated together; it retains its documented launch-blocking gaps rather than being relabelled fully complete.
