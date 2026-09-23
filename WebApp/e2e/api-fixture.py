#!/usr/bin/env python3
"""Create disposable local-only browser fixtures; never print credentials."""
from pathlib import Path
import json
import re
import os
import secrets
import subprocess
import time
from urllib.request import urlopen, Request

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
    fixtures = {}
    titles = []
    for spec in [root/'e2e/api.spec.ts', root/'e2e/parity.api.spec.ts']:
        titles += re.findall(r"test\('([^']+)'", spec.read_text())
    for index, title in enumerate(titles):
        ids, pair = [], [f'web-{tag}-{index}-{i}@example.invalid' for i in range(2)]
        for i,email in enumerate(pair):
            env = dict(os.environ, QPF_CTL_EMAIL=email, QPF_CTL_NAME=f'API Review Owner {i+1}',
                       QPF_CTL_PASSWORD_FILE=str(fixture_dir/'password'), QPF_CTL_PIN_FILE=str(fixture_dir/'pin'))
            result = subprocess.run(['/tmp/qpfctl','seed-local'],env=env,capture_output=True,text=True)
            if result.returncode: raise RuntimeError('Local fixture creation failed; inspect isolated configuration')
            ids.append(json.loads(result.stdout)['id'])
        handle=f'payee_{tag}_{index}'
        def call(path,body,token=''):
            headers={'Content-Type':'application/json'}
            if token: headers['Authorization']='Bearer '+token
            request=Request('http://127.0.0.1:8080'+path,data=json.dumps(body).encode(),headers=headers,method='POST' if path!='/v1/me/controls' else 'PATCH')
            with urlopen(request,timeout=10) as response: return json.load(response)['data']
        session=call('/v1/auth/login',{'email':pair[1],'password':password,'client':'mobile','device':'isolated-fixture'})
        token=session['access_token']
        call('/v1/me/controls',{'handle':handle,'discoverable':True,'per_payment_minor':'10000000','daily_minor':'50000000','password':password},token)
        # A stable idempotency key belongs to this fixture request, not a real financial operation.
        body={'memo':'Synthetic shared household bill','amount_minor':'5000','expires_at':__import__('datetime').datetime.fromtimestamp(time.time()+86400,__import__('datetime').timezone.utc).isoformat(),'shares':[{'payer_id':ids[0],'amount_minor':'5000'}]}
        req=Request('http://127.0.0.1:8080/v1/money-requests',data=json.dumps(body).encode(),headers={'Content-Type':'application/json','Authorization':'Bearer '+token,'Idempotency-Key':'fixture_'+secrets.token_hex(16)},method='POST')
        with urlopen(req,timeout=10) as response: request_id=json.load(response)['data']['id']
        fixtures[title]={'recipient_handle':handle,'request_id':request_id,'email':pair[0],'recipient_email':pair[1],'password':password,'pin':pin,'sender_id':ids[0],'recipient_id':ids[1]}
    fd=os.open(fixture_dir/'api.json',os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(fd,'w') as stream: json.dump({'accounts':fixtures},stream)
    if os.environ.get('GITHUB_ENV'):
        with open(os.environ['GITHUB_ENV'], 'a') as stream:
            stream.write('QPF_BROWSER_FIXTURE='+str(fixture_dir / 'api.json')+'\nVITE_API_MODE=api\nVITE_API_BASE_URL=http://localhost:8080\n'
                         'VITE_ALLOW_LOCAL_HTTP=true\nVITE_ENABLE_REVIEW_TOOLS=true\n')
    print('Created independent isolated synthetic owners for every browser case; no real funding, provider or email used.')
finally:
    for name in files:
        (fixture_dir / name).unlink(missing_ok=True)
