# Initial risk register
All risk ownership roles below are proposed; accountable individuals and acceptance decisions remain unassigned.

| Risk | Severity | Owner role | Control and closure evidence |
|---|---|---|---|
| Unclear legal/custody model | Critical | Business/compliance | Executed partner agreement and reviewed operating authority before live funds |
| Two payment/ledger writers | Critical | Backend/architecture | One documented product ledger owner and explicit upstream obligation mapping |
| Duplicate payout after timeout | Critical | Backend | Durable original-reference recovery, no unsafe failover, race/crash tests |
| Reconciliation deferred until after launch | Critical | Finance/backend | Three-way reconciliation and aged-break workflow in vertical slices |
| QPay adapter mistaken for live support | High | Integrations | Current runtime wiring, contract and financial qualification evidence |
| Unsafe recovery/account takeover | Critical | Security | Independent identity/transaction authorisation, device/session controls and recovery abuse tests |
| Native SDK incompatibility | High | Mobile | Selected provider SDK works on signed Android/iOS builds before release acceptance |
| Public leak of identity/provider secrets | Critical | Security | Synthetic docs, private storage, secret scans and reviewed repo visibility |
| Documentation drift/fake readiness | High | Engineering | Canonical register, generated checks, reviewed source/test/provider evidence |
| Provider float exhaustion | High | Treasury | Thresholds, alerts, funding controls and defined fail-safe behaviour |
| Backup cannot restore pending money | Critical | Operations | Off-host/key recovery rehearsal and post-restore reconciliation |
| Growth scope overwhelms core quality | High | Product | Approved milestone boundaries and separately gated products |
| Fraudulent referrals/cashback liabilities | High | Risk/finance | Eligibility and clawback rules, liability accounting, fraud controls before rewards |
| Unsupported old app loses access | High | Mobile/backend | API compatibility policy, migration tests and support access path |
