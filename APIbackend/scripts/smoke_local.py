#!/usr/bin/env python3
"""Exercise an already-running local Compose stack with synthetic funds only."""
import json
import os
from pathlib import Path
import secrets
import subprocess
import time
from urllib.request import Request, urlopen

root = Path(__file__).resolve().parents[1]
base = 'http://127.0.0.1:8080'
compose = ['docker', 'compose', '--env-file', '.env.local', '-f', 'compose.local.yml']
secret_dir = root / 'local-secrets'
secret_dir.mkdir(mode=0o700, exist_ok=True)
password, pin = secrets.token_urlsafe(24), '483729'
for name, value in [('smoke-password', password), ('smoke-pin', pin)]:
    fd = os.open(secret_dir/name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as stream:
        stream.write(value+'\n')

def api(method, path, body=None, token='', idem=''):
    headers = {'Accept': 'application/json'}
    if body is not None:
        headers['Content-Type'] = 'application/json'
    if token:
        headers['Authorization'] = 'Bearer '+token
    if idem:
        headers['Idempotency-Key'] = idem
    request = Request(base+path, data=None if body is None else json.dumps(body).encode(), headers=headers, method=method)
    with urlopen(request, timeout=15) as response:
        return json.load(response)['data']

def seed(email):
    args=compose+['run','--rm','--no-deps','--user',f'{os.getuid()}:{os.getgid()}',
        '-e',f'QPF_CTL_EMAIL={email}','-e','QPF_CTL_NAME=Synthetic smoke customer',
        '-e','QPF_CTL_PASSWORD_FILE=/run/qpf-secrets/smoke-password',
        '-e','QPF_CTL_PIN_FILE=/run/qpf-secrets/smoke-pin','control','seed-local']
    result=subprocess.run(args,cwd=root,text=True,capture_output=True,check=True)
    return json.loads(result.stdout)['id']

try:
    for attempt in range(60):
        try:
            if api('GET','/health/ready')['status']=='ready': break
        except Exception:
            if attempt==59: raise
            time.sleep(2)
    capabilities=api('GET','/v1/capabilities')
    assert capabilities['environment']=='local' and capabilities['synthetic_execution'] is True
    tag=secrets.token_hex(6)
    emails=[f'smoke-{tag}-{i}@example.invalid' for i in range(2)]
    ids=[seed(email) for email in emails]
    sessions=[api('POST','/v1/auth/login',{'email':email,'password':password,'client':'mobile','device':'synthetic-smoke'}) for email in emails]
    token=sessions[0]['access_token']
    before=api('GET','/v1/wallet',token=token)
    assert before['balance_minor']=='10000000'
    def pay(details):
        quote=api('POST','/v1/quotes',details,token)
        approval=api('POST',f"/v1/quotes/{quote['id']}/authorisations",{'pin':pin},token)
        payload={'quote_id':quote['id'],'authorisation_token':approval['authorisation_token']}
        idem='smoke_'+secrets.token_hex(16)
        payment=api('POST','/v1/payments',payload,token,idem)
        duplicate=api('POST','/v1/payments',payload,token,idem)
        assert duplicate['id']==payment['id']
        for _ in range(60):
            current=api('GET',f"/v1/payments/{payment['id']}",token=token)
            if current['status']=='succeeded' and (details['kind']!='bill' or current['fulfilment_status']=='ready'):
                return current
            assert current['status'] not in ('failed','pending_review'), current['status']
            time.sleep(1)
        raise AssertionError('synthetic worker did not complete original payment')
    internal=pay({'kind':'internal','recipient_id':ids[1],'amount_minor':'10000','currency':'NGN','narration':'Synthetic internal test'})
    assert api('GET',f"/v1/payments/{internal['id']}/receipt",token=token)['copy'] is True
    enquiry=api('POST','/v1/banks/enquiries',{'bank_code':'999','account_number':'1234567890'},token)
    beneficiary=api('POST','/v1/beneficiaries',{'enquiry_id':enquiry['id'],'label':'Synthetic recipient'},token)
    bank=pay({'kind':'bank','beneficiary_id':beneficiary['id'],'amount_minor':'10000','currency':'NGN','narration':'Synthetic bank test'})
    validation=api('POST','/v1/bills/validations',{'product_id':'synthetic-electricity','customer_id':'synthetic-customer','amount_minor':'5000'},token)
    bill=pay({'kind':'bill','validation_id':validation['id'],'amount_minor':'5000','currency':'NGN','narration':'Synthetic electricity test'})
    value=api('GET',f"/v1/payments/{bill['id']}/fulfilment",token=token)
    assert 'SYNTHETIC' in value['value']
    after=api('GET','/v1/wallet',token=token)
    assert after['balance_minor']=='9974800' and after['held_minor']=='0', after
    print(json.dumps({'result':'passed','environment':'local','journeys':['internal-transfer','bank-transfer-simulator','bill-vend-simulator','duplicate-recovery','receipt','fulfilment','wallet-balance'],'real_provider_calls':0}))
finally:
    for name in ['smoke-password','smoke-pin']:
        (secret_dir/name).unlink(missing_ok=True)
