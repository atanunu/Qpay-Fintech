#!/usr/bin/env python3
"""Generate fresh owner-only local credentials; never overwrite an existing setup."""
import base64
import os
from pathlib import Path
import secrets

root = Path(__file__).resolve().parents[1]
key = lambda: base64.b64encode(secrets.token_bytes(32)).decode()
password = secrets.token_urlsafe(24)
content = f"""# SYNTHETIC LOCAL DEVELOPMENT ONLY
QPF_ENV=local
POSTGRES_PASSWORD={password}
DATABASE_URL=postgres://qpf:{password}@postgres:5432/qpf?sslmode=disable
AUTH_PEPPER={key()}
DATA_KEY_ID=v1
DATA_KEYS=v1:{key()}
NOTIFICATION_POLICY_KEY={key()}
WEB_ORIGINS=http://localhost:5173,http://localhost:5174
EXECUTION_MODE=local
NOTIFICATION_MODE=local
HTTP_ADDR=:8080
"""
fd = os.open(root / '.env.local', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'w') as stream:
    stream.write(content)
print('Created APIbackend/.env.local with fresh local-only keys. No provider is enabled.')
