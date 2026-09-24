import type {LoginInput,Session,Transport,User} from './types';
import { APIError } from './errors';
export { APIError } from './errors';
export function apiOrigin(value:string,allowLocal:boolean):string {
 const url=new URL(value);const local=['localhost','127.0.0.1','[::1]'].includes(url.hostname);
 if(url.username||url.password||url.search||url.hash||url.pathname!=='/'||(url.protocol!=='https:'&&!(allowLocal&&local&&url.protocol==='http:')))throw new Error('Use an approved HTTPS API origin');return url.origin;
}
export class AdminAPI implements Transport {
 private csrf='';private refresh:Promise<void>|null=null;private origin:string;private fetcher:typeof fetch;
 constructor(base:string,local=false,fetcher:typeof fetch=globalThis.fetch.bind(globalThis)){this.origin=apiOrigin(base,local);this.fetcher=fetcher}
 private async send(method:string,path:string,body?:unknown):Promise<Response>{
  if(!['GET','POST','PATCH','DELETE'].includes(method)||!path.startsWith('/v1/admin/')||path.includes('..')||path.includes('\\')||path.includes('#'))throw new Error('Only relative staff API paths are allowed');
  const headers:Record<string,string>={Accept:'application/json'};if(body!==undefined)headers['Content-Type']='application/json';if(method!=='GET'&&this.csrf)headers['X-CSRF-Token']=this.csrf;
  // No automatic mutation retries. The user must inspect current state after ambiguity.
  const res=await this.fetcher(this.origin+path,{method,headers,body:body===undefined?undefined:JSON.stringify(body),credentials:'include',redirect:'error',cache:'no-store',signal:AbortSignal.timeout(20000)});
  if(!res.ok){let data;try{data=await res.json()}catch{throw new APIError(res.status,'invalid_response','API returned an unreadable error')};throw new APIError(res.status,data.error?.code||'request_failed',data.error?.message||'Operation failed',data.request_id||'')};return res;
 }
 async request<T>(method:string,path:string,body?:unknown):Promise<T>{const res=await this.send(method,path,body);if(res.status===204)return undefined as T;const result=await res.json();if(result?.data===undefined)throw new APIError(res.status,'invalid_response','API response did not contain data');return result.data as T}
 async blob(path:string,body:unknown):Promise<Blob>{return (await this.send('POST',path,body)).blob()}
 async login(input:LoginInput):Promise<Session>{const s=await this.request<Session>('POST','/v1/admin/auth/login',input);this.csrf=s.csrf_token||'';return s}
 async restore():Promise<User|null>{
  try{const csrf=await this.request<{csrf_token:string}>('GET','/v1/admin/auth/csrf');this.csrf=csrf.csrf_token;
   try{return await this.request<User>('GET','/v1/admin/auth/me')}catch(e){if(!(e instanceof APIError)||e.status!==401)throw e}
   if(!this.refresh)this.refresh=this.request<Session>('POST','/v1/admin/auth/refresh',{client:'web'}).then(s=>{this.csrf=s.csrf_token||''}).finally(()=>{this.refresh=null});await this.refresh;return await this.request<User>('GET','/v1/admin/auth/me');
  }catch(e){if(e instanceof APIError&&[401,403].includes(e.status)){this.csrf='';return null}throw e}
 }
 async logout():Promise<void>{try{await this.request('POST','/v1/admin/auth/logout')}finally{this.csrf=''}}
}
// Explicit API origin required. The invalid default fails closed without sending credentials.
export const api=new AdminAPI(import.meta.env.VITE_API_BASE_URL||'https://api.example.invalid',import.meta.env.VITE_ALLOW_LOCAL_HTTP==='true');
