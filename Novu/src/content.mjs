/** Built-in email content; no network calls, recipients or credentials are stored here. */
import { readFileSync } from 'node:fs';

export const themes = Object.freeze({
  core: { label: 'Account update', ink: '#17364a', accent: '#17645b', paper: '#f1f6f8', shape: '8px' },
  security: { label: 'Account security', ink: '#2c2850', accent: '#5a3d85', paper: '#f4f2f9', shape: '0px' },
  receipt: { label: 'Transaction record', ink: '#173b31', accent: '#22684e', paper: '#eff5f1', shape: '4px' },
  support: { label: 'Service and support', ink: '#423428', accent: '#8a4d20', paper: '#faf4ed', shape: '12px' },
  operations: { label: 'Authorised operations', ink: '#152539', accent: '#254e82', paper: '#eef2f7', shape: '0px' },
  campaign: { label: 'Optional account offer', ink: '#4c2342', accent: '#843568', paper: '#faf0f7', shape: '16px' }
});
export const previewBrand = Object.freeze({
  name: 'Qpay-Fintech', legalName: 'Qpay-Fintech — synthetic content preview',
  customerOrigin: 'https://app.example.invalid', staffOrigin: 'https://admin.example.invalid',
  businessOrigin: 'https://business.example.invalid', supportEmail: 'support@example.invalid',
  postalAddress: 'Approved legal address required before live sending'
});
const classes = ['essential', 'transactional', 'optional', 'marketing', 'operational'];
const audiences = ['customer', 'customer-old-contact', 'customer-new-contact', 'business', 'staff-admin', 'staff-support', 'staff-finance', 'staff-security', 'staff-platform'];
const scopes = ['core', 'growth', 'regulated'];
const clean = (s, max = 1000) => typeof s === 'string' && s.length > 0 && s.length <= max && !/[\x00-\x1f\x7f]/.test(s);
const safeId = s => clean(s, 100) && /^[A-Za-z0-9][A-Za-z0-9_-]{5,99}$/.test(s);
export const escapeHTML = s => String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]);
const fail = message => { throw new Error(message); };

export function parseCatalogue(text) {
  const lines = text.trim().split(/\r?\n/);
  if (lines.shift() !== 'key|audience|scope|class|theme|guard|subject|body') fail('Invalid catalogue header');
  const seen = new Set();
  return lines.map((line, i) => {
    const fields = line.split('|');
    if (fields.length !== 8) fail(`Invalid catalogue row ${i + 2}`);
    const [key, audience, scope, classification, theme, guard, subject, body] = fields;
    if (!/^[a-z][a-z0-9-]+$/.test(key) || seen.has(key)) fail('Invalid or duplicate workflow key');
    seen.add(key);
    if (!audiences.includes(audience) || !scopes.includes(scope) || !classes.includes(classification) || !Object.hasOwn(themes, theme)) fail(`Invalid metadata: ${key}`);
    if (!/^[a-z][a-z0-9_]+$/.test(guard) || !clean(subject, 120) || !clean(body)) fail(`Invalid copy/guard: ${key}`);
    const tokens = [...body.matchAll(/\{\{([^}]+)\}\}/g)].map(x => x[1]);
    if (tokens.some(t => !['name', 'reference', 'amount', 'code'].includes(t)) || /\{\{|\}\}/.test(body.replace(/\{\{(?:name|reference|amount|code)\}\}/g, ''))) fail(`Unknown placeholder: ${key}`);
    return Object.freeze({ key, workflowId: `qpf-email-${key}-v1`, eventType: `qpf.notification.${key}.v1`, audience, scope, classification, theme, guard, subject, body, tokens, ttlSeconds: tokens.includes('code') ? 600 : classification === 'operational' ? 3600 : 86400, status: 'content-built; live-integration-pending' });
  });
}
export const catalogue = parseCatalogue(readFileSync(new URL('../catalogue/emails.psv', import.meta.url), 'utf8'));

export function payloadSchema(event) {
  const properties = {
    version: { type: 'integer', const: 1 },
    event_id: { type: 'string', pattern: '^[A-Za-z0-9][A-Za-z0-9_-]{5,99}$' },
    notification_id: { type: 'string', pattern: '^[A-Za-z0-9][A-Za-z0-9_-]{5,99}$' },
    recipient_ref: { type: 'string', pattern: '^[A-Za-z0-9][A-Za-z0-9_-]{5,99}$' },
    reference: { type: 'string', pattern: '^[A-Za-z0-9][A-Za-z0-9_-]{5,99}$' },
    audience: { type: 'string', const: event.audience },
    state: { type: 'string', const: event.guard },
    locale: { type: 'string', const: 'en-NG' },
    name: { type: 'string', minLength: 1, maxLength: 80 },
    occurred_at: { type: 'string', format: 'date-time' },
    expires_at: { type: 'string', format: 'date-time' }
  };
  if (event.tokens.includes('amount')) Object.assign(properties, { amount_minor: { type: 'string', pattern: '^(0|[1-9][0-9]{0,29})$' }, currency: { type: 'string', const: 'NGN' } });
  if (event.tokens.includes('code')) properties.code = { type: 'string', pattern: '^[0-9]{6}$' };
  if (['optional', 'marketing'].includes(event.classification)) properties.unsubscribe_ref = { type: 'string', pattern: '^[a-f0-9]{64}$' };
  return { type: 'object', additionalProperties: false, required: Object.keys(properties), properties };
}
function timestamp(value) {
  if (typeof value !== 'string' || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z$/.test(value)) fail('UTC timestamp with milliseconds required');
  const ms = Date.parse(value);
  if (!Number.isFinite(ms) || new Date(ms).toISOString() !== value) fail('Invalid timestamp');
  return ms;
}
export function validatePayload(event, p) {
  if (!p || typeof p !== 'object' || Array.isArray(p)) fail('Payload must be an object');
  const keys = Object.keys(payloadSchema(event).properties);
  if (Object.keys(p).some(k => !keys.includes(k)) || keys.some(k => !Object.hasOwn(p, k))) fail('Unknown or missing payload fields');
  if (p.version !== 1 || p.audience !== event.audience || p.state !== event.guard || p.locale !== 'en-NG') fail('Payload contract mismatch');
  for (const k of ['event_id', 'notification_id', 'recipient_ref', 'reference']) if (!safeId(p[k])) fail(`Invalid ${k}`);
  if (!clean(p.name, 80)) fail('Invalid display name');
  const start = timestamp(p.occurred_at), end = timestamp(p.expires_at);
  if (end <= start || end - start > event.ttlSeconds * 1000) fail('Invalid notification expiry window');
  if (event.tokens.includes('amount') && (typeof p.amount_minor !== 'string' || !/^(0|[1-9][0-9]{0,29})$/.test(p.amount_minor) || p.currency !== 'NGN')) fail('Invalid exact NGN amount');
  if (event.tokens.includes('code') && (typeof p.code !== 'string' || !/^[0-9]{6}$/.test(p.code))) fail('Invalid one-time code');
  if (Object.hasOwn(p, 'unsubscribe_ref') && !/^[a-f0-9]{64}$/.test(p.unsubscribe_ref)) fail('Invalid unsubscribe reference');
  return p;
}
export function money(minor) {
  if (typeof minor !== 'string' || !/^(0|[1-9][0-9]{0,29})$/.test(minor)) fail('Lossless integer amount required');
  const n = BigInt(minor), whole = (n / 100n).toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  return `NGN ${whole}.${(n % 100n).toString().padStart(2, '0')}`;
}
export function validateBrand(b, live = false) {
  for (const k of ['name', 'legalName', 'supportEmail', 'postalAddress']) if (!clean(b?.[k], 200)) fail(`Invalid brand ${k}`);
  if (!/^[A-Za-z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$/.test(b.supportEmail)) fail('Invalid support email');
  for (const k of ['customerOrigin', 'staffOrigin', 'businessOrigin']) {
    const u = new URL(b[k]);
    if (u.protocol !== 'https:' || u.username || u.password || u.pathname !== '/' || u.search || u.hash) fail('HTTPS origin without path or credentials required');
    if (live && /(?:^localhost$|\.(?:invalid|example|test)$|^example\.)/.test(u.hostname)) fail('Preview origin cannot be deployed');
  }
  if (live && /example|synthetic|required before/i.test(`${b.supportEmail} ${b.legalName} ${b.postalAddress}`)) fail('Preview legal/sender configuration cannot be deployed');
  return b;
}
export function fixture(event) {
  const p = { version: 1, event_id: 'evt_synthetic_001', notification_id: 'notice_synthetic_001', recipient_ref: 'recipient_synthetic_001', reference: 'QPF_SYNTHETIC_001', audience: event.audience, state: event.guard, locale: 'en-NG', name: 'Sample Customer', occurred_at: '2026-09-22T12:00:00.000Z', expires_at: new Date(Date.parse('2026-09-22T12:00:00.000Z') + event.ttlSeconds * 1000).toISOString() };
  if (event.tokens.includes('amount')) Object.assign(p, { amount_minor: '250000', currency: 'NGN' });
  if (event.tokens.includes('code')) p.code = '123456';
  if (['optional', 'marketing'].includes(event.classification)) p.unsubscribe_ref = 'a'.repeat(64);
  return p;
}
export function renderEmail(event, payload, brand = previewBrand) {
  validatePayload(event, payload); validateBrand(brand);
  const t = themes[event.theme], e = escapeHTML;
  const values = { name: payload.name, reference: payload.reference, amount: event.tokens.includes('amount') ? money(payload.amount_minor) : '', code: payload.code ?? '' };
  const message = event.body.replace(/\{\{(name|reference|amount|code)\}\}/g, (_, key) => values[key]);
  const origin = event.audience.startsWith('staff-') ? brand.staffOrigin : event.audience === 'business' ? brand.businessOrigin : brand.customerOrigin;
  const action = `${origin}/notifications/view/${encodeURIComponent(payload.reference)}`;
  const unsubscribe = payload.unsubscribe_ref ? `${origin}/email/unsubscribe/${payload.unsubscribe_ref}` : null;
  const headers = unsubscribe ? { 'List-Unsubscribe': `<${unsubscribe}>`, 'List-Unsubscribe-Post': 'List-Unsubscribe=One-Click' } : {};
  const security = event.theme === 'security' ? '<p style="border-left:4px solid #5a3d85;padding:12px">Never share your password, transaction PIN or one-time codes with anyone, including support.</p>' : '';
  const receipt = event.tokens.includes('amount') ? `<table role="presentation" width="100%" style="border-top:1px solid #d4dedb;border-bottom:1px solid #d4dedb"><tr><td style="padding:16px 0">Amount</td><td align="right" style="font-size:24px;font-weight:bold">${e(values.amount)}</td></tr><tr><td>Reference</td><td align="right">${e(payload.reference)}</td></tr></table>` : '';
  const actionText = event.audience.startsWith('staff-') || event.audience === 'business' ? 'Open secure dashboard' : 'Open secure application';
  const html = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${e(event.subject)}</title></head><body style="margin:0;background:${t.paper};font-family:Arial,Helvetica,sans-serif;color:${t.ink}"><table role="presentation" width="100%" cellspacing="0" cellpadding="0"><tr><td align="center" style="padding:32px 12px"><table role="presentation" width="600" style="width:100%;max-width:600px;background:#ffffff;border-radius:${t.shape};overflow:hidden" cellspacing="0" cellpadding="0"><tr><td style="padding:24px 32px;background:${t.ink};color:#ffffff;font-size:22px;font-weight:bold">${e(brand.name)}</td></tr><tr><td style="padding:30px 32px"><p style="font-size:12px;letter-spacing:1px;text-transform:uppercase;color:${t.accent}">${e(t.label)}</p><h1 style="font-size:26px;line-height:1.3;margin:12px 0 20px">${e(event.subject)}</h1><p style="font-size:16px;line-height:1.7">${e(message)}</p>${receipt}${security}<p style="margin:30px 0"><a href="${e(action)}" style="display:inline-block;padding:14px 20px;background:${t.accent};color:#ffffff;text-decoration:none;font-weight:bold;border-radius:4px">${e(actionText)}</a></p><p style="font-size:13px;line-height:1.6">You can also open your usual application directly. This email cannot approve a payment or replace its current transaction record.</p>${unsubscribe ? `<p style="font-size:13px"><a href="${e(unsubscribe)}">Unsubscribe from this optional email category</a></p>` : ''}</td></tr><tr><td style="padding:20px 32px;border-top:1px solid #dce3e8;font-size:12px;line-height:1.7">${e(brand.legalName)}<br>${e(brand.postalAddress)}<br>Support: ${e(brand.supportEmail)}</td></tr></table></td></tr></table></body></html>\n`;
  const text = `${brand.name}\n${event.subject}\n\n${message}\n\n${actionText}: ${action}\nOpen your usual application directly if you prefer. Email cannot approve a payment.\n${unsubscribe ? `\nUnsubscribe: ${unsubscribe}\n` : ''}\n${brand.legalName}\n${brand.postalAddress}\nSupport: ${brand.supportEmail}\n`;
  return { subject: event.subject, html, text, headers, workflowId: event.workflowId, theme: event.theme };
}
