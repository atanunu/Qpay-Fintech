import { useEffect, useRef } from 'react';
import type { ButtonHTMLAttributes, FormEvent, ReactNode } from 'react';
import { ArrowRight, CheckCircle2, Copy, Info, LoaderCircle, ShieldCheck, TriangleAlert, X } from 'lucide-react';
import { Link } from 'react-router-dom';
import { APIError, message } from '../api/client';
import { useApp } from '../state';

export function Button({ children, busy, variant = 'primary', className = '', ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { busy?: boolean; variant?: string }) {
  return <button type="button" {...props} disabled={props.disabled || busy} aria-busy={busy} className={`button ${variant} ${className}`}>{busy && <LoaderCircle size={17} className="spin"/>}{children}</button>;
}
export function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) { return <label className="field"><span className="field-label">{label}</span>{children}{hint && <span className="field-hint">{hint}</span>}</label>; }
export function Form({ children, onSubmit, className = '' }: { children: ReactNode; onSubmit: () => void; className?: string }) { return <form className={`form ${className}`} onSubmit={(e: FormEvent) => { e.preventDefault(); onSubmit(); }}>{children}</form>; }
export function ErrorBox({ error, retry }: { error: unknown; retry?: () => void }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => { if (error) ref.current?.focus(); }, [error]);
  if (!error) return null;
  return <div className="alert error" role="alert" tabIndex={-1} ref={ref}><TriangleAlert size={20}/><div><strong>We couldn’t complete that request</strong><p>{message(error)}</p>{error instanceof APIError && error.requestId && <small>Support reference: {error.requestId}</small>}{retry && <Button variant="quiet" onClick={retry}>Try again <ArrowRight size={15}/></Button>}</div></div>;
}
export function Callout({ title, children, tone = 'info' }: { title: string; children?: ReactNode; tone?: string }) {
  return <div className={`alert ${tone}`}><Info size={20}/><div><strong>{title}</strong>{children && <div className="alert-copy">{children}</div>}</div></div>;
}
export function Gap({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return <div className="gap"><span className="eyebrow"><span className="dot amber"/> UI ready · API connection pending</span><h3>{title}</h3><p>{children}</p><Link to={`/review?gap=${id}`} className="text-link">Review integration note · {id}<ArrowRight size={15}/></Link></div>;
}
export function Card({ title, action, children, className = '' }: { title?: string; action?: ReactNode; children: ReactNode; className?: string }) { return <section className={`card ${className}`}>{(title || action) && <div className="card-head"><h2>{title}</h2>{action}</div>}{children}</section>; }
export function PageTitle({ eyebrow = 'YOUR ACCOUNT', title, description, action }: { eyebrow?: string; title: string; description?: string; action?: ReactNode }) { return <div className="page-title"><div><span className="eyebrow">{eyebrow}</span><h1 tabIndex={-1}>{title}</h1>{description && <p>{description}</p>}</div>{action}</div>; }
export function Loading({ label = 'Loading your account…' }: { label?: string }) { return <div className="loading" role="status"><LoaderCircle size={23} className="spin"/><span>{label}</span></div>; }
export function Empty({ title, children, action }: { title: string; children?: ReactNode; action?: ReactNode }) { return <div className="empty"><span className="empty-icon"><Info size={25}/></span><h3>{title}</h3><p>{children}</p>{action}</div>; }
export function Badge({ status }: { status: string }) { const labels: Record<string, string> = { succeeded: 'Completed', pending_review: 'Under review', submitted: 'Processing', accepted: 'Request received', failed: 'Unsuccessful', ready: 'Value ready', approved: 'Approved', information_required: 'Action needed' }; return <span className={`badge ${status}`}><span className="dot"/>{labels[status] || status.replaceAll('_', ' ')}</span>; }
export function Modal({ title, children, onClose }: { title: string; children: ReactNode; onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => { const d = ref.current!; if (!d.open) d.showModal(); return () => { if (d.open) d.close(); }; }, []);
  return <dialog ref={ref} className="modal" aria-labelledby="modal-title" onCancel={onClose}><div className="modal-head"><h2 id="modal-title">{title}</h2><Button variant="icon" aria-label="Close dialog" onClick={onClose}><X size={20}/></Button></div>{children}</dialog>;
}
export function CopyButton({ value, label = 'Copy' }: { value: string; label?: string }) { const { toast } = useApp(); return <Button variant="quiet" onClick={() => { void navigator.clipboard.writeText(value).then(() => toast('Copied to clipboard.')).catch(() => toast('Clipboard is unavailable. Select and copy the value manually.')); }}><Copy size={15}/>{label}</Button>; }
export function Steps({ current, labels }: { current: number; labels: string[] }) { return <ol className="steps" aria-label="Progress">{labels.map((label, i) => <li key={label} className={i === current ? 'current' : i < current ? 'complete' : ''} aria-current={i === current ? 'step' : undefined}><span>{i < current ? <CheckCircle2 size={17}/> : i + 1}</span><strong>{label}</strong></li>)}</ol>; }
export function SecurityNote() { return <p className="security-note"><ShieldCheck size={16}/>Your PIN stays private. Never share it with support.</p>; }
