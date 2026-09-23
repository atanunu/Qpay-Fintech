import { describe, expect, it, vi } from 'vitest';
import { APIError, HttpTransport, safeBaseURL, validateCapabilities, validatePayment, validateQuote, validateUser, validateWallet } from '../src/api/client';
import { amountInput, MAX_MINOR, minor, money, parseAmount, statementRange } from '../src/api/money';
import { clearIntent, findOriginal, loadIntent, saveIntent } from '../src/api/recovery';
import type { Intent } from '../src/api/recovery';
import type { Transport, User } from '../src/api/types';
import { integrationGaps, acceptanceJourneys } from '../src/api/integration';
class MemoryStorage implements Storage {
  values = new Map<string,string>(); get length() { return this.values.size; }
  clear(){this.values.clear();} getItem(k:string){return this.values.get(k)??null;} key(i:number){return [...this.values.keys()][i]??null;}
  removeItem(k:string){this.values.delete(k);} setItem(k:string,v:string){this.values.set(k,String(v));}
}
const user: User = { id:'usr_test_customer', email:'customer@example.invalid', name:'Synthetic Customer', role:'customer', status:'active', email_verified:true, tier:1, mfa_enabled:false, pin_set:true,version:1,created_at:new Date().toISOString() };
const quote = {id:'quo_test_original', kind:'bank', amount_minor:'10000',fee_minor:'100',total_minor:'10100',currency:'NGN',destination:{account_name:'Synthetic recipient'},narration:'Test',policy_version:1,used:false,expires_at:'2026-09-23T12:00:00Z'};
const payment = {id:'pay_test_original',quote_id:quote.id,kind:'bank',status:'pending',amount_minor:'10000',fee_minor:'100',total_minor:'10100',currency:'NGN',direction:'outgoing',fulfilment_status:'not_applicable',created_at:'2026-09-23T12:00:00Z',updated_at:'2026-09-23T12:00:00Z'};
const response = (data:unknown,status=200) => new Response(JSON.stringify(status<400?{data,request_id:'req_test'}:{error:data,request_id:'req_test'}),{status,headers:{'Content-Type':'application/json'}});

describe('exact money',()=>{
  it.each(['1.2','1e3','-1','01','NaN','Infinity','9000000000000001',1,null])('rejects invalid minor units %s',v=>expect(()=>minor(v)).toThrow());
  it.each([['1','100'],['1000.01','100001'],['0.01','1'],['90000000000000.00',MAX_MINOR.toString()]])('converts %s without floats',(a,b)=>expect(parseAmount(a)).toBe(b));
  it.each(['0','-2','1,000','01.00','1.001','1e2','90000000000000.01',''])('rejects unsafe displayed amount %s',v=>expect(()=>parseAmount(v)).toThrow());
  it('round-trips the largest supported amount',()=>expect(parseAmount(amountInput(MAX_MINOR.toString()))).toBe(MAX_MINOR.toString()));
  it('formats exact kobo and a signed journal delta',()=>{expect(money('101')).toBe('₦1.01');expect(money('-101',true)).toBe('−₦1.01');});
  it('projects calendar days to WAT and clips today to the current instant',()=>{expect(statementRange('2026-09-01','2026-09-02',new Date('2026-09-23T12:00:00Z'))).toEqual({from:'2026-08-31T23:00:00.000Z',to:'2026-09-02T23:00:00.000Z'});expect(statementRange('2026-09-23','2026-09-23',new Date('2026-09-23T12:00:00Z')).to).toBe('2026-09-23T12:00:00.000Z');});
  it('rejects impossible, reversed, future and excessive date ranges',()=>{for(const [f,t] of [['2026-02-30','2026-03-01'],['2026-09-20','2026-09-01'],['2027-01-01','2027-01-02'],['2024-01-01','2026-09-23']])expect(()=>statementRange(f,t,new Date('2026-09-23T12:00:00Z'))).toThrow();});
});
describe('API trust boundary',()=>{
  it('validates wallet arithmetic before display',()=>{expect(validateWallet({currency:'NGN',balance_minor:'101',available_minor:'100',held_minor:'1'}).balance_minor).toBe('101');expect(()=>validateWallet({currency:'NGN',balance_minor:'101',available_minor:'101',held_minor:'1'})).toThrow();});
  it('rejects staff identities in the customer application',()=>{expect(validateUser(user).id).toBe(user.id);expect(()=>validateUser({...user,role:'admin'})).toThrow();});
  it('rejects float and mismatched quote totals',()=>{expect(validateQuote(quote).id).toBe(quote.id);expect(()=>validateQuote({...quote,fee_minor:100})).toThrow();expect(()=>validateQuote({...quote,total_minor:'10001'})).toThrow();});
  it('rejects unrecognised or incomplete payment states',()=>{expect(validatePayment(payment).status).toBe('pending');expect(()=>validatePayment({...payment,status:'timeout_success'})).toThrow();expect(()=>validatePayment({...payment,fulfilment_status:'delivered_maybe'})).toThrow();});
  it('requires explicit typed capabilities',()=>expect(()=>validateCapabilities({environment:'production',external_payments_configured:'yes'})).toThrow());
  it.each(['http://api.example.invalid','https://user:pass@api.example.invalid','https://api.example.invalid/path','https://api.example.invalid?token=test','https://api.example.invalid#fragment'])('rejects unsafe API origin %s',v=>expect(()=>safeBaseURL(v,false)).toThrow());
  it('permits local HTTP only when explicitly enabled',()=>{expect(safeBaseURL('http://localhost:8080',true)).toBe('http://localhost:8080');expect(()=>safeBaseURL('http://remote.invalid',true)).toThrow();});
  it('uses cookies plus CSRF and never a browser bearer token',async()=>{
    const fetcher=vi.fn(async (_:RequestInfo|URL,init?:RequestInit)=>{expect(init?.credentials).toBe('include');expect(init?.cache).toBe('no-store');expect((init?.headers as Record<string,string>).Authorization).toBeUndefined();expect((init?.headers as Record<string,string>)['X-CSRF-Token']).toBe('csrf-test');return response({status:'accepted'});});
    const api=new HttpTransport('https://api.example.invalid',fetcher as typeof fetch);api.csrf='csrf-test';await api.request('PATCH','/v1/preferences',{body:{optional_email:true,marketing_email:false}});expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it('never retries a financial write after a transport loss',async()=>{const fetcher=vi.fn(async()=>{throw new TypeError('Network failure');});const api=new HttpTransport('https://api.example.invalid',fetcher);api.csrf='csrf';await expect(api.request('POST','/v1/payments',{body:{quote_id:quote.id,authorisation_token:'temporary'},idempotencyKey:'persisted-test-key'})).rejects.toMatchObject({code:'network_unknown'});expect(fetcher).toHaveBeenCalledTimes(1);});
  it('never automatically replays a write after 401',async()=>{const fetcher=vi.fn(async()=>response({code:'unauthorized',message:'Session expired'},401));const api=new HttpTransport('https://api.example.invalid',fetcher);api.csrf='csrf';await expect(api.request('POST','/v1/payments',{body:{}})).rejects.toMatchObject({status:401});expect(fetcher).toHaveBeenCalledTimes(1);});
  it('preserves request reference on a structured failure',async()=>{const api=new HttpTransport('https://api.example.invalid',async()=>response({code:'conflict',message:'Quote changed'},409));await expect(api.request('GET','/v1/me')).rejects.toMatchObject({code:'conflict',requestId:'req_test'});});
  it('rejects browser login responses containing mobile tokens',async()=>{const api=new HttpTransport('https://api.example.invalid',async()=>response({user,csrf_token:'test',access_token:'should-not-be-here'}));await expect(api.request('POST','/v1/auth/login',{body:{}})).rejects.toMatchObject({code:'unsafe_session'});});
  it('coordinates concurrent expired-session recovery',async()=>{
    let refreshed=false,refreshes=0; const calls:string[]=[];
    const api=new HttpTransport('https://api.example.invalid',async (url,init)=>{const p=String(url);calls.push(p);if(p.endsWith('/csrf'))return response({csrf_token:'new-csrf'});if(p.endsWith('/refresh')){refreshes++;refreshed=true;return response({user,csrf_token:'next-csrf'});}if(p.endsWith('/auth/me'))return refreshed?response(user):response({code:'unauthorized',message:'expired'},401);return refreshed?response({currency:'NGN',balance_minor:'100',available_minor:'100',held_minor:'0'}):response({code:'unauthorized',message:'expired'},401);});
    const values=await Promise.all([api.request('GET','/v1/wallet'),api.request('GET','/v1/wallet')]);expect(values).toHaveLength(2);expect(refreshes).toBe(1);expect(api.csrf).toBe('next-csrf');
  });
  it('does not call any API for a non-relative application path',async()=>{const fetcher=vi.fn();const api=new HttpTransport('https://api.example.invalid',fetcher);await expect(api.request('GET','//evil.invalid')).rejects.toThrow();expect(fetcher).not.toHaveBeenCalled();});
});
describe('payment recovery',()=>{
  const intent:Intent={version:1,owner:user.id,quoteId:quote.id,idempotencyKey:'same-original-key',createdAt:new Date().toISOString(),phase:'unknown'};
  it('persists references only and isolates another signed-in owner',()=>{const storage=new MemoryStorage();saveIntent(intent,storage);expect(loadIntent(user.id,storage)).toEqual(intent);expect(loadIntent('usr_other',storage)).toBeNull();expect(storage.getItem('qpf.payment-recovery.v1')).not.toContain('authorisation_token');clearIntent(storage);expect(loadIntent(user.id,storage)).toBeNull();});
  it('refuses to hide a persistence failure before submission',()=>{const storage=new MemoryStorage();storage.setItem=()=>{throw Error('storage denied');};expect(()=>saveIntent(intent,storage)).toThrow();});
  it('finds an original request through owner-scoped lookup',async()=>{const request=vi.fn(async(method,path)=>{expect(method).toBe('GET');expect(path).toContain('/v1/payments/lookup?');return {found:true,payment};});expect(await findOriginal({request,csrf:'',clear() {}} as Transport,intent)).toEqual(payment);expect(request).toHaveBeenCalledTimes(1);});
  it('returns not-found without creating a replacement payment',async()=>{const request=vi.fn(async()=>({found:false,conclusive_failure:false}));expect(await findOriginal({request,csrf:'',clear(){}} as Transport,intent)).toBeNull();expect(request).toHaveBeenCalledTimes(1);});
  it('queries a saved payment ID instead of searching or resubmitting',async()=>{const request=vi.fn(async()=>payment);await findOriginal({request,csrf:'',clear(){}} as Transport,{...intent,paymentId:payment.id});expect(request).toHaveBeenCalledWith('GET','/v1/payments/'+payment.id);});
  it('keeps unique actionable gap and UAT identifiers',()=>{expect(new Set(integrationGaps.map(x=>x.id)).size).toBe(integrationGaps.length);expect(new Set(acceptanceJourneys.map(x=>x[0])).size).toBe(acceptanceJourneys.length);expect(integrationGaps).toHaveLength(14);});
});
