import {describe,it,expect,vi} from 'vitest';
import {HttpTransport} from '../src/api/client';
import {paymentEligibility} from '../src/api/parity';
describe('browser delivery regressions',()=>{
 it('calls default fetch on the host global rather than the transport instance',async()=>{
  const fetcher=vi.fn(function(this:unknown){expect(this).toBe(globalThis);return Promise.resolve(new Response(JSON.stringify({data:{ok:true}}),{status:200}));});
  vi.stubGlobal('fetch',fetcher);
  try{const client=new HttpTransport('https://api.example.invalid');expect(await client.request('GET','/v1/delivery-probe')).toEqual({ok:true});expect(fetcher).toHaveBeenCalledTimes(1);}finally{vi.unstubAllGlobals();}
 });
 it.each([null,undefined,{}, {transfers:'eligible'}, {payments:42}, {payments:'unknown'}])('does not infer eligibility from invalid capability evidence %s',value=>expect(paymentEligibility(value)).toBe('unavailable'));
 it('uses the backend payments capability rather than the absent transfers field',()=>{expect(paymentEligibility({payments:'verification_required'})).toBe('verification required');expect(paymentEligibility({payments:'eligible'})).toBe('eligible');expect(paymentEligibility({payments:'restricted'})).toBe('restricted');});
});
