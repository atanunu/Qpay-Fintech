# ADR-0001: Four applications and authoritative Go financial engine
Status: Accepted. Decision date: 2026-09-22. Approval: repository owner in the project conversation, "Proceed with the recommendations and push the complete documentation and structure to github."

## Decision
B1: APIbackend/ uses Go net/http + chi and PostgreSQL. M1: MobileApp/ uses React Native, TypeScript and Expo development builds. A1: AdminDashboard/ uses a bespoke React, TypeScript, Vite and React Router application. W1: WebApp/ uses React, TypeScript, Vite and React Router. E1: APIbackend communicates server-to-server with the existing QPay API. Alternative frameworks in the decision register are retained as history only.

APIbackend is the authoritative writer of this product's customer wallet subledger. QPay and the contracted financial partner have separately defined execution/accounting/custody responsibilities. No shared mutable wallet tables or direct admin database writes. A presentation server introduced later cannot become another money engine.

## Consequences
Use one modular backend codebase with independently operated API, worker and scheduler processes. Share versioned API types and safe validation helpers across clients; native UI is not assumed to be portable browser UI. Keep customer/staff authentication audiences and sessions separate. Pin supported dependencies and validate their licences and supply chain during implementation. Native identity/payment SDK proof and Android/iOS signed builds remain required.

## Scope of approval
Approves architecture, documentation, repository structure and the core bills/transfers product direction. Nigeria, NGN and adult individual customers are the initial planning baseline, not a legal conclusion. No real payment, provider activation, production deployment, licence approval, signed sponsor agreement, store acceptance or completed app is implied. Provider capabilities must be inspected at a pinned QPay revision before use.

## Alternatives
Go/Gin, Flutter/native mobile, Laravel/Svelte web and Next.js were considered. Reopening an accepted choice requires a new ADR with migration costs, security implications and user approval; do not silently switch frameworks.
