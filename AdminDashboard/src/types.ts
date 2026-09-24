export type Json = string|number|boolean|null|Json[]|{[key:string]:Json};
export type Row=Record<string,Json>;
export type Role='admin'|'finance'|'compliance'|'support'|'platform'|'auditor';
export interface User {id:string;email:string;name:string;role:Role;status:string;mfa_enabled:boolean;version:number}
export interface Session {user:User;csrf_token?:string;mfa_enrolment_required:boolean}
export interface Bootstrap {user:User;resources:string[];actions:string[];environment:string;synthetic_execution:boolean;server_time:string;external_execution_configured:boolean;notification_mode:string;schedules_configured:boolean;private_storage_configured:boolean;scanner_configured:boolean;live_acceptance:boolean;gates:string[]}
export interface Page {items:Row[];has_more:boolean;next_before:string;generated_at:string}
export interface Detail {record:Row;notes:Row[];[key:string]:Row|Row[]}
export interface LoginInput {email:string;password:string;mfa_code?:string;recovery_code?:string;client:'web';device:string}
export interface Transport {request<T=Row>(method:string,path:string,body?:unknown):Promise<T>;blob(path:string,body:unknown):Promise<Blob>;restore():Promise<User|null>;login(input:LoginInput):Promise<Session>;logout():Promise<void>}
declare global {const __ADMIN_REVIEW__:boolean}
