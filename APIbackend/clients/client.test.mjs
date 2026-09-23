import test from 'node:test';
import assert from 'node:assert/strict';
import { APIError, QpayClient, minor } from './client.mjs';
const ok = data => ({ ok: true, status: 200, json: async () => ({ data }) });
test('money never accepts floats or exponent notation', () => { for (const v of [1.2, '1e3', '-1', '01', '9000000000000001']) assert.throws(() => minor(v)); assert.equal(minor('12345'), '12345'); });
test('mobile uses only injected bearer credentials', async () => { const client = new QpayClient({ baseURL: 'https://api.example.invalid', transport: 'mobile', getAccessToken: () => 'synthetic', fetchImpl: async (_, init) => { assert.equal(init.headers.Authorization, 'Bearer synthetic'); assert.equal(init.credentials, 'omit'); return ok({ value: true }); } }); assert.deepEqual(await client.request('GET', '/v1/me'), { value: true }); });
test('browser uses cookies and CSRF without bearer', async () => { const client = new QpayClient({ baseURL: 'https://api.example.invalid', transport: 'web', getCSRFToken: () => 'csrf', fetchImpl: async (_, init) => { assert.equal(init.headers['X-CSRF-Token'], 'csrf'); assert.equal(init.headers.Authorization, undefined); assert.equal(init.credentials, 'include'); return ok({}); } }); await client.request('PATCH', '/v1/me', { body: { name: 'Test' } }); });
test('unknown financial transport result is not retried', async () => { let calls = 0; const c = new QpayClient({ baseURL: 'https://api.example.invalid', transport: 'mobile', fetchImpl: async () => { calls++; throw new Error('connection lost'); } }); await assert.rejects(c.pay('quote', 'token', 'persistent-idem-key')); assert.equal(calls, 1); });
test('rejects cross-origin credential destinations', async () => { assert.throws(() => new QpayClient({ baseURL: 'https://user:password@api.example.invalid', transport: 'web' })); const c = new QpayClient({ baseURL: 'https://api.example.invalid', transport: 'mobile' }); await assert.rejects(c.request('GET', '//evil.invalid')); });
test('surfaces structured errors', async () => { const c = new QpayClient({ baseURL: 'https://api.example.invalid', transport: 'mobile', fetchImpl: async () => ({ ok: false, status: 409, json: async () => ({ error: { code: 'conflict', message: 'Changed quote' }, request_id: 'req-test' }) }) }); await assert.rejects(c.request('GET', '/v1/me'), e => e instanceof APIError && e.status === 409 && e.requestId === 'req-test'); });

test('default browser transport preserves native fetch receiver', async () => {
  const original=globalThis.fetch;let calls=0;
  globalThis.fetch=function(){assert.equal(this,globalThis);calls++;return Promise.resolve(ok({verified:true}));};
  try{const client=new QpayClient({baseURL:'https://api.example.invalid',transport:'web'});assert.deepEqual(await client.request('GET','/v1/me'),{verified:true});assert.equal(calls,1);}finally{globalThis.fetch=original;}
});
