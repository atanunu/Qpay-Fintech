import { reviewParity, beforeReviewPayment, afterReviewPayment, type ParityState } from './parity';
import { operationID } from '../api/id';
/** Explicitly synthetic UI harness. Never imported as a fallback for an API error. */
import { APIError } from '../api/client';
import type { Approval, Beneficiary, Capabilities, Device, Entry, Funding, Kind, KycCase, Message, Method, Notice, Payment, Preferences, Product, Quote, QuoteInput, RequestOptions, Session, Statement, SupportCase, Transport, User, Wallet } from '../api/types';

export const reviewCredentials = { email: 'review@qpay.example.invalid', password: 'Review-only-2026', code: '123456', pin: '123456', recipient: 'usr_review_tunde_02' };
export type Scenario = 'success' | 'pending' | 'failed' | 'unknown' | 'expired' | 'insufficient' | 'restricted' | 'offline';
export interface ReviewControls { sampleEmail: string; scenario: Scenario; setScenario(value: Scenario): void; reset(): void; resolvePending(): void }
export interface State { parity?: ParityState; version: 1; signedIn: boolean; user: User; balance: string; held: string; payments: Payment[]; quotes: Record<string, Quote>; beneficiaries: Beneficiary[]; notices: Notice[]; preferences: Preferences; cases: SupportCase[]; messages: Record<string, Message[]>; kyc: KycCase[]; devices: Device[]; entries: Entry[]; funding: Funding[]; idempotency: Record<string, string> }
const storeKey = 'qpf.synthetic-review.v1';
const now = () => new Date().toISOString();
const id = (kind: string) => `${kind}_review_${operationID().replaceAll('-', '')}`;
const at = (days: number) => new Date(Date.now() - days * 86400000).toISOString();
const user = (): User => ({ id: 'usr_review_amina_01', email: reviewCredentials.email, name: 'Amina Okoro', role: 'customer', status: 'active', email_verified: true, tier: 1, mfa_enabled: false, pin_set: true, version: 1, created_at: at(35) });
const products: Product[] = [
  { id: 'review-airtime', name: 'Mobile airtime', category: 'airtime', amount_minor: '0', variable_amount: true },
  { id: 'review-data-5gb', name: '5 GB · 30 days', category: 'data', amount_minor: '250000', variable_amount: false },
  { id: 'review-electricity', name: 'Prepaid electricity', category: 'electricity', amount_minor: '0', variable_amount: true },
  { id: 'review-tv', name: 'Family television package', category: 'television', amount_minor: '550000', variable_amount: false },
  { id: 'review-internet', name: 'Home internet · 30 days', category: 'internet', amount_minor: '1500000', variable_amount: false },
  { id: 'review-other', name: 'Other biller', category: 'other', amount_minor: '0', variable_amount: true },
];
function initial(): State {
  const payments: Payment[] = [
    ['bank', 'succeeded', '2500000', 0, 'outgoing'], ['bill', 'succeeded', '250000', 1, 'outgoing'], ['internal', 'succeeded', '7500000', 2, 'incoming'], ['bank', 'pending', '1000000', 3, 'outgoing'], ['bill', 'failed', '500000', 4, 'outgoing'],
  ].map((v, i) => ({ id: `pay_review_history_0${i}`, quote_id: `quo_review_history_0${i}`, kind: v[0] as Kind, status: v[1] as Payment['status'], amount_minor: String(v[2]), fee_minor: '0', total_minor: String(v[2]), currency: 'NGN', fulfilment_status: v[0] === 'bill' && v[1] === 'succeeded' ? 'ready' : 'not_applicable', direction: v[4] as Payment['direction'], created_at: at(Number(v[3])), updated_at: at(Number(v[3])) }));
  return { version: 1, signedIn: false, user: user(), balance: '28475000', held: '1000000', payments, quotes: {}, idempotency: {}, beneficiaries: [{ id: 'ben_review_tunde_01', label: 'Tunde Adeyemi', account_name: 'TUNDE ADEYEMI · SYNTHETIC' }, { id: 'ben_review_studio_02', label: 'Studio account', account_name: 'STUDIO · SYNTHETIC' }], notices: [{ id: 'notice_review_fund_001', workflow: 'funding-received', subject: 'Funds credited to your wallet', reference: 'fund_review_0001', created_at: at(0), read: false }, { id: 'notice_review_transfer_002', workflow: 'transfer-completed', subject: 'Transfer completed', reference: payments[0].id, created_at: at(1), read: false }], preferences: { optional_email: false, marketing_email: false }, cases: [{ id: 'case_review_0001', owner_id: user().id, payment_id: payments[3].id, kind: 'support', subject: 'A transfer is taking longer than expected', status: 'open', created_at: at(2), updated_at: at(1) }], messages: { case_review_0001: [{ id: 'msg_review_1', author_id: user().id, message: 'This is a synthetic support conversation for the review session.', created_at: at(2) }, { id: 'msg_review_2', author_id: 'staff_review_001', message: 'We are checking the original transaction. Please do not send a replacement.', created_at: at(1) }] }, kyc: [{ id: 'kyc_review_001', owner_id: user().id, status: 'approved', evidence_reference: 'synthetic-review-evidence', created_at: at(30) }], devices: [{ id: 'ses_review_current', device: 'This browser', client: 'web', created_at: at(0), expires_at: at(-1), revoked: false, current: true }, { id: 'ses_review_mobile', device: 'Sample mobile device', client: 'mobile', created_at: at(3), expires_at: at(-1), revoked: false, current: false }], entries: [{ id: 2, journal_id: 'jnl_review_002', reference: payments[0].id, kind: 'external_payment', delta_minor: '-2500000', created_at: at(0) }, { id: 1, journal_id: 'jnl_review_001', reference: 'fund_review_0001', kind: 'funding', delta_minor: '30975000', created_at: at(1) }], funding: [{ id: 'fund_review_0001', amount_minor: '30975000', currency: 'NGN', created_at: at(1) }] };
}
export class ReviewTransport implements Transport, ReviewControls {
  csrf = 'synthetic-csrf';
  scenario: Scenario = 'success';
  private state: State;
  private bankName = '';
  private validations = new Map<string, { product: Product; customer: string; amount: string }>();
  private approvals = new Map<string, Approval>();
  constructor(private storage: Storage = sessionStorage) {
    try { const v = storage.getItem('qpf.synthetic-scenario.v1'); if (v && ['success','pending','failed','unknown','expired','insufficient','restricted','offline'].includes(v)) this.scenario = v as Scenario; } catch { /* Synthetic selection is optional. */ }
    try { const saved = JSON.parse(storage.getItem(storeKey) || 'null'); this.state = saved?.version === 1 ? saved : initial(); } catch { this.state = initial(); }
  }
  get sampleEmail() { return this.state.user.email; }
  clear() { this.csrf = ''; }
  private persist() { this.storage.setItem(storeKey, JSON.stringify(this.state)); }
  setScenario(value: Scenario) { this.storage.setItem('qpf.synthetic-scenario.v1', value); this.scenario = value; }
  reset() { this.storage.removeItem('qpf.synthetic-scenario.v1'); this.state = initial(); this.state.signedIn = false; this.scenario = 'success'; this.approvals.clear(); this.storage.removeItem('qpf.payment-recovery.v1'); this.persist(); }
  resolvePending() {
    this.state.payments.filter(p => p.status === 'pending' || p.status === 'accepted').forEach(p => {
      p.status = 'succeeded'; p.updated_at = now();
      this.state.balance = (BigInt(this.state.balance) - BigInt(p.total_minor)).toString();
      this.state.held = (BigInt(this.state.held) - BigInt(p.total_minor)).toString();
      if (p.kind === 'bill') p.fulfilment_status = 'ready';
      if (!this.state.entries.some(e => e.reference === p.id)) this.state.entries.unshift({ id: Math.max(0, ...this.state.entries.map(e => e.id)) + 1, journal_id: 'jnl_' + p.id, reference: p.id, kind: p.kind === 'internal' ? 'internal_transfer' : 'external_payment', delta_minor: '-' + p.total_minor, created_at: p.updated_at });
      afterReviewPayment(this.state, p);
    }); this.persist();
  }
  async request<T>(method: Method, inputPath: string, options: RequestOptions = {}): Promise<T> {
    if (options.signal?.aborted) throw new DOMException('Aborted', 'AbortError');
    await new Promise(resolve => setTimeout(resolve, 140));
    if (options.signal?.aborted) throw new DOMException('Aborted', 'AbortError');
    if (this.scenario === 'offline' && !inputPath.includes('capabilities')) throw new APIError(0, 'network_unknown', 'Synthetic offline scenario. Change the review scenario to reconnect.');
    const result = this.route(method, inputPath, options); this.persist(); return structuredClone(result) as T;
  }
  private route(method: Method, inputPath: string, options: RequestOptions): unknown {
    const url = new URL(inputPath, 'https://synthetic.invalid'); const path = url.pathname;
    const b = (options.body || {}) as Record<string, string>;
    const s = this.state;
    const session = (): Session => ({ csrf_token: 'synthetic-csrf', expires_at: at(-1), refresh_expires_at: at(-30), user: s.user, mfa_enrolment_required: false });
    if (path === '/v1/capabilities') return { environment: 'review', currency: 'NGN', external_payments_configured: true, notifications: 'synthetic', synthetic_execution: true, growth_products_enabled: false, production_acceptance: false } satisfies Capabilities;
    if (path === '/v1/auth/login') {
      if (b.email !== s.user.email || b.password !== reviewCredentials.password || (s.user.mfa_enabled && b.mfa_code !== '123456' && !b.recovery_code)) throw new APIError(401, 'unauthorized', 'Use the displayed synthetic review credentials.');
      s.signedIn = true; this.csrf = 'synthetic-csrf'; return session();
    }
    if (path === '/v1/auth/register') {
      if (!b.email.endsWith('.invalid')) throw new APIError(400, 'synthetic_only', 'Use an example.invalid email in the synthetic review.');
      s.user = { ...user(), email: b.email, name: b.name, email_verified: false, tier: 0, pin_set: false }; s.signedIn = false; return { status: 'accepted' };
    }
    if (path === '/v1/auth/challenges') return { status: 'accepted' };
    if (path === '/v1/auth/challenges/verify') { if (b.code !== '123456') throw new APIError(401, 'unauthorized', 'The review code is 123456.'); s.user.email_verified = true; return { status: 'accepted' }; }
    if (!s.signedIn) throw new APIError(401, 'unauthorized', 'Sign in to continue.');
    const parityResult = reviewParity(s, method, url, options); if (parityResult.handled) return parityResult.value;
    if (path === '/v1/auth/csrf') return { csrf_token: 'synthetic-csrf' };
    if (path === '/v1/auth/refresh') return session();
    if (path === '/v1/auth/logout') { s.signedIn = false; return undefined; }
    if (path === '/v1/auth/me' || path === '/v1/me') { if (method === 'PATCH') s.user.name = b.name; return s.user; }
    if (path === '/v1/wallet') {
      const balance = this.scenario === 'insufficient' ? '0' : s.balance; const held = this.scenario === 'insufficient' ? '0' : s.held;
      return { currency: 'NGN', balance_minor: balance, held_minor: held, available_minor: (BigInt(balance) - BigInt(held)).toString() } satisfies Wallet;
    }
    if (path === '/v1/wallet/funding') return s.funding;
    if (path === '/v1/wallet/entries') return s.entries;
    if (path === '/v1/banks') return [{ code: '999', name: 'Review Bank · synthetic only' }];
    if (path === '/v1/banks/enquiries') { this.bankName = 'SYNTHETIC RECIPIENT'; return { id: id('enq'), name: this.bankName, expires_at: new Date(Date.now() + 300000).toISOString(), amount_minor: '0' }; }
    if (path === '/v1/beneficiaries') {
      if (method === 'POST') { const v = { id: id('ben'), label: b.label, account_name: this.bankName || 'SYNTHETIC RECIPIENT' }; s.beneficiaries.push(v); return { id: v.id }; }
      return s.beneficiaries;
    }
    if (/^\/v1\/beneficiaries\//.test(path) && method === 'DELETE') { s.beneficiaries = s.beneficiaries.filter(x => x.id !== path.split('/').pop()); return undefined; }
    if (path === '/v1/bills/products') return products;
    if (path === '/v1/bills/validations') {
      const product = products.find(x => x.id === b.product_id); if (!product) throw new APIError(404, 'not_found', 'Product unavailable.');
      const validation = id('val'); this.validations.set(validation, { product, customer: b.customer_id, amount: b.amount_minor }); return { id: validation, name: 'SYNTHETIC BILL CUSTOMER', expires_at: new Date(Date.now() + 300000).toISOString(), amount_minor: b.amount_minor };
    }
    if (path === '/v1/quotes' && method === 'POST') {
      if (this.scenario === 'restricted') throw new APIError(403, 'account_not_eligible', 'Synthetic account restriction: payments are unavailable.');
      const body = options.body as QuoteInput;
      const destination = body.kind === 'internal' ? { user_id: body.recipient_id, account_name: 'TUNDE ADEYEMI · SYNTHETIC' } : body.kind === 'bank' ? { bank_code: '999', account_name: s.beneficiaries.find(x => x.id === body.beneficiary_id)?.account_name || 'SYNTHETIC RECIPIENT', account_number: '0000000000' } : { account_name: 'SYNTHETIC BILL CUSTOMER', product_id: this.validations.get(body.validation_id || '')?.product.id, customer_id: this.validations.get(body.validation_id || '')?.customer };
      const fee = body.kind === 'internal' ? '0' : '100';
      const q: Quote = { id: id('quo'), kind: body.kind, amount_minor: body.amount_minor, fee_minor: fee, total_minor: (BigInt(body.amount_minor) + BigInt(fee)).toString(), currency: 'NGN', destination, narration: body.narration, policy_version: 1, expires_at: new Date(Date.now() + (this.scenario === 'expired' ? -1000 : 300000)).toISOString(), used: false };
      s.quotes[q.id] = q; return q;
    }
    if (/^\/v1\/quotes\/[^/]+$/.test(path)) { const q = s.quotes[path.split('/')[3]]; if (!q) throw new APIError(404, 'not_found', 'Quote not found.'); return q; }
    if (path.endsWith('/authorisations')) {
      if (b.pin !== reviewCredentials.pin && !s.user.pin_set) throw new APIError(401, 'unauthorized', 'Use the synthetic review PIN.');
      if (b.pin !== '123456') throw new APIError(401, 'unauthorized', 'The synthetic PIN is 123456.');
      const qid = path.split('/')[3]; const q = s.quotes[qid];
      if (!q || Date.parse(q.expires_at) <= Date.now()) throw new APIError(409, 'conflict', 'Quote expired. Request a new quote.');
      const a: Approval = { quote_id: qid, authorisation_token: id('approval'), expires_at: q.expires_at }; this.approvals.set(qid, a); return a;
    }
    if (path === '/v1/payments' && method === 'POST') {
      const key = options.idempotencyKey || ''; const existing = s.idempotency[key];
      if (existing) { const p = s.payments.find(x => x.id === existing)!; if (p.quote_id !== b.quote_id) throw new APIError(409, 'conflict', 'Key already bound.'); return p; }
      const q = s.quotes[b.quote_id]; if (!q || q.used || this.approvals.get(q.id)?.authorisation_token !== b.authorisation_token) throw new APIError(401, 'unauthorized', 'Authorise this quote first.');
      beforeReviewPayment(s, q);
      if (this.scenario === 'insufficient' || BigInt(q.total_minor) > BigInt(s.balance) - BigInt(s.held)) throw new APIError(409, 'insufficient_funds', 'Insufficient available balance.');
      const status = this.scenario === 'pending' ? 'pending' : this.scenario === 'failed' ? 'failed' : 'succeeded';
      const p: Payment = { id: id('pay'), quote_id: q.id, kind: q.kind, status, amount_minor: q.amount_minor, fee_minor: q.fee_minor, total_minor: q.total_minor, currency: 'NGN', direction: 'outgoing', fulfilment_status: q.kind === 'bill' ? status === 'succeeded' ? 'ready' : 'pending' : 'not_applicable', created_at: now(), updated_at: now() };
      q.used = true; s.payments.unshift(p); s.idempotency[key] = p.id;
      if (status === 'succeeded') { s.balance = (BigInt(s.balance) - BigInt(p.total_minor)).toString(); s.entries.unshift({ id: Date.now(), journal_id: id('jnl'), reference: p.id, kind: p.kind + '_payment', delta_minor: '-' + p.total_minor, created_at: now() }); }
      if (status === 'pending') s.held = (BigInt(s.held) + BigInt(p.total_minor)).toString();
      s.notices.unshift({ id: id('notice'), workflow: 'transfer-' + status, subject: q.kind === 'bill' ? 'Bill payment update' : 'Transfer update', reference: p.id, created_at: now(), read: false });
      afterReviewPayment(s, p); this.persist();
      if (this.scenario === 'unknown') throw new APIError(0, 'network_unknown', 'Synthetic response loss after the request was accepted. Check the original request.');
      return p;
    }
    if (path === '/v1/payments') {
      const before = url.searchParams.get('before'); const start = before ? s.payments.findIndex(p => p.id === before) + 1 : 0;
      return s.payments.slice(start, start + Number(url.searchParams.get('limit') || 50));
    }
    if (/^\/v1\/payments\//.test(path)) {
      const p = s.payments.find(x => x.id === path.split('/')[3]); if (!p) throw new APIError(404, 'not_found', 'Payment not found.');
      if (path.endsWith('/receipt')) { if (p.status !== 'succeeded') throw new APIError(409, 'conflict', 'A success receipt is not available for this status.'); return { payment: p, generated_at: now(), copy: true, environment: 'review' }; }
      if (path.endsWith('/fulfilment')) return { status: p.fulfilment_status, payment_status: p.status, ...(p.fulfilment_status === 'ready' ? { value: 'SYNTHETIC VALUE — NOT REDEEMABLE' } : {}) };
      return p;
    }
    if (path === '/v1/notifications') return { items: s.notices, unread: s.notices.filter(n => !n.read).length };
    if (path.endsWith('/read')) { const n = s.notices.find(n => n.id === path.split('/')[3]); if (n) n.read = true; return undefined; }
    if (path === '/v1/preferences') { if (method === 'PATCH') { s.preferences = options.body as Preferences; return { status: 'accepted' }; } return s.preferences; }
    if (path === '/v1/auth/sessions') return s.devices;
    if (path.startsWith('/v1/auth/sessions/') && method === 'DELETE') { const d = s.devices.find(d => d.id === path.split('/').pop()); if (d) { d.revoked = true; if (d.current) s.signedIn = false; } return undefined; }
    if (path === '/v1/auth/mfa/enrol') return { secret: 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ', otpauth_url: 'otpauth://totp/Qpay:Synthetic?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&issuer=Qpay' };
    if (path === '/v1/auth/mfa/confirm') { if (b.code !== '123456') throw new APIError(401, 'unauthorized', 'Use review code 123456.'); s.user.mfa_enabled = true; return Array.from({ length: 10 }, (_, i) => `SYNTHETIC-RECOVERY-${i + 1}`); }
    if (path === '/v1/auth/password' || path === '/v1/auth/pin') { if (path.endsWith('/pin')) s.user.pin_set = true; s.signedIn = false; return { status: 'accepted' }; }
    if (path === '/v1/kyc/cases') {
      if (method === 'POST') { const k = { id: id('kyc'), owner_id: s.user.id, status: 'submitted', evidence_reference: b.evidence_reference, created_at: now() }; s.kyc.unshift(k); return { id: k.id }; }
      return s.kyc;
    }
    if (path === '/v1/support/cases') {
      if (method === 'POST') { const c: SupportCase = { id: id('case'), owner_id: s.user.id, payment_id: b.payment_id || '', kind: b.kind, subject: b.subject, status: 'open', created_at: now(), updated_at: now() }; s.cases.unshift(c); s.messages[c.id] = [{ id: id('msg'), author_id: s.user.id, message: b.message, created_at: now() }]; return { id: c.id }; }
      return s.cases;
    }
    if (path.endsWith('/messages')) { const cid = path.split('/')[4]; if (!s.messages[cid]) throw new APIError(404, 'not_found', 'Case not found.'); if (method === 'POST') { s.messages[cid].push({ id: id('msg'), author_id: s.user.id, message: b.message, created_at: now() }); return { status: 'accepted' }; } return s.messages[cid]; }
    if (path === '/v1/statements') {
      const from = url.searchParams.get('from')!, to = url.searchParams.get('to')!;
      const entries = s.entries.filter(e => e.created_at >= from && e.created_at < to);
      const opening = s.entries.filter(e => e.created_at < from).reduce((sum, e) => sum + BigInt(e.delta_minor), 0n);
      const statement: Statement = { currency: 'NGN', from, to_exclusive: to, opening_minor: opening.toString(), closing_minor: entries.reduce((sum, e) => sum + BigInt(e.delta_minor), opening).toString(), entries, generated_at: now() };
      if (options.raw) return new Blob(['date,reference,delta_minor,currency\n' + entries.map(e => `${e.created_at},${e.reference},${e.delta_minor},NGN`).join('\n')], { type: 'text/csv' });
      return statement;
    }
    throw new APIError(404, 'not_implemented', 'This workflow is not implemented in the review adapter. No action was performed.');
  }
}
