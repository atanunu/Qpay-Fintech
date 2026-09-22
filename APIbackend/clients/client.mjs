/** Transport helper for browser and React Native clients. No automatic financial retries. */
export class APIError extends Error {
  constructor(status, code, message, requestId = '') {
    super(message); this.name = 'APIError'; this.status = status; this.code = code; this.requestId = requestId;
  }
}
export function minor(value) {
  if (typeof value !== 'string' || !/^(0|[1-9][0-9]{0,15})$/.test(value) || BigInt(value) > 9000000000000000n) throw new TypeError('Use a non-negative integer minor-unit string within the API limit');
  return value;
}
export class QpayClient {
  constructor({ baseURL, transport, fetchImpl = globalThis.fetch, getAccessToken = () => '', getCSRFToken = () => '', allowLocalHTTP = false }) {
    const url = new URL(baseURL);
    const local = ['localhost', '127.0.0.1'].includes(url.hostname);
    if (url.username || url.password || url.search || url.hash || url.pathname !== '/' || (url.protocol !== 'https:' && !(allowLocalHTTP && local && url.protocol === 'http:'))) throw new TypeError('Use an approved HTTPS API origin');
    if (!['web', 'mobile'].includes(transport) || typeof fetchImpl !== 'function') throw new TypeError('Transport and fetch implementation required');
    Object.assign(this, { baseURL: url.origin, transport, fetchImpl, getAccessToken, getCSRFToken });
  }
  async request(method, path, { body, idempotencyKey, signal } = {}) {
    if (!/^(GET|POST|PATCH|DELETE)$/.test(method) || !/^\/v1\//.test(path) || path.includes('..') || path.includes('\\')) throw new TypeError('Use a relative v1 API path');
    const headers = { Accept: 'application/json' };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (this.transport === 'mobile') { const token = await this.getAccessToken(); if (token) headers.Authorization = `Bearer ${token}`; }
    else if (method !== 'GET') { const token = await this.getCSRFToken(); if (token) headers['X-CSRF-Token'] = token; }
    if (idempotencyKey) headers['Idempotency-Key'] = idempotencyKey;
    const response = await this.fetchImpl(this.baseURL + path, { method, headers, credentials: this.transport === 'web' ? 'include' : 'omit', body: body === undefined ? undefined : JSON.stringify(body), signal, redirect: 'error' });
    if (response.status === 204) return undefined;
    let result; try { result = await response.json(); } catch { throw new APIError(response.status, 'invalid_response', 'Server response was not valid JSON'); }
    if (!response.ok) throw new APIError(response.status, result.error?.code ?? 'request_failed', result.error?.message ?? 'Request failed', result.request_id ?? '');
    return result.data;
  }
  quote(input, options) { minor(input.amount_minor); return this.request('POST', '/v1/quotes', { ...options, body: input }); }
  authorise(quoteId, pin, mfaCode, options) { return this.request('POST', `/v1/quotes/${encodeURIComponent(quoteId)}/authorisations`, { ...options, body: { pin, ...(mfaCode ? { mfa_code: mfaCode } : {}) } }); }
  pay(quoteId, token, idempotencyKey, options) {
    if (typeof idempotencyKey !== 'string' || idempotencyKey.length < 16 || idempotencyKey.length > 100) throw new TypeError('Persist a unique 16–100 character key before submitting');
    return this.request('POST', '/v1/payments', { ...options, idempotencyKey, body: { quote_id: quoteId, authorisation_token: token } });
  }
}
