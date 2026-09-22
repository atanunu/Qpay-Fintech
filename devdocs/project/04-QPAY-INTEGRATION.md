# Relationship to atanunu/QPay
Status: E1 server-to-server integration approved on 22 September 2026. This does not establish current compatibility, partner acceptance or live activation.

## Source review finding
The reviewed QPay TransferDev and BillPaymentDev indexes describe older Go acceptance/Laravel execution/shared-database designs. The newer project architecture and 21 September handover state the target as dashboard configuration/observation with Go execution, while explicitly identifying remaining legacy bridges and activation gates. Adopt the documentation discipline, not both incompatible execution models.

References: [QPay TransferDev](https://github.com/atanunu/QPay/blob/main/devdocs/TransferDev/00-INDEX.md), [QPay architecture](https://github.com/atanunu/QPay/blob/main/devdocs/project/ARCHITECTURE.md), [handover](https://github.com/atanunu/QPay/blob/main/setup-design-docs/ai-context/DOCUMENTATION_HANDOVER_2026-09-21.md). Reviewed 22 September 2026; re-check current code and commit before implementation. Main is mutable.

## Options
| Code | Model | Trade-off |
|---|---|---|
| E1 approved | New product consumes QPay's versioned server-to-server APIs | Reuses execution investment but depends on current QPay capability, reliability, contractual authority and acceptance |
| E2 | New Go engine integrates directly with providers | More independence but duplicated adapters, operational controls, certification and maintenance |
| E3 | Explicit hybrid | Later flexibility with greater routing, accounting, recovery and operational complexity |

No shared database access. Keep credentials in the APIbackend/worker environment, never MobileApp/WebApp. QPay's own platform accounting and this product's customer subledger represent different obligations; document how they reconcile. If QPay is chosen as the authoritative customer-ledger provider instead, revise the ADR and remove competing wallet writers before building.

## Mandatory integration audit
For each required capability record: upstream endpoint/version; operation semantics; authentication and key rotation; environment; provider/capability actually selected by runtime; input/output schema; idempotency scope and retention; accepted/pending/final states; callback authentication and replay controls; original-reference status lookup; funding match fields; reconciliation export; errors; limits; sandbox result; partner acceptance; live activation approval. A settings entry or adapter file is not an executable integration.

QPay's bill spec names Quickteller/Interswitch and CoralPay VAS as intended providers. This plan does not infer live acceptance, negotiated prices, credentials or all-biller availability from that spec. Reuse selected components only after code, licence, dependency, tests and boundary review; do not import demo authentication or local-only state into the retail product.

## Ownership and routing rules
Select a route before external submission. Persist the route and upstream reference with the intent. Never switch an unresolved request to another upstream just because the first timed out. Reconcile the original operation or use a supported, verified cancellation/finality procedure. Provider query retries are different from payout retries. Callback replay is different from a new financial instruction.

Create consumer-driven contract tests with synthetic fixtures; run staging qualification independently. No live credentials, production route changes or money-moving tests are authorised by this planning document.
