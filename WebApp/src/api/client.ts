import type { Capabilities, Method, Payment, Quote, RequestOptions, Session, Transport, User, Wallet } from './types';
import { minor } from './money';

export class APIError extends Error {
  constructor(public status: number, public code: string, message: string, public requestId = '') { super(message); this.name = 'APIError'; }
}
export const record = (v: unknown): Record<string, unknown> => {
  if (!v || typeof v !== 'object' || Array.isArray(v)) throw new APIError(502, 'invalid_response', 'The API response is not in the expected format.');
  return v as Record<string, unknown>;
};
function text(v: unknown): string { if (typeof v !== 'string') throw new APIError(502, 'invalid_response', 'A required API field is missing.'); return v; }
function boolean(v: unknown): boolean { if (typeof v !== 'boolean') throw new APIError(502, 'invalid_response', 'The API returned an invalid capability.'); return v; }
function validDate(v: unknown): void { if (!Number.isFinite(Date.parse(text(v)))) throw new APIError(502, 'invalid_response', 'The API returned an invalid date.'); }
export function validateUser(v: unknown): User {
  const a = record(v); for (const k of ['id', 'email', 'name', 'role', 'status']) text(a[k]);
  for (const k of ['email_verified', 'mfa_enabled', 'pin_set']) boolean(a[k]);
  if (a.role !== 'customer' || !['active', 'restricted', 'closed'].includes(a.status as string) || !Number.isInteger(a.tier)) throw new APIError(403, 'wrong_account', 'Sign in with a customer account.');
  return v as User;
}
export function validateWallet(v: unknown): Wallet {
  const a = record(v); if (a.currency !== 'NGN') throw new APIError(502, 'invalid_response', 'Only NGN wallets are supported.');
  const available = BigInt(minor(a.available_minor)), held = BigInt(minor(a.held_minor)), book = BigInt(minor(a.balance_minor));
  if (available + held !== book) throw new APIError(502, 'invalid_response', 'The wallet totals are inconsistent. Refresh or contact support.');
  return v as Wallet;
}
export function validateQuote(v: unknown): Quote {
  const a = record(v); text(a.id); text(a.narration); record(a.destination); validDate(a.expires_at); boolean(a.used);
  if (a.currency !== 'NGN' || !['internal', 'bank', 'bill'].includes(text(a.kind)) || !Number.isInteger(a.policy_version) || BigInt(minor(a.amount_minor)) <= 0n || BigInt(minor(a.amount_minor)) + BigInt(minor(a.fee_minor)) !== BigInt(minor(a.total_minor))) throw new APIError(502, 'invalid_response', 'The quote totals or policy are invalid. No payment was approved.');
  return v as Quote;
}
export function validatePayment(v: unknown): Payment {
  const a = record(v); text(a.id); text(a.quote_id); validDate(a.created_at); validDate(a.updated_at);
  if (!['accepted', 'submitted', 'pending', 'succeeded', 'failed', 'pending_review'].includes(text(a.status)) || !['internal', 'bank', 'bill'].includes(text(a.kind)) || !['incoming', 'outgoing'].includes(text(a.direction)) || !['not_applicable', 'pending', 'ready'].includes(text(a.fulfilment_status)) || a.currency !== 'NGN' || BigInt(minor(a.amount_minor)) + BigInt(minor(a.fee_minor)) !== BigInt(minor(a.total_minor))) throw new APIError(502, 'invalid_response', 'The payment response needs investigation. Do not submit a replacement.');
  return v as Payment;
}
export function validateCapabilities(v: unknown): Capabilities {
  const a = record(v); text(a.environment); text(a.currency); for (const k of ['external_payments_configured', 'synthetic_execution', 'growth_products_enabled', 'production_acceptance']) boolean(a[k]);
  return v as Capabilities;
}
export function validated<T>(path: string, data: unknown): T {
  const p = path.split('?')[0];
  if (p === '/v1/capabilities') return validateCapabilities(data) as T;
  if (p === '/v1/wallet') return validateWallet(data) as T;
  if (p === '/v1/me' || p === '/v1/auth/me') return validateUser(data) as T;
  if (p === '/v1/auth/login' || p === '/v1/auth/refresh' || p === '/v1/auth/passkeys/finish') { const a = record(data); validateUser(a.user); text(a.csrf_token); if (a.access_token || a.refresh_token) throw new APIError(502, 'unsafe_session', 'The API returned credentials for the wrong transport.'); return data as T; }
  if (/^\/v1\/quotes(?:\/[^/]+)?$/.test(p)) return validateQuote(data) as T;
  if (p !== '/v1/payments/lookup' && /^\/v1\/payments\/[^/]+$/.test(p)) return validatePayment(data) as T;
  if (p === '/v1/payments') {
    if (Array.isArray(data)) return data.map(validatePayment) as T;
    return validatePayment(data) as T;
  }
  return data as T;
}
export function safeBaseURL(value: string, localAllowed = false): string {
  const u = new URL(value);
  if (u.username || u.password || u.search || u.hash || u.pathname !== '/' || (u.protocol !== 'https:' && !(localAllowed && u.protocol === 'http:' && ['localhost', '127.0.0.1', '[::1]'].includes(u.hostname)))) throw new Error('Configure an HTTPS API origin, or localhost HTTP for development.');
  return u.origin;
}
export class HttpTransport implements Transport {
  csrf = '';
  private refreshInFlight: Promise<void> | undefined;
  constructor(public base: string, private fetcher: typeof fetch = fetch) {}
  clear() { this.csrf = ''; }
  private async call<T>(method: Method, path: string, options: RequestOptions = {}): Promise<T> {
    if (!path.startsWith('/v1/') || path.includes('..') || path.includes('\\') || path.includes('#')) throw new Error('Invalid API path.');
    const headers: Record<string, string> = { Accept: options.raw ? 'text/csv' : 'application/json' };
    const multipart = typeof FormData !== 'undefined' && options.body instanceof FormData;
    if (options.body !== undefined && !multipart) headers['Content-Type'] = 'application/json';
    if (method !== 'GET' && this.csrf) headers['X-CSRF-Token'] = this.csrf;
    if (options.idempotencyKey) headers['Idempotency-Key'] = options.idempotencyKey;
    let response: Response;
    try {
      response = await this.fetcher(this.base + path, { method, headers, body: options.body === undefined ? undefined : multipart ? options.body as FormData : JSON.stringify(options.body), credentials: 'include', cache: 'no-store', redirect: 'error', signal: options.signal ? AbortSignal.any([options.signal, AbortSignal.timeout(multipart ? 60000 : 25000)]) : AbortSignal.timeout(multipart ? 60000 : 25000) });
    } catch (error) {
      if (options.signal?.aborted) throw error;
      throw new APIError(0, 'network_unknown', method === 'GET' ? 'Cannot reach the API. Check your connection and try refreshing.' : 'The response was not received. The outcome may be unknown; check the original operation before retrying.');
    }
    if (response.status === 204 && response.ok) return undefined as T;
    if (options.raw && response.ok) return await response.blob() as T;
    let envelope: Record<string, unknown>;
    try { envelope = record(await response.json()); } catch { throw new APIError(response.status, 'invalid_response', 'The server did not return a valid response.'); }
    if (!response.ok) {
      const error = record(envelope.error ?? {});
      throw new APIError(response.status, String(error.code ?? 'request_failed'), String(error.message ?? 'Request could not be completed.'), String(envelope.request_id ?? ''));
    }
    const data = validated<T>(path, envelope.data);
    if (path === '/v1/auth/login' || path === '/v1/auth/refresh' || path === '/v1/auth/passkeys/finish') this.csrf = (data as Session).csrf_token;
    return data;
  }
  async refreshCSRF() { const data = await this.call<{ csrf_token: string }>('GET', '/v1/auth/csrf'); this.csrf = text(data.csrf_token); }
  async recoverSession(): Promise<void> {
    if (this.refreshInFlight) return this.refreshInFlight;
    const perform = async () => {
      try { await this.call<User>('GET', '/v1/auth/me'); await this.refreshCSRF(); return; }
      catch (error) { if (!(error instanceof APIError) || error.status !== 401) throw error; }
      await this.refreshCSRF();
      await this.call<Session>('POST', '/v1/auth/refresh', { body: { client: 'web' } });
    };
    this.refreshInFlight = Promise.resolve(typeof navigator !== 'undefined' && navigator.locks ? navigator.locks.request('qpf-refresh:' + this.base, perform) : perform()).then(() => {}).finally(() => { this.refreshInFlight = undefined; });
    return this.refreshInFlight;
  }
  async request<T>(method: Method, path: string, options: RequestOptions = {}): Promise<T> {
    const publicAuth = ['/v1/auth/login', '/v1/auth/register', '/v1/auth/challenges', '/v1/auth/challenges/verify', '/v1/auth/passkeys/begin','/v1/auth/passkeys/finish'].includes(path);
    if (method !== 'GET' && !publicAuth && !this.csrf) await this.refreshCSRF();
    try { return await this.call<T>(method, path, options); }
    catch (error) {
      // Only reads may replay automatically. Financial and credential writes never do.
      if (error instanceof APIError && error.status === 401 && method === 'GET' && !path.startsWith('/v1/auth/')) {
        await this.recoverSession(); return this.call<T>(method, path, options);
      }
      throw error;
    }
  }
}
export async function restoreUser(client: Transport): Promise<User> {
  try { return await client.request<User>('GET', '/v1/auth/me'); }
  catch (error) { if (client instanceof HttpTransport && error instanceof APIError && error.status === 401) { await client.recoverSession(); return client.request<User>('GET', '/v1/auth/me'); } throw error; }
}
export function message(error: unknown): string { return error instanceof Error ? error.message : 'Something went wrong. Try again.'; }
