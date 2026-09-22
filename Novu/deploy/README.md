# Self-hosted deployment workspace
Read the [complete setup/import runbook](../../devdocs/Novu/05-DEPLOYMENT-AND-IMPORT.md) and [compatibility research](../../devdocs/Novu/08-SOURCES-AND-COMPATIBILITY.md).

compatibility.json is intentionally unqualified and activates no workflows. .env.example in the service root lists required configuration names, not working credentials or a verified Compose stack. Select the actual release, qualify its dependencies, then add immutable production/non-production Compose manifests and a signed bridge Dockerfile here. No latest tags, copied quick-start secrets, public MongoDB/Redis ports, embedded provider keys or Cloud default API endpoints are permitted.

A production deployment is blocked until every enabled workflow has a real producer, guard, authorised recipient, provider mapping, delivery evidence and release approval. Do not turn a documentation baseline into a deployment by filling placeholder variables alone. Separate test and production secrets, databases, subscriber populations and sender streams. Capture the selected upstream licences and preserve required branding.
