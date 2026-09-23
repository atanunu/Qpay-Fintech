# Private object adapters

`Local` stores application-encrypted bytes under an `os.Root`, validates generated object keys, uses exclusive owner-only files and refuses oversized reads. Local scanner state is explicitly `local_unscanned`, never claimed antivirus-clean. This mode is forbidden outside `QPF_ENV=local`.

`S3` uses the pinned Minio client, TLS, explicit region/credentials, a dedicated private bucket and bounded reads. Startup rejects nonempty bucket policies; operational ACL/public-block verification remains required. It never exposes presigned public document links. Go decrypts only after current object/role checks and audits downloads.

`Clamd` uses a private Unix socket with bounded chunked INSTREAM framing, deadlines and NUL-terminated exact clean responses. Scanner failure/incomplete response fails closed. Tests use a synthetic socket protocol peer, not proof of live antivirus deployment.

Configure `PRIVATE_UPLOAD_MODE`, `PRIVATE_UPLOAD_ROOT`, `PRIVATE_S3_ENDPOINT`, `PRIVATE_S3_BUCKET`, `PRIVATE_S3_REGION`, `PRIVATE_S3_ACCESS_KEY`, `PRIVATE_S3_SECRET_KEY`, and `CLAMD_SOCKET` as documented in [.env.example](../../.env.example). Encrypt and restore the store and data-key ring together. [Security specification](../../../devdocs/WebApp/08-SECURITY-PRIVACY-AND-ACCESSIBILITY.md).
