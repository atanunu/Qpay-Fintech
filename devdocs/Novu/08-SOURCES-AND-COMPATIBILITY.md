# Official research and compatibility record
Reviewed 22 September 2026. These are current documentation observations, not a test of the user's installed version. The exact installation version was not supplied. Do not infer a compatible deployment from the old v2.2.0 announcement, an SDK version in an example, a moving next branch or a latest image tag.

| ID | Official source | Relevant finding and design consequence |
|---|---|---|
| S1 | https://docs.novu.co/community/self-hosting-novu/deploy-with-docker | Documents self-hosted components, setup, Go/custom API URLs, bridge sync, registration controls and outbound host allow rules. Qualify the selected release and use private production topology. |
| S2 | https://docs.novu.co/community/self-hosted-and-novu-cloud | Community supports Framework workflows/layouts, but differs on environments, staff administration/security, tracking and branding. Do not assume Cloud features or certifications. |
| S3 | https://docs.novu.co/framework/deployment/syncing and https://docs.novu.co/framework/deployment/cli | Git-owned bridge deployment plus workflow sync; Dashboard identifiers do not automatically convert to Framework. Migrate deliberately. |
| S4 | https://docs.novu.co/framework/endpoint and https://docs.novu.co/framework/deployment/production | Bridge is runtime code, not disposable import data. Preserve and test signature validation rather than development bypass. |
| S5 | https://docs.novu.co/framework/typescript/workflow and https://docs.novu.co/framework/schema/json-schema | Workflow identity, payload schemas and preferences support code-owned contracts. |
| S6 | https://docs.novu.co/framework/typescript/steps/email and https://docs.novu.co/framework/typescript/steps | Email-step output is constrained; arbitrary text/headers are not base output fields. Use a tested provider mapping and real skip/discovery acceptance. |
| S7 | https://docs.novu.co/platform/integrations/email/activity-tracking | Email activity tracking is Cloud/Enterprise self-hosted, not Community. Plan independent provider-to-Go evidence; replies are a different capability. |
| S8 | https://docs.novu.co/community/self-hosting-novu/telemetry | Telemetry setting does not by itself establish no egress; a keep-alive beacon is separately documented. |
| S9 | https://support.google.com/a/answer/81126 | Sender authentication, transport, reputation and bulk/subscription unsubscribe requirements. Project sender acceptance checks actual delivered messages. |
| S10 | https://docs.novu.co/platform/integrations | Integrations are environment-specific; no automatic cross-provider send fallback. |

Architecture controls, rollout order, templates, TTL ceilings and service boundaries are project design decisions. They are not representations of a provider contract, legal deadline or a guaranteed SLA. Delivery-provider choice, domains, legal sender details, hosting resources and actual software pins remain qualification inputs. Self-hosted Enterprise may be evaluated without changing the Novu/ source-ownership model, but no licence purchase is implied.
