# Decision register and technology options
Status: **B1 + M1 + A1 + W1 + E1 approved by the user on 22 September 2026.** Alternatives below are retained as decision history, not open implementation choices. See [accepted ADR](../adrs/0001-APPROVED-ARCHITECTURE.md). Package versions and native-provider SDK compatibility must be verified during implementation; approval is not a claim that those proofs have passed.

## Application options
| Code | Application | Option | Assessment |
|---|---|---|---|
| B1 | APIbackend | Go net/http + chi, PostgreSQL, SQL-first repositories | Recommended: explicit domain boundaries and small HTTP layer; use pgx/sqlc and one migration owner after dependency review |
| B2 | APIbackend | Go + Gin, same persistence design | Good alternative when the team prefers Gin conventions; ledger design and safety requirements are unchanged |
| B3 | APIbackend | Go standard library only | Few framework dependencies but more internal HTTP conventions to maintain |
| B4 | APIbackend | Laravel or NestJS | Viable staffing-led alternatives; not preferred over the user's Go direction |
| M1 | MobileApp | React Native + TypeScript + Expo development builds | Recommended with React portals; shared API types and validation knowledge, not assumed full UI reuse |
| M2 | MobileApp | Flutter + Dart | Strong alternative for a mobile-first team and a unified custom mobile UI; a separate language/tooling ecosystem |
| M3 | MobileApp | Native Swift/SwiftUI and Kotlin/Compose | Maximum direct platform control; two client implementations and release/test tracks |
| A1 | AdminDashboard | React + TypeScript + Vite + React Router | Recommended bespoke operations interface; API-only access to financial actions |
| A2 | AdminDashboard | Laravel + Blade or Inertia | Good for a PHP-focused team; no Filament required; local staff-session/view data only, never a second wallet writer |
| A3 | AdminDashboard | SvelteKit | Reasonable Svelte-oriented alternative; separate framework conventions from RN/React |
| W1 | WebApp | React + TypeScript + Vite + React Router | Recommended for an authenticated dashboard; no SEO-driven SSR requirement assumed |
| W2 | WebApp | Next.js + TypeScript | Prefer when server-rendered/public content or a justified server layer is required; financial execution stays in Go |
| W3 | WebApp | SvelteKit | Good when Svelte is the team's selected frontend ecosystem |
| W4 | WebApp | Laravel + Blade or Inertia | Operationally consistent with a Laravel admin; two backend languages to maintain alongside Go |

Framework capabilities are grounded in the official sources in [Research](14-RESEARCH-SOURCES.md). Rankings are design recommendations, not performance benchmarks. Pin supported stable versions after selection, verify native-provider SDK compatibility and record licences, update policy and lockfiles. Do not adopt an unverified version number because another project used it.

## Coherent bundles
| Bundle | Choices | Suitable emphasis |
|---|---|---|
| S1 recommended | B1 + M1 + A1 + W1 | Go financial engine and TypeScript client ecosystem |
| S2 | B1 + M2 + A1 + W1 | Flutter mobile while keeping React browser apps |
| S3 | B1 + M1 or M2 + A2 + W4 | Laravel-oriented web/admin team, Go-only financial authority |
| S4 | B1 + M1 + A1 + W2 | React Native plus a Next.js user experience |

Before release acceptance of the selected M1 stack, build a disposable Android/iOS integration proof covering the selected KYC/liveness SDK, payment SDK where applicable, secure storage, biometrics, push/deep links, signing and release builds. An Expo Go demonstration is not that proof. Budget for real-device testing and both stores. Expo cloud build services are optional, not assumed purchased.

## Execution integration
E1 approved: consume the existing QPay API as an upstream execution service. E2: own direct provider adapters in the new Go engine. E3: staged hybrid with explicit operation ownership and safe pre-submission routing. See [Integration](04-QPAY-INTEGRATION.md).

## Business and infrastructure decisions still required
| ID | Decision | Proposed default or required evidence |
|---|---|---|
| DEC-01 | Legal entity, launch market, customer eligibility | Nigeria/NGN/adult individuals adopted as product-planning baseline; legal entity still to identify |
| DEC-02 | Custody and operating permissions | Approved sponsor/partner arrangement or independently authorised model; legal review required |
| DEC-03 | Relationship to QPay | E1 approved; implementation still requires current API and financial acceptance audit |
| DEC-04 | Four application stacks | S1 approved: B1 + M1 + A1 + W1 |
| DEC-05 | Authoritative wallet/accounting owner | APIbackend only for this product, mapped to partner obligations |
| DEC-06 | Partner selection and credentials | Procure using capability/acceptance matrix, not logo or adapter presence |
| DEC-07 | Hosting, data residency, recovery budget | Docker/private service networking proposed; production topology and off-host recovery still to approve |
| DEC-08 | Domains, branding and repository visibility | Not selected; review public-repository exposure before proprietary content |
| DEC-09 | Authentication and recovery policy | Separate staff/customer identities; staff MFA, privileged dual control, customer risk-based step-up |
| DEC-10 | Theme requirements and supported devices | Bespoke separate auth shells; theme counts and target OS/browser versions not fixed |
| DEC-11 | Phase-one commercial scope | Core bills/transfers and supporting financial operations; growth requires explicit approval |
| DEC-12 | SLOs and launch thresholds | Set measured throughput/latency, pending-age alerts, RPO/RTO and support commitments before release |

Record each approval in an ADR. Unresolved choices permit planning and isolated proofs, not silent production assumptions.

## Accepted notification decision
Self-hosted Novu under `Novu/` is accepted. The proposed `Notifications/` name is superseded. Exact server release, image/SDK pins, production hostnames, legal sender and provider account are qualification inputs, not silently selected defaults.

See the [Novu specification index](../Novu/00-INDEX.md) and [service task register](../../Novu/README.md).
