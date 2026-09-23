# Browser delivery verification

## Accepted automated baseline

Application source head `254f586b65223ed4db3d4e2df9c209e6e5ee3f58` passed all three GitHub workflows: web run **35838616223**, backend run **35838616217**, and documentation/Novu run **35838616287**. There were 79 frontend unit/contract tests, 20 synthetic browser tests, 11 actual Go/PostgreSQL browser tests, seven shared client tests, 218 Novu content tests, and 29 documentation regression tests. The full Go race/PostgreSQL suite, builds, contract export and Docker Compose smoke passed. Browser tests had no failures, skips or retries.

Evidence-only commit `6504923011cf981c8383d5ceda9e3e3972cc902e` published all 38 runtime PNG captures, a source/UTC/viewport/SHA-256 manifest, screenshot gallery, root/service README updates and machine-readable verification. The evidence publisher verified that the screenshot source Git tree exactly matched the tested application head. Raw traces, credentials, failure snapshots and private documents were not copied into the repository. This commit reruns CI over the complete source and documentation package before merge.

## Historical failures and corrections

Earlier browser runs exposed incorrect native fetch receivers, an absent capability field, and asynchronous recipient/product/recovery-screen readiness. Additive regression tests and explicit readiness checks fixed these; assertions were not removed and automatic retries remain disabled. Run 35837962795 was an intermediate result (11 API journeys passed; 18 review tests passed and two failed). It is superseded by the green baseline above, not relabelled as passed.

## Acceptance boundary

Manual in-person UAT remains not-tested. Real QPay/provider qualification, live self-hosted Novu delivery, production private-store/scanner acceptance, financial refund/return/treasury work and the service remaining-work registers remain separate. This is verified development-source delivery, not a real-money or production launch.

[Exact CI links and source recovery](GITHUB-DELIVERY.md) · [Machine-readable evidence](DELIVERY-VERIFICATION.json) · [38-screen gallery](../../WebApp/docs/SCREENSHOTS.md) · [Web register](../../WebApp/README.md) · [API register](../../APIbackend/README.md)
