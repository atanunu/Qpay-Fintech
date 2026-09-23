import type { Money } from './types';
export interface Controls { handle: string; discoverable: boolean; frozen: boolean; per_payment_minor: Money; daily_minor: Money; daily_remaining_minor: Money }
export interface CustomerCapabilities { version: number; currency: string; tier: number; account_status: string; email_verified: boolean; controls: Controls; features: Record<string,string>; checked_at: string; synthetic: boolean; limit_day_timezone: string }
export interface SavedBill { id: string; label: string; product_id: string; customer_id: string; amount_minor: Money; currency: string; favourite: boolean }
export interface Reminder { id: string; title: string; amount_minor: Money; bill_id: string; due_at: string; ends_at?: string; cadence: string; status: string; version: number }
export interface Mandate { id: string; title: string; cadence: string; next_at: string; ends_at: string; max_debit_minor: Money; max_total_minor: Money; max_occurrences: number; occurrences: number; committed_total_minor: Money; status: string; version: number }
export interface MoneyRequest { id: string; owner_id: string; requester_name: string; memo: string; amount_minor: Money; received_minor: Money; currency: string; expires_at: string; status: string; direction: string; created_at: string }
export interface Share { id: string; payer_id: string; name: string; amount_minor: Money; received_minor: Money; status: string }
export interface RequestDetail { id: string; owner_id: string; requester_name: string; memo: string; amount_minor: Money; currency: string; expires_at: string; status: string; shares: Share[] }
export interface Insights { month: string; currency: string; timezone: string; money_in_minor: Money; money_out_minor: Money; spending_minor: Money; excluded_minor: Money; fees_minor: Money; previous_money_out_minor: Money; next_30_days_reminders_minor: Money; categories: {category:string;spent_minor:Money;budget_minor:Money}[]; estimate_note:string }
export interface Upload { id:string;name:string;purpose:string;case_id:string;mime:string;size:number;state:string;created_at:string }
export interface Identity {legal_name:string;date_of_birth:string;address:string;document_type:string;upload_ids:string[];consent:boolean;version:number}
export interface Draft {version:number;draft:Identity|null;submitted_case?:string}
export interface FundingAccount {id?:string;state:string;configured:boolean;updated_at?:string;details?:{reference:string;state:string;account_name:string;account_number:string;institution:string;synthetic:boolean}}
export const categories=['transfers','airtime','data','electricity','television','internet','groceries','transport','housing','education','health','entertainment','other'];
export function localMonth(date=new Date()):string { return new Intl.DateTimeFormat('en-CA',{timeZone:'Africa/Lagos',year:'numeric',month:'2-digit'}).format(date).slice(0,7); }
/** Input datetime-local values are explicitly WAT, independent of the device zone. */
export function watISO(value:string):string {const date=new Date(value+':00+01:00');if(!Number.isFinite(date.getTime()))throw new Error('Choose a valid West Africa Time date.');return date.toISOString();}
export function watInput(date=new Date(Date.now()+86400000)):string{return new Date(date.getTime()+3600000).toISOString().slice(0,16);}
export function splitMinor(total:string,count:number):string[]{if(!Number.isInteger(count)||count<1||count>20)throw new Error('Choose 1 to 20 people.');const t=BigInt(total);if(t<BigInt(count))throw new Error('Every share must be at least one kobo.');const base=t/BigInt(count),extra=Number(t%BigInt(count));return Array.from({length:count},(_,i)=>(base+BigInt(i<extra?1:0)).toString());}
