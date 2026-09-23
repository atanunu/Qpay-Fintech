export type Money = string;
export type Kind = 'internal' | 'bank' | 'bill';
export type PaymentStatus = 'accepted' | 'submitted' | 'pending' | 'succeeded' | 'failed' | 'pending_review';
export interface User { id: string; email: string; name: string; role: string; status: 'active' | 'restricted' | 'closed'; email_verified: boolean; tier: number; mfa_enabled: boolean; pin_set: boolean; version: number; created_at: string }
export interface Session { csrf_token: string; expires_at: string; refresh_expires_at: string; user: User; mfa_enrolment_required: boolean }
export interface Capabilities { environment: string; currency: string; external_payments_configured: boolean; notifications: string; synthetic_execution: boolean; growth_products_enabled: boolean; production_acceptance: boolean }
export interface Wallet { currency: 'NGN'; available_minor: Money; held_minor: Money; balance_minor: Money }
export interface Destination { user_id?: string; bank_code?: string; account_number?: string; account_name?: string; product_id?: string; customer_id?: string; validation_id?: string }
export interface QuoteInput { kind: Kind; amount_minor: Money; currency: 'NGN'; narration: string; recipient_id?: string; beneficiary_id?: string; enquiry_id?: string; validation_id?: string }
export interface Quote { id: string; kind: Kind; amount_minor: Money; fee_minor: Money; total_minor: Money; currency: 'NGN'; destination: Destination; narration: string; policy_version: number; expires_at: string; used: boolean }
export interface Approval { quote_id: string; authorisation_token: string; expires_at: string }
export interface Payment { id: string; quote_id: string; kind: Kind; status: PaymentStatus; amount_minor: Money; fee_minor: Money; total_minor: Money; currency: 'NGN'; fulfilment_status: 'not_applicable' | 'pending' | 'ready'; direction: 'incoming' | 'outgoing'; created_at: string; updated_at: string; provider_reference?: string }
export interface Bank { code: string; name: string }
export interface Product { id: string; name: string; category: string; amount_minor: Money; variable_amount: boolean }
export interface Enquiry { id: string; name: string; expires_at: string; amount_minor: Money }
export interface Beneficiary { id: string; label: string; account_name: string; favourite?: boolean }
export interface Notice { id: string; workflow: string; subject: string; reference: string; created_at: string; read: boolean }
export interface NoticeList { items: Notice[]; unread: number }
export interface Preferences { optional_email: boolean; marketing_email: boolean }
export interface Device { id: string; device: string; client: string; created_at: string; expires_at: string; revoked: boolean; current: boolean }
export interface KycCase { id: string; owner_id: string; status: string; evidence_reference: string; created_at: string }
export interface SupportCase { id: string; owner_id: string; payment_id: string; kind: string; subject: string; status: string; created_at: string; updated_at: string }
export interface Message { id: string; author_id: string; message: string; created_at: string }
export interface Entry { id: number; journal_id: string; reference: string; kind: string; delta_minor: string; created_at: string }
export interface Statement { currency: 'NGN'; from: string; to_exclusive: string; opening_minor: string; closing_minor: string; entries: Entry[]; generated_at: string }
export interface Funding { id: string; amount_minor: Money; currency: 'NGN'; created_at: string }
export interface Fulfilment { status: string; payment_status: PaymentStatus; value?: string }
export interface Receipt { payment: Payment; generated_at: string; copy: boolean; environment: string }
export type Method = 'GET' | 'POST' | 'PATCH' | 'DELETE';
export interface RequestOptions { body?: unknown; idempotencyKey?: string; signal?: AbortSignal; raw?: boolean }
export interface Transport { request<T>(method: Method, path: string, options?: RequestOptions): Promise<T>; csrf: string; clear(): void }
