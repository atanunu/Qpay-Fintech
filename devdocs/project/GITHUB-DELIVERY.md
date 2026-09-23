# GitHub delivery verification — 23 September 2026

The recovered WebApp and supporting persistent Go APIbackend are published as a coherent development-source delivery. Complete before/after hash checks recovered 100 file records; the truncated documentation tail was rebuilt and canonical progress regenerated. This is not claimed to be a byte-identical copy of the former local workspace.

## Verified revision and runs

Application head: `254f586b65223ed4db3d4e2df9c209e6e5ee3f58`. Screenshot source: `4cd7960e41dda6d5ba6f6d05fcfb2fbf0294f04a`, with the identical Git tree.

| Gate | Result | Evidence |
|---|---|---|
| Frontend TypeScript and unit/contract tests | Passed; 79 tests | Web workflow |
| API-connected and standalone review builds | Passed; no implicit demo fallback | Web workflow |
| Synthetic browser journeys | 20 passed; no failures, skips or retries | Web workflow |
| Actual browser → Go → PostgreSQL journeys | 11 passed; no failures or skips | Web workflow |
| Runtime screenshots/accessibility | 38 captures; axe and overflow assertions passed | Screenshot manifest |
| Go module checks, vet, race/PostgreSQL tests and builds | Passed | Backend workflow |
| Shared transport helper tests | Seven passed | Backend workflow |
| Docker Compose API/worker/scheduler smoke | Passed, synthetic funds only | Backend workflow |
| Novu content checks/build/tests | Passed; 218 tests and 175 catalogue scenarios | Documentation workflow |
| Documentation integrity/governance/regressions | Passed; 29 regression tests | Documentation workflow |

- [Web workflow](https://github.com/atanunu/Qpay-Fintech/actions/runs/35838616223)
- [Backend workflow](https://github.com/atanunu/Qpay-Fintech/actions/runs/35838616217)
- [Documentation workflow](https://github.com/atanunu/Qpay-Fintech/actions/runs/35838616287)

[Machine-readable verification](DELIVERY-VERIFICATION.json) · [Recovery hashes](DELIVERY-SOURCE-MANIFEST.json) · [WebApp screenshot gallery](../../WebApp/docs/SCREENSHOTS.md).

## Corrections found through actual browser testing

Native fetch binding was corrected in both clients. Customer capability rendering now uses payments rather than an absent transfers field. Quote buttons wait for loaded recipient/product records; the recovery test waits for the destination screen. OpenAPI parameter duplication was fixed with a regression preserving concrete schemas. No assertions were removed and browser retries remain disabled. Historical failing runs are not labelled passed.

## Remaining acceptance

Source delivery is not a production launch. In-person UAT remains not-tested. Actual QPay/provider qualification, live self-hosted Novu delivery, production private storage/scanner acceptance, refunds/returns/full reconciliation/treasury, SMS/automatic KYC and the documented remaining engineering/product gates remain outstanding. MobileApp and AdminDashboard remain planned scaffolds. No real-money transaction or production deployment was performed. Keep these limitations in the service task registers rather than promoting every feature to complete.
