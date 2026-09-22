# UX, screen inventory and visual evidence
Status: proposed screen requirements. No runnable UI or application screenshots exist in this delivery.

## Product navigation
Mobile: Home, Pay, Activity and Profile with contextual support. Web: Overview, Transfers, Bills, Activity, Statements, Beneficiaries and Settings. Business/merchant navigation is conditional on approved capabilities, not empty marketing tabs. Admin: Overview, Customers/KYC, Payments, Bills, Reconciliation, Treasury, Risk, Support, Providers, Policy/Approvals, Staff and Audit.

Use independent authentication layouts with no authenticated dashboard sidebar/header. Separate responsive layouts from theme variation: authentication theme counts and portal theme counts remain decisions. Use a shared brand system, accessible typography, keyboard focus, contrast, consistent forms and dark-mode policy if approved. Do not assume an app-store theme or template is already licensed.

## Required mobile/web screens and states
Registration and verification; login; MFA; recovery; devices/sessions; KYC steps and review; wallet overview; funding instructions; transfer bank/recipient selection; name enquiry; fee quote; transaction authorisation; accepted/pending/final transfer; beneficiaries; bill categories/products; customer validation; purchase confirmation; pending vend; electricity token/fulfilment; activity search/detail; receipts; statements/exports; notifications; support/disputes; security/privacy/settings; maintenance and unavailable-service states.

Each money journey needs loading, empty, invalid input, unavailable provider, expired quote, insufficient available funds, restricted account, pending, success, failure and recovery states where applicable. A refresh, app kill or weak connection must recover an existing payment without silently submitting another. Never show success before authoritative completion; display stale balances clearly. Offline viewing is separately controlled and must not enable offline payment execution.

## Admin visual requirements
Real operational metrics with defined data queries, KYC evidence review, transaction timeline, provider observations, unresolved queue, independent approvals, catalogue controls, recon breaks, treasury views, support cases, role enforcement and audit history. Do not render fabricated KPI values as production metrics.

## Image standard
Capture screenshots from the actual running application using synthetic fixtures. Record application, route/screen, source commit, capture time in UTC, environment, fixture identifier, viewport/device/OS, state, image path, SHA-256 and capture diagnostics. UI changes require refreshed relevant captures or a reviewed no-visual-impact explanation. Mark design mockups explicitly and keep them separate from runtime evidence.

APIbackend has no invented GUI. Show validated architecture/sequence diagrams and redacted real API/contract examples. Mermaid source must actually parse/render in CI; detecting a code fence is insufficient. Include representative normal and error-state captures, not just an attractive login page. Use mobile and desktop/browser coverage independently.

No real credentials, customer details, bank balances, OTPs, identity documents or recovery codes in images. No production authentication bypass or payment submission for documentation capture. The example screenshot manifest is a template, not evidence.

## Baseline renderer
The supplied diagrams are deterministic SVGs generated from `docs/diagrams/diagrams.json`; CI verifies source/output equality and structural validity. Mermaid blocks are rejected until an actual parsing/rendering toolchain is added with tests. This avoids claiming that detecting a code fence validates a diagram.
