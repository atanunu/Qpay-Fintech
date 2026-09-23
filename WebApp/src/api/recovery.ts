import type { Payment, Transport } from './types';
/** Session storage contains only operation references, never PINs, tokens, contact or bank details. */
export interface Intent { version: 1; owner: string; quoteId: string; idempotencyKey: string; paymentId?: string; createdAt: string; phase: 'submitting' | 'unknown' | 'accepted' }
const KEY = 'qpf.payment-recovery.v1';
export function saveIntent(intent: Intent, storage: Storage = sessionStorage): void {
  storage.setItem(KEY, JSON.stringify(intent)); // Refuse submission when safe recovery metadata cannot be persisted.
}
export function loadIntent(owner: string, storage: Storage = sessionStorage): Intent | null {
  try {
    const v = JSON.parse(storage.getItem(KEY) || 'null');
    if (!v || v.version !== 1 || v.owner !== owner || typeof v.quoteId !== 'string' || typeof v.idempotencyKey !== 'string' || !['submitting', 'unknown', 'accepted'].includes(v.phase)) return null;
    return v;
  } catch { return null; }
}
export function clearIntent(storage: Storage = sessionStorage): void { storage.removeItem(KEY); }
export async function findOriginal(client: Transport, intent: Intent): Promise<Payment | null> {
  if (intent.paymentId) return client.request('GET', `/v1/payments/${encodeURIComponent(intent.paymentId)}`);
  let before = '';
  for (let page = 0; page < 10; page++) {
    const records = await client.request<Payment[]>('GET', `/v1/payments?limit=100${before ? '&before=' + encodeURIComponent(before) : ''}`);
    const original = records.find(x => x.quote_id === intent.quoteId);
    if (original) return original;
    if (records.length < 100) return null;
    before = records[records.length - 1].id;
  }
  return null; // Not-found is not proof of failure. UI must retain unknown status.
}
