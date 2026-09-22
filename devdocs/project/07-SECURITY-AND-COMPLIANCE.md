# Security, privacy and operating approval
Status: planning requirements, not legal advice, certification or an assertion of regulatory permission.

## Release-blocking business evidence
Identify the operating company and precisely who holds customer funds, issues accounts, performs KYC, operates payment rails, reports suspicious activity, handles complaints, bears loss and supports failed transactions. Document contracts, applicable licences/permissions, territory and customer eligibility with qualified Nigerian counsel and the selected partner. CBN publishes distinct payment-provider categories; selecting an API or owning source code does not establish the chosen operating model's authorisation. Use the current official references in [Research](14-RESEARCH-SOURCES.md).

Assess current BVN/NIN, account-tier and partner onboarding rules without inventing numeric limits. Compliance must approve AML/sanctions/PEP workflows, monitoring, escalation, retention and reporting responsibilities. Do not promise sanctions screening or regulatory reporting merely because a review dashboard exists.

Privacy work must assess applicable NDPA/NDPC requirements, data-controller/processor roles, lawful bases, notices, consent where appropriate, retention/legal holds, user requests, breach response, cross-border processing and impact assessments. Obtain the current authoritative rules and document applicability before production. This pack is not an exhaustive legal review.

## Technical control baseline
Separate customer/staff identities, staff MFA, least privilege, strong recovery, revocable sessions, device visibility and transaction-bound authorisation. Use OS-backed secret storage on mobile; a biometric unlock is not a substitute for server-side authorisation. Review selected SDK behaviour on biometric enrolment changes, device backup, reinstallation and compromised devices.

Protect server-to-server keys with scope, rotation and audit. Encrypt sensitive documents and backups with recoverable key management. Keep logs and analytics free of credentials, OTPs, full identity documents and raw card data. Restrict signed download links and export lifetimes. Avoid raw card collection where a partner-hosted/tokenised flow is sufficient; obtain a formal card-data scope assessment before adding card features.

Risk controls: velocity and limit enforcement, new-device/new-beneficiary risk, account takeover controls, enumeration protection, suspicious funding, transaction monitoring, reviewed restrictions, reason-coded decisions and independent financial approvals. Do not rely on a hidden menu or client-side permission checks.

## Assurance
Map mobile controls to OWASP MASVS and adopt an API/web threat-model baseline. Test object-level authorisation, cross-user/cross-organisation data access, callback forgery, replay, SSRF, injection, dependency exposure and secret leakage. Commission independent penetration testing and fix release-blocking findings. Establish emergency access with restricted, logged and reviewed use; do not add production auth bypasses for screenshots.

## Data and publication
The target repository was public when inspected. Review that choice before publishing proprietary material. Never put real bank/customer/KYC data, credentials, internal incident payloads or provider-confidential specifications in public docs. Synthetic examples only. Public issue templates must direct security vulnerabilities to a private reporting route once established.
