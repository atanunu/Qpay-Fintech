import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { catalogue, parseCatalogue, themes, fixture, renderEmail, validatePayload, payloadSchema, validateBrand, previewBrand, money } from '../src/content.mjs';
import { defineWorkflows } from '../bridge/workflows.mjs';

for (const event of catalogue) test(`renders HTML/plain text/schema: ${event.workflowId}`, () => {
  const p = fixture(event), r = renderEmail(event, p);
  assert.ok(r.html.startsWith('<!doctype html>'));
  assert.ok(r.text.includes(event.subject));
  assert.ok(!/\{\{(?:name|reference|amount|code)\}\}/.test(r.html));
  assert.ok(Buffer.byteLength(r.html) < 50000);
  assert.deepEqual(payloadSchema(event).required.sort(), Object.keys(p).sort());
  assert.equal(r.workflowId, event.workflowId);
  assert.ok(Object.hasOwn(themes, r.theme));
});
const find = key => catalogue.find(e => e.key === key);
const welcome = find('identity-welcome'), transfer = find('transfer-completed');
const code = find('identity-verify-email'), optional = find('identity-onboarding-reminder');
const changed = (e, patch) => ({ ...fixture(e), ...patch });

test('catalogue has stable unique IDs and guards', () => {
  assert.equal(catalogue.length, 175);
  assert.equal(new Set(catalogue.map(e => e.workflowId)).size, 175);
  assert.equal(catalogue.filter(e => e.scope === 'core').length, 145);
});
test('rejects duplicate catalogue keys', () => {
  const lines = readFileSync(new URL('../catalogue/emails.psv', import.meta.url), 'utf8').trim().split('\n');
  assert.throws(() => parseCatalogue([lines[0], lines[1], lines[1]].join('\n')), /duplicate/);
});
test('rejects unknown placeholders', () => {
  const raw = readFileSync(new URL('../catalogue/emails.psv', import.meta.url), 'utf8').replace('{{code}}', '{{password}}');
  assert.throws(() => parseCatalogue(raw), /placeholder/);
});
test('rejects unknown audience metadata', () => {
  const raw = readFileSync(new URL('../catalogue/emails.psv', import.meta.url), 'utf8').replace('|customer-new-contact|', '|everybody|');
  assert.throws(() => parseCatalogue(raw), /metadata/);
});
test('escapes HTML in names without escaping the whole layout', () => {
  const r = renderEmail(welcome, changed(welcome, { name: '<img src=x onerror=alert(1)>' }));
  assert.ok(r.html.includes('&lt;img'));
  assert.ok(!r.html.includes('<img'));
  assert.ok(r.html.includes('<table'));
});
test('rejects money represented as float', () => assert.throws(() => renderEmail(transfer, changed(transfer, { amount_minor: 2500.25 })), /amount/));
test('rejects exponent notation', () => assert.throws(() => money('1e4'), /integer/));
test('rejects negative amount in this NGN receipt contract', () => assert.throws(() => money('-100'), /integer/));
test('lossless amounts above JavaScript safe integer', () => assert.equal(money('900719925474099312345'), 'NGN 9,007,199,254,740,993,123.45'));
test('formats zero correctly', () => assert.equal(money('0'), 'NGN 0.00'));
test('rejects unsupported currency', () => assert.throws(() => validatePayload(transfer, changed(transfer, { currency: 'USD' })), /NGN/));
test('rejects mismatched audience', () => assert.throws(() => validatePayload(welcome, changed(welcome, { audience: 'staff-admin' })), /mismatch/));
test('rejects wrong financial state', () => assert.throws(() => validatePayload(transfer, changed(transfer, { state: 'timeout' })), /mismatch/));
test('rejects arbitrary URLs or data', () => assert.throws(() => validatePayload(welcome, changed(welcome, { action_url: 'https://evil.invalid' })), /fields/));
test('rejects an accidentally supplied PIN', () => assert.throws(() => validatePayload(welcome, changed(welcome, { pin: '1234' })), /fields/));
test('rejects control characters', () => assert.throws(() => validatePayload(welcome, changed(welcome, { name: 'Hello\r\nBcc: other' })), /name/));
test('rejects missing required field', () => { const p = fixture(welcome); delete p.recipient_ref; assert.throws(() => validatePayload(welcome, p), /fields/); });
test('rejects invalid calendar timestamp', () => assert.throws(() => validatePayload(welcome, changed(welcome, { occurred_at: '2026-02-30T12:00:00.000Z' })), /timestamp/));
test('rejects timezone-ambiguous timestamps', () => assert.throws(() => validatePayload(welcome, changed(welcome, { occurred_at: '2026-09-22 12:00' })), /UTC/));
test('rejects expiry beyond workflow TTL', () => assert.throws(() => validatePayload(code, changed(code, { expires_at: '2026-09-22T13:00:00.000Z' })), /expiry/));
test('rejects malformed code', () => assert.throws(() => validatePayload(code, changed(code, { code: '12345a' })), /code/));
test('optional notifications have plain-text and header unsubscribe material', () => { const r = renderEmail(optional, fixture(optional)); assert.ok(r.text.includes('Unsubscribe:')); assert.equal(r.headers['List-Unsubscribe-Post'], 'List-Unsubscribe=One-Click'); });
test('essential notices do not inherit optional unsubscribe', () => assert.deepEqual(renderEmail(code, fixture(code)).headers, {}));
test('rejects missing optional unsubscribe reference', () => { const p = fixture(optional); delete p.unsubscribe_ref; assert.throws(() => renderEmail(optional, p), /fields/); });
test('does not embed remote assets, tracking pixels or scripts', () => { for (const e of catalogue) assert.ok(!/<(?:img|script|iframe)\b/i.test(renderEmail(e, fixture(e)).html)); });
test('routes staff notices only to staff origin', () => { const e = find('finance-low-float'), r = renderEmail(e, fixture(e)); assert.ok(r.html.includes(previewBrand.staffOrigin)); assert.ok(!r.html.includes(previewBrand.customerOrigin)); });
test('rejects unsafe branding URL', () => assert.throws(() => validateBrand({ ...previewBrand, customerOrigin: 'javascript:alert(1)' }), /HTTPS/));
test('rejects URL credentials', () => assert.throws(() => validateBrand({ ...previewBrand, customerOrigin: 'https://name:password@safe.invalid' }), /credentials/));
test('preview brand fails production validation', () => assert.throws(() => validateBrand(previewBrand, true), /Preview/));

// Contract harness only: this does not import or execute the actual Novu SDK.
const brand = { name: 'Test product', legalName: 'Unit test entity', customerOrigin: 'https://app.qpf-unit.net', staffOrigin: 'https://admin.qpf-unit.net', businessOrigin: 'https://business.qpf-unit.net', supportEmail: 'support@qpf-unit.net', postalAddress: 'Unit test fixture address' };
function bindings(overrides = {}) {
  return { workflow: (id, handler, options) => ({ id, handler, options }), authorise: async () => ({ allowed: true }), recordSuppression: async () => {}, providerOptions: () => ({ qualificationOnly: () => ({}) }), brand, allowlist: [welcome.workflowId], now: () => Date.parse('2026-09-22T12:01:00.000Z'), ...overrides };
}
async function run(b, p = fixture(welcome), subscriberId = p.recipient_ref) {
  const [w] = defineWorkflows(b), sent = [];
  await w.handler({ payload: p, subscriber: { subscriberId }, step: { email: async (id, resolver, options) => { if (!(await options.skip())) sent.push(await resolver()); } } });
  return sent;
}
test('release defaults to no workflow activation', () => assert.deepEqual(defineWorkflows(bindings({ allowlist: [] })), []));
test('factory rejects missing policy binding', () => assert.throws(() => defineWorkflows(bindings({ authorise: undefined })), /bindings/));
test('factory rejects unknown workflow', () => assert.throws(() => defineWorkflows(bindings({ allowlist: ['not-real'] })), /Unknown/));
test('factory rejects duplicate workflow IDs', () => assert.throws(() => defineWorkflows(bindings({ allowlist: [welcome.workflowId, welcome.workflowId] })), /allowlist/));
test('growth requires an explicitly approved scope', () => assert.throws(() => defineWorkflows(bindings({ allowlist: [find('growth-campaign').workflowId] })), /unapproved/));
test('authorised workflow returns only accepted base output keys', async () => { const out = await run(bindings()); assert.equal(out.length, 1); assert.deepEqual(Object.keys(out[0]).sort(), ['body', 'subject']); });
test('expired message is suppressed and recorded', async () => { const reasons = []; const out = await run(bindings({ now: () => Date.parse('2026-09-24T12:00:00.000Z'), recordSuppression: async r => reasons.push(r) })); assert.equal(out.length, 0); assert.equal(reasons[0].reason, 'expired-or-future'); });
test('policy denial suppresses message', async () => assert.deepEqual(await run(bindings({ authorise: async () => ({ allowed: false, reason: 'superseded' }) })), []));
test('policy lookup failure does not send', async () => assert.rejects(run(bindings({ authorise: async () => { throw new Error('policy offline'); } })), /policy offline/));
test('invalid policy response does not send', async () => assert.rejects(run(bindings({ authorise: async () => ({ allowed: 'yes' }) })), /authorisation/));
test('subscriber mismatch does not send', async () => assert.rejects(run(bindings(), fixture(welcome), 'other_recipient'), /Subscriber/));
test('unqualified provider mapping does not send', async () => assert.rejects(run(bindings({ providerOptions: () => ({}) })), /mapping/));
test('future messages do not send early', async () => assert.deepEqual(await run(bindings({ now: () => Date.parse('2026-09-21T12:00:00.000Z') })), []));
test('invalid clock does not send', async () => assert.rejects(run(bindings({ now: () => NaN })), /clock/));
