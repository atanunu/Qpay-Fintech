import { ArrowDownLeft, ArrowUpRight, ChevronRight, Zap } from 'lucide-react';
import { Link } from 'react-router-dom';
import type { Payment } from '../api/types';
import { dateTime, money } from '../api/money';
import { Badge, Empty } from './ui';
export function PaymentRows({ payments }: { payments: Payment[] }) {
  if (!payments.length) return <Empty title="Your next chapter starts here">Your payments will appear here once you make a transaction.</Empty>;
  return <div className="payment-list">{payments.map(p => <Link key={p.id} className="payment-row" to={`/activity/${encodeURIComponent(p.id)}`}><span className={`transaction-icon ${p.direction === 'incoming' ? 'incoming' : p.kind}`}>
    {p.kind === 'bill' ? <Zap size={18}/> : p.direction === 'incoming' ? <ArrowDownLeft size={19}/> : <ArrowUpRight size={19}/>}</span><span className="transaction-info"><strong>{p.kind === 'bill' ? 'Bill payment' : p.direction === 'incoming' ? 'Money received' : p.kind === 'internal' ? 'Qpay transfer' : 'Bank transfer'}</strong><small>{dateTime(p.created_at)}</small></span><span className="transaction-state"><Badge status={p.status}/></span><span className={`transaction-amount ${p.direction === 'incoming' ? 'credit' : ''}`}><strong>{p.direction === 'incoming' ? '+' : '−'}{money(p.direction === 'incoming' ? p.amount_minor : p.total_minor)}</strong><small>{p.kind === 'bill' && p.fulfilment_status === 'pending' ? 'Value pending' : p.currency}</small></span><ChevronRight size={15} className="muted"/></Link>)}</div>;
}
