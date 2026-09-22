# ADR-0003: Self-hosted Novu and built-in notification content
Status: **Accepted by the user, 22 September 2026**.

Use self-hosted Novu. Own its Qpay-Fintech configuration, email themes, workflow source, catalogue, fixtures, tests and operational documentation in **Novu/**. The previously proposed Notifications/ directory is superseded. Go remains the sole product financial authority and publishes authorised business events through a durable outbox.

Build notification content with each product milestone. Git is the source of truth; deploy the compatible signed bridge and synchronise workflows into the selected self-hosted environment. Workflow sync is not a replacement for provider setup, credentials, subscriber/preferences migration or delivery testing.

Community is the planning baseline; confirm the exact release/edition/licence before installation. Do not assume Cloud-only activity tracking, enterprise dashboard administration, extra environments or removable branding. Plan private dashboard access, isolated installations and a provider-to-Go delivery adapter. An Enterprise self-hosted purchase would be a separate decision.

This decision authorises repository planning/content work, not a live deployment, production mailing, provider activation or financial operation. See [Novu specifications](../Novu/00-INDEX.md) and [canonical service register](../../Novu/README.md).
