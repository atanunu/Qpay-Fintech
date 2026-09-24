#!/usr/bin/env python3
"""Provision disposable admin browser fixtures through real CLI, cookies and staff APIs.

No production bypass, direct ledger SQL, provider call, or secret output. A fresh
isolated PostgreSQL database, local execution and local private store are required.
"""
from pathlib import Path
import base64, hashlib, hmac, http.cookiejar, json, os, secrets, struct, subprocess, time
from urllib.request import Request, build_opener, HTTPCookieProcessor
from urllib.error import HTTPError
if os.environ.get('QPF_ENV') != 'local' or os.environ.get('EXECUTION_MODE') != 'local':
    raise SystemExit('Only isolated local synthetic execution is permitted')
ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'.test-fixtures'; OUT.mkdir(mode=0o700,exist_ok=True)
BASE=os.environ.get('QPF_FIXTURE_API_URL','http://localhost:8080'); ORIGIN='http://localhost:5174'
class Client:
    def __init__(self, origin=ORIGIN):
        self.opener=build_opener(HTTPCookieProcessor(http.cookiejar.CookieJar())); self.csrf=''; self.origin=origin; self.token=''
    def request(self, method, path, body=None, raw=None, content_type=None, idem=None):
        headers={'Accept':'application/json','Origin':self.origin}
        if self.token: headers['Authorization']='Bearer '+self.token
        if self.csrf: headers['X-CSRF-Token']=self.csrf
        data=raw
        if body is not None: data=json.dumps(body).encode(); headers['Content-Type']='application/json'
        if content_type: headers['Content-Type']=content_type
        if idem: headers['Idempotency-Key']=idem
        try:
            with self.opener.open(Request(BASE+path,data=data,headers=headers,method=method),timeout=20) as res:
                return json.load(res)['data'] if res.status!=204 else None
        except HTTPError as exc:
            detail=json.load(exc)
            raise RuntimeError(f'{method} {path}: {exc.code} {detail.get("error",{}).get("code")}') from None
    def login(self,email,password,client='web'):
        prefix='/v1/admin/auth' if self.origin==ORIGIN else '/v1/auth'
        data=self.request('POST',prefix+'/login',{'email':email,'password':password,'client':client,'device':'Isolated admin browser fixture'})
        self.csrf=data.get('csrf_token',''); self.token=data.get('access_token',''); return data

def totp(secret):
    key=base64.b32decode(secret+'='*((8-len(secret)%8)%8)); v=hmac.new(key,struct.pack('>Q',int(time.time())//30),hashlib.sha1).digest(); n=v[-1]&15
    return f'{(struct.unpack(">I",v[n:n+4])[0]&0x7fffffff)%1000000:06}'

def secret_file(name,value):
    path=OUT/name; fd=os.open(path,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
    with os.fdopen(fd,'w') as f: f.write(value+'\n')
    return str(path)

def cli(command,email,password,name):
    p=secret_file('secret-'+secrets.token_hex(8),password)
    pin=secret_file('pin-'+secrets.token_hex(8),'483729')
    try:
        env=dict(os.environ,QPF_CTL_EMAIL=email,QPF_CTL_NAME=name,QPF_CTL_PASSWORD_FILE=p,QPF_CTL_PIN_FILE=pin)
        result=subprocess.run([os.environ.get('QPF_CTL_BIN','/tmp/qpfctl'),command],env=env,capture_output=True,text=True)
        if result.returncode: raise RuntimeError('Isolated fixture CLI failed: '+command)
        return json.loads(result.stdout)['id']
    finally: Path(p).unlink(); Path(pin).unlink()

def enrol(c,password):
    info=c.request('POST','/v1/admin/auth/mfa/enrol',{'password':password})
    return c.request('POST','/v1/admin/auth/mfa/confirm',{'code':totp(info['secret'])}), info['secret']

health=Client('http://localhost:5173')
for attempt in range(60):
    try:
        if health.request('GET','/health/ready')['status']=='ready': break
    except Exception:
        if attempt==59: raise
        time.sleep(1)
users={}; clients={}; tag=secrets.token_hex(6)
for key,command in [('admin','bootstrap-admin'),('checker','bootstrap-checker')]:
    email=f'{key}-{tag}@example.invalid'; password=secrets.token_urlsafe(24)
    uid=cli(command,email,password,'Synthetic '+key.title())
    c=Client(); c.login(email,password); codes,secret=enrol(c,password)
    users[key]={'id':uid,'email':email,'password':password,'recoveryCodes':codes,'secret':secret}; clients[key]=c
for role in ['finance','compliance','support','platform','auditor','enrol']:
    email=f'{role}-{tag}@example.invalid'; password=secrets.token_urlsafe(24)
    invite=clients['admin'].request('POST','/v1/admin/console/invitations',{'email':email,'name':'Synthetic '+role.title(),'role':'auditor' if role=='enrol' else role,'reason':'Isolated browser role verification'})['id']
    grant=clients['checker'].request('POST',f'/v1/admin/console/invitations/{invite}/decision',{'decision':'approve','reason':'Independent synthetic identity verification'})
    c=Client(); uid=c.request('POST','/v1/admin/invitations/accept',{'token':grant['invitation_token'],'password':password})['id']
    codes=[]; secret=''
    if role!='enrol': c.login(email,password); codes,secret=enrol(c,password)
    users[role]={'id':uid,'email':email,'password':password,'recoveryCodes':codes,'secret':secret}
email=f'customer-{tag}@example.invalid'; password=secrets.token_urlsafe(24); uid=cli('seed-local',email,password,'Synthetic Customer')
rid=cli('seed-local',f'recipient-{tag}@example.invalid',secrets.token_urlsafe(24),'Synthetic Recipient')
customer=Client('http://localhost:5173'); customer.login(email,password,'mobile')
case=customer.request('POST','/v1/support/cases',{'kind':'support','subject':'Synthetic missing bill enquiry','message':'Please investigate this synthetic payment record.'})['id']
quote=customer.request('POST','/v1/quotes',{'kind':'internal','recipient_id':rid,'amount_minor':'10000','currency':'NGN','narration':'Synthetic admin browser transfer'})
auth=customer.request('POST',f'/v1/quotes/{quote["id"]}/authorisations',{'pin':'483729'})
pay=customer.request('POST','/v1/payments',{'quote_id':quote['id'],'authorisation_token':auth['authorisation_token']},idem='admin-fixture-'+secrets.token_hex(16))
# Small valid PNG with synthetic pixels only; no government document or PII.
png=base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLttAAAAABJRU5ErkJggg==')
boundary='fixture_'+secrets.token_hex(12)
parts=[]
for name,value in [('purpose','kyc'),('case_id','')]:
    parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="{name}"\r\n\r\n{value}\r\n'.encode())
parts.append(f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="synthetic-evidence.png"\r\nContent-Type: image/png\r\n\r\n'.encode()+png+f'\r\n--{boundary}--\r\n'.encode())
upload=customer.request('POST','/v1/uploads',raw=b''.join(parts),content_type='multipart/form-data; boundary='+boundary)
draft=customer.request('PATCH','/v1/identity/draft',{'legal_name':'Synthetic Legal Customer','date_of_birth':'1990-01-02','address':'Synthetic private test address','document_type':'national-id','upload_ids':[upload['id']],'consent':True,'version':0})
kyc=customer.request('POST','/v1/identity/submit',{'version':draft['version']})['id']
for c in clients.values(): c.request('POST','/v1/admin/auth/logout')
fd=os.open(OUT/'admin.json',os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
with os.fdopen(fd,'w') as f: json.dump({'users':users,'customerId':uid,'recipientId':rid,'paymentId':pay['id'],'supportId':case,'kycId':kyc,'documentId':upload['id']},f)
print('Created isolated staff via bootstrap + independent invitation + mandatory MFA; synthetic customer evidence and payment only.')
