# Infrastructure workspace
Status: structure and operations design only; no deployable production configuration. Use the [operations plan](../devdocs/project/11-OPERATIONS-AND-DEPLOYMENT.md) and [configuration contract](../devdocs/project/19-ENVIRONMENT-AND-SECRETS.md).

Reserve docker/ for reviewed local/test container definitions; observability/ for redacted telemetry and alerts; runbooks/ for deployment, incidents, reconciliation, recovery and provider degradation. API, worker and scheduler are separate processes from APIbackend, not separate ledgers. Frontends deploy independently. Production topology, domains, storage/key/backup ownership and objectives remain explicit release decisions.
