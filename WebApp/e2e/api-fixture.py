#!/usr/bin/env python3
"""Create disposable local-only browser fixtures; never print credentials."""
from pathlib import Path
import json
import os
import secrets
import subprocess
import time
from urllib.request import urlopen

if os.environ.get('QPF_ENV') != 'local' or os.environ.get('EXECUTION_MODE') != 'local':
    raise SystemExit('API browser fixtures require explicit local synthetic execution')
root = Path(__file__).resolve().parents[1]
fixture_dir = root / '.test-fixtures'
fixture_dir.mkdir(mode=0o700, exist_ok=True)
password, pin = secrets.token_urlsafe(24), '483729'
tag = secrets.token_hex(6)
emails = [f'web-review-{tag}-{i}@example.invalid' for i in range(2)]
files = {'password': password, 'pin': pin}
for name, value in files.items():
    fd = os.open(fixture_dir / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as stream:
        stream.write(value + '\n')
try:
    for attempt in range(60):
        try:
            with urlopen('http://127.0.0.1:8080/health/ready', timeout=3) as response:
                if json.load(response)['data']['status'] == 'ready':
                    break
        except Exception:
            if attempt == 59:
                raise
            time.sleep(1)
    ids = []
    for i, email in enumerate(emails):
        env = dict(os.environ, QPF_CTL_EMAIL=email, QPF_CTL_NAME=f'API Review Owner {i+1}',
                   QPF_CTL_PASSWORD_FILE=str(fixture_dir / 'password'),
                   QPF_CTL_PIN_FILE=str(fixture_dir / 'pin'))
        result = subprocess.run(['/tmp/qpfctl', 'seed-local'], env=env, capture_output=True, text=True)
        if result.returncode:
            raise RuntimeError('Local fixture creation failed; inspect the isolated backend configuration')
        ids.append(json.loads(result.stdout)['id'])
    fd = os.open(fixture_dir / 'api.json', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as stream:
        json.dump({'email': emails[0], 'password': password, 'pin': pin,
                   'senderId': ids[0], 'recipientId': ids[1]}, stream)
    if os.environ.get('GITHUB_ENV'):
        with open(os.environ['GITHUB_ENV'], 'a') as stream:
            stream.write('VITE_API_MODE=api\nVITE_API_URL=http://localhost:8080\n'
                         'VITE_ALLOW_LOCAL_HTTP=true\nVITE_ENABLE_REVIEW_TOOLS=true\n')
    print('Created two isolated synthetic owners; no real funding, provider or email used.')
finally:
    for name in files:
        (fixture_dir / name).unlink(missing_ok=True)
