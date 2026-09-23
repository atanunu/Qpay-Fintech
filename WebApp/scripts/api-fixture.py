#!/usr/bin/env python3
"""Seed an isolated local Go/PostgreSQL fixture for browser tests. Never for a live API."""
import json, os, pathlib, secrets, subprocess
root=pathlib.Path(__file__).resolve().parents[2]
if os.environ.get('QPF_ENV') != 'local' or os.environ.get('EXECUTION_MODE') != 'local' or os.environ.get('NOTIFICATION_MODE') != 'local':
    raise SystemExit('Only an explicit local synthetic backend is allowed')
folder=pathlib.Path('/tmp/qpf-web-secrets');folder.mkdir(mode=0o700,exist_ok=True)
password=secrets.token_urlsafe(24);pin='826194'
for name,value in [('password',password),('pin',pin)]:
    path=folder/name;fd=os.open(path,os.O_CREAT|os.O_TRUNC|os.O_WRONLY,0o600)
    with os.fdopen(fd,'w') as f:f.write(value+'\n')
def seed(email):
    env={**os.environ,'QPF_CTL_EMAIL':email,'QPF_CTL_NAME':'Synthetic Web Reviewer','QPF_CTL_PASSWORD_FILE':str(folder/'password'),'QPF_CTL_PIN_FILE':str(folder/'pin')}
    completed=subprocess.run(['/tmp/qpf-bin/qpfctl','seed-local'],cwd=root/'APIbackend',env=env,text=True,capture_output=True,check=True)
    return json.loads(completed.stdout)['id']
tag=secrets.token_hex(8);email=f'web-{tag}@example.invalid';seed(email);recipient=seed(f'recipient-{tag}@example.invalid')
fd=os.open('/tmp/qpf-web-fixture.json',os.O_WRONLY|os.O_CREAT|os.O_TRUNC,0o600)
with os.fdopen(fd,'w') as f:json.dump({'email':email,'password':password,'pin':pin,'recipient_id':recipient},f)
for p in folder.iterdir():p.unlink()
print('Created synthetic browser fixtures; credentials kept in an owner-only temporary file.')
