# backup-agent

Linux-only isolated execution process. Build with `go build ./cmd/backup-agent`. No HTTP server, financial worker, transaction producer or automatic database startup is included. See [Backups README](../../../Backups/README.md) and [CLI/setup](../../../devdocs/Backups/06-SETUP-AND-PROVIDERS.md).

All commands require deployment-owned profile/keys, except standalone key generation/watchdog operations. Offline recovery uses two external custodians and an independently protected original evidence bundle. Never use production credentials in examples/tests or expose the journal in web assets.
