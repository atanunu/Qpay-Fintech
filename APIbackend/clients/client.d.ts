export type MinorUnits = string;
export type PaymentStatus = 'accepted'|'submitted'|'pending'|'succeeded'|'failed'|'pending_review';
export interface APIUser { id:string; email:string; name:string; role:'customer'|'admin'|'finance'|'compliance'|'support'|'platform'; status:'active'|'restricted'|'closed'; email_verified:boolean; tier:number; mfa_enabled:boolean; pin_set:boolean; version:number; created_at:string; }
export interface Balance { currency:'NGN'; available_minor:MinorUnits; held_minor:MinorUnits; balance_minor:MinorUnits; }
export interface Destination { user_id?:string; bank_code?:string; account_number?:string; account_name?:string; product_id?:string; customer_id?:string; validation_id?:string; }
export interface QuoteInput {kind:'internal'|'bank'|'bill';amount_minor:MinorUnits;currency:'NGN';narration:string;recipient_id?:string;beneficiary_id?:string;validation_id?:string;}
export interface Quote {id:string;kind:QuoteInput['kind'];amount_minor:MinorUnits;fee_minor:MinorUnits;total_minor:MinorUnits;currency:'NGN';destination:Destination;narration:string;policy_version:number;expires_at:string;used:boolean;}
export interface Payment {id:string;quote_id:string;kind:QuoteInput['kind'];direction:'outgoing'|'incoming';status:PaymentStatus;amount_minor:MinorUnits;fee_minor:MinorUnits;total_minor:MinorUnits;currency:'NGN';fulfilment_status:'not_applicable'|'pending'|'ready';provider_reference?:string;created_at:string;updated_at:string;}
export interface Authorisation {authorisation_token:string;quote_id:string;expires_at:string;}
export interface Session {access_token?:string;refresh_token?:string;csrf_token?:string;expires_at:string;refresh_expires_at:string;mfa_enrolment_required:boolean;user:APIUser;}
export interface RequestOptions {body?:unknown;idempotencyKey?:string;signal?:AbortSignal;}
export class APIError extends Error {status:number;code:string;requestId:string;constructor(status:number,code:string,message:string,requestId?:string);}
export function minor(value:string):MinorUnits;
export class QpayClient {
 constructor(options:{baseURL:string;transport:'web'|'mobile';fetchImpl?:typeof fetch;getAccessToken?:()=>string|Promise<string>;getCSRFToken?:()=>string|Promise<string>;allowLocalHTTP?:boolean});
 request<T=unknown>(method:'GET'|'POST'|'PATCH'|'DELETE',path:string,options?:RequestOptions):Promise<T>;
 quote(input:QuoteInput,options?:RequestOptions):Promise<Quote>;
 authorise(quoteId:string,pin:string,mfaCode?:string,options?:RequestOptions):Promise<Authorisation>;
 pay(quoteId:string,token:string,idempotencyKey:string,options?:RequestOptions):Promise<Payment>;
}
