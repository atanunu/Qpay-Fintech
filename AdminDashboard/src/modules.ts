export interface Module {id:string;title:string;group:string;description:string;columns:string[]}
export const modules:Module[]=[
 ['customers','Customers','Customers','Identity, account status and security controls. Contact details are masked by the API.','name,email,status,tier,created_at'],
 ['kyc','Identity review','Customers','Review private evidence and submit independently approved verification decisions.','owner_id,name,status,tier,document_count'],
 ['funding','Funding accounts','Customers','Account provisioning and funding evidence. Active is not proof of partner acceptance.','owner_id,provider,status,updated_at'],
 ['privacy','Privacy & closure','Customers','Account closure records. Retention and financial obligations remain separate.','owner_id,status,created_at'],
 ['payments','Transactions','Money movement','Inspect original payment intents, observations, holds and journal references. Never submit a replacement payout.','kind,owner_id,status,amount_minor,fee_minor,fulfilment_status,created_at'],
 ['jobs','Payment recovery','Money movement','Durable jobs and original-reference recovery. Re-query is permitted only for eligible dead jobs.','object_id,status,attempts,last_error,available_at'],
 ['schedules','Scheduled payments','Money movement','Inspect authorised mandates and every occurrence. Staff may pause, not silently create or resume customer mandates.','title,owner_id,status,cadence,next_at,occurrences'],
 ['requests','Money requests','Money movement','Requests and their participant shares. Staff cannot approve a customer debit.','owner_id,memo,status,amount_minor,expires_at'],
 ['reminders','Reminders','Money movement','Customer reminders are notifications, not payment authorisations.','title,owner_id,status,cadence,due_at'],
 ['products','Bill products','Products','Versioned product controls require independent approval. Upstream price/catalogue ownership remains with QPay.','id,enabled,version,updated_at'],
 ['availability','Service availability','Products','Observed availability expires after five minutes; it is not a guarantee of transaction success.','id,status,observed_at,version'],
 ['ledger','Ledger journals','Finance','Append-only, balanced journal entries. Open a journal to inspect its postings. No balance-edit controls exist.','reference,kind,currency,created_at'],
 ['treasury','Treasury & balances','Finance','Wallet liabilities, reservations, clearing and fee accounts. Clearing must not be described as verified bank float.','id,kind,currency,balance_minor,held_minor,available_minor'],
 ['reconciliations','Reconciliation','Finance','Import and compare bounded provider/bank records without changing money. Completeness requires separate settlement evidence.','source,row_count,exceptions,created_at'],
 ['exceptions','Reconciliation breaks','Finance','Unmatched imported records remain visible. Open a linked investigation with evidence and ownership.','reference,amount_minor,status,reason,created_at'],
 ['investigations','Refunds & investigations','Finance','Evidence, assignment and investigation lifecycle. Resolving a case never executes or certifies a refund or return.','kind,subject,status,severity,assigned_to,due_at'],
 ['support','Support & complaints','Engagement','Customer conversations, private attachments, assignment, escalation and response deadlines.','subject,kind,status,assigned_to,response_due_at'],
 ['notifications','Notification delivery','Engagement','Redacted non-secret intents and delivery observations. OTPs, recipients and message payloads are not exposed.','workflow,owner_id,status,attempts,created_at'],
 ['risk','Risk & compliance','Governance','Reasoned investigation records and case ownership; not an automated sanctions or regulatory filing service.','subject,target,severity,status,assigned_to,due_at'],
 ['incidents','Incident management','Governance','Record incidents and containment decisions. Emergency stop blocks new acceptance, not recovery of existing obligations.','subject,severity,status,assigned_to,created_at'],
 ['proposals','Financial approvals','Governance','Review immutable details, expiry and target version. Maker and checker must be different eligible operators.','action,target,maker_id,status,expires_at'],
 ['controls','Operational approvals','Governance','Independent staff, product and service-resumption controls. Stale versions and self-approval are rejected.','kind,target,value,maker_id,status,expires_at'],
 ['audit','Audit trail','Governance','Immutable, searchable staff/system events. Viewing and exporting this data creates its own audit event.','actor,action,target,created_at'],
 ['exports','Export register','Governance','Who exported which filtered records, why, and how many. Export content is not retained in the browser.','actor_id,resource,reason,row_count,created_at'],
 ['staff','Staff & access','Administration','Role changes and suspension require independent review. No impersonation or provider-secret reveal.','name,email,role,status,mfa_enabled,version'],
 ['invitations','Staff invitations','Administration','Propose, independently approve, hand over once, accept, enrol MFA, or revoke.','email,name,role,status,maker_id,expires_at'],
].map(([id,title,group,description,columns])=>({id,title,group,description,columns:columns.split(',')}));
export const moduleById=(id:string)=>modules.find(m=>m.id===id);
export function label(key:string):string{return ({id:'Reference',amount_minor:'Amount',fee_minor:'Fee',balance_minor:'Book balance',held_minor:'Held funds',available_minor:'Available',mfa_enabled:'MFA enrolled',owner_id:'Customer',assigned_to:'Assignee',row_count:'Imported rows',document_count:'Documents',target_version:'Expected version'} as Record<string,string>)[key]||key.replace(/_/g,' ').replace(/^./,s=>s.toUpperCase())}
export function money(value:unknown):string{if(typeof value!=='string'||! /^-?\d+$/.test(value))return '—';const n=BigInt(value);const a=n<0n?-n:n;return `${n<0n?'−':''}₦${(a/100n).toLocaleString('en-NG')}.${(a%100n).toString().padStart(2,'0')}`}
export function format(key:string,value:unknown):string{if(value===null||value===undefined||value==='')return '—';if(key.endsWith('_minor'))return money(value);if(typeof value==='boolean')return value?'Yes':'No';if(typeof value==='object')return JSON.stringify(value,null,2);if(typeof value==='string'&&/(?:_at|expires_at|due_at)$/.test(key)&&!Number.isNaN(Date.parse(value)))return new Date(value).toLocaleString('en-GB',{dateStyle:'medium',timeStyle:'short'});return String(value)}
