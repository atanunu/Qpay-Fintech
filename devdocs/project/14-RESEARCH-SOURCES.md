# Research and source register
Reviewed 22 September 2026. Links are provenance; mutable upstream main/docs must be re-checked before implementation. Recommendations are design judgements, not copied claims of readiness.

## Repository sources read through the connected GitHub tool
| Source | Finding used |
|---|---|
| [New repository contents](https://github.com/atanunu/Qpay-Fintech) | Initial root README only at inspection; no product implementation inferred |
| [QPay AGENTS](https://github.com/atanunu/QPay/blob/main/AGENTS.md) | Code/reference separation and current agent/documentation requirements |
| [QPay CONTRIBUTING](https://github.com/atanunu/QPay/blob/main/CONTRIBUTING.md) | Combined code/test/docs/screenshots delivery and honest evidence |
| [QPay project index](https://github.com/atanunu/QPay/blob/main/devdocs/project/PROJECTS.md) | Canonical service READMEs, stable IDs, derived JSON and non-additive counts |
| [QPay architecture](https://github.com/atanunu/QPay/blob/main/devdocs/project/ARCHITECTURE.md) | Target Go execution, remaining legacy bridges and known readiness debt |
| [TransferDev index](https://github.com/atanunu/QPay/blob/main/devdocs/TransferDev/00-INDEX.md) | Nine-part-style module discipline, older split-execution assumptions |
| [BillPaymentDev index](https://github.com/atanunu/QPay/blob/main/devdocs/BillPaymentDev/00-INDEX.md) | Catalogue/validation/vend/recovery and intended Quickteller/CoralPay integrations |
| [Documentation handover](https://github.com/atanunu/QPay/blob/main/setup-design-docs/ai-context/DOCUMENTATION_HANDOVER_2026-09-21.md) | Image provenance, true Mermaid validation and source versus live acceptance |

This was a targeted documentation/architecture review, not an exhaustive line-by-line audit of QPay code or proof of current provider readiness. Do not import historical capability counts as this project's completion.

## Official technical sources
[React Native setup](https://reactnative.dev/docs/environment-setup), [Expo development builds](https://docs.expo.dev/develop/development-builds/introduction/), [Expo SecureStore](https://docs.expo.dev/versions/latest/sdk/securestore/), [Flutter architecture](https://docs.flutter.dev/resources/architectural-overview), [Go chi](https://github.com/go-chi/chi), [Gin](https://gin-gonic.com/en/docs/), [React app architecture](https://react.dev/learn/creating-a-react-app), [Next.js](https://nextjs.org/docs), [SvelteKit](https://svelte.dev/docs/kit/introduction), [Laravel frontend](https://laravel.com/framework/docs/13.x/frontend), [PostgreSQL isolation](https://www.postgresql.org/docs/current/transaction-iso.html), [OWASP MASVS](https://mas.owasp.org/MASVS/).

These support platform/framework capabilities and selected design requirements; no numerical performance ranking, fixed version compatibility or SDK acceptance is claimed. Exact dependencies remain to be selected and tested.

## Official regulatory discovery sources
[CBN payment-provider categories](https://www.cbn.gov.ng/PaymentsSystem/PSPs.html), [CBN BVN information](https://www.cbn.gov.ng/PaymentsSystem/BVN.html), [Nigeria Data Protection Commission](https://ndpc.gov.ng/).

These identify responsible institutions and applicable review areas, not a complete opinion on this product's licence, permitted custody or current numeric limits. The BVN information page contains historical material: do not use it alone to set live tier rules. Retrieve current binding circulars, NDPC guidance and partner policies during formal compliance review. Exact fees, taxes, limits, reporting requirements and approvals remain unresolved. No legal PDF was relied on as a fully analysed source in this pack.

## Approval-round tooling checks (22 September 2026)
GitHub secure-use guidance, the official chi repository and Expo development-build introduction were re-opened when adopting the stack and workflow. The Actions checkout v5 ref was read from the official actions/checkout repository and pinned to `fbc6f3992d24b796d5a048ff273f7fcc4a7b6c09`. These checks are not a new comprehensive legal/provider audit.

- https://docs.github.com/en/actions/reference/security/secure-use
- https://docs.expo.dev/develop/development-builds/introduction/
- https://github.com/go-chi/chi
- https://api.github.com/repos/actions/checkout/git/ref/tags/v5
