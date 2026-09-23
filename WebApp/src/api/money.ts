/** No floating point in money parsing, totals, or receipt comparisons. */
export const MAX_MINOR = 9000000000000000n;
export function minor(value: unknown, signed = false): string {
  if (typeof value !== 'string' || !(signed ? /^-?(0|[1-9][0-9]{0,15})$/ : /^(0|[1-9][0-9]{0,15})$/).test(value)) throw new Error('The API returned an invalid money value.');
  const n = BigInt(value);
  if (n > MAX_MINOR || n < -MAX_MINOR) throw new Error('Amount exceeds the supported limit.');
  return value;
}
export function parseAmount(input: string): string {
  const text = input.trim();
  if (!/^(0|[1-9]\d*)(\.\d{1,2})?$/.test(text)) throw new Error('Enter an amount such as 1000.00, without commas.');
  const [whole, fraction = ''] = text.split('.');
  const n = BigInt(whole) * 100n + BigInt(fraction.padEnd(2, '0'));
  if (n <= 0n || n > MAX_MINOR) throw new Error('Enter a positive amount within the supported limit.');
  return n.toString();
}
export function amountInput(value: string): string { const n = BigInt(minor(value)); return `${n / 100n}.${(n % 100n).toString().padStart(2, '0')}`; }
export function money(value: string, signed = false): string {
  const n = BigInt(minor(value, signed)); const magnitude = n < 0n ? -n : n;
  return `${n < 0n ? '−' : ''}₦${new Intl.NumberFormat('en-NG').format(magnitude / 100n)}.${(magnitude % 100n).toString().padStart(2, '0')}`;
}
export function dateTime(value: string): string { const date = new Date(value); return Number.isNaN(date.getTime()) ? 'Date unavailable' : new Intl.DateTimeFormat('en-NG', { timeZone: 'Africa/Lagos', dateStyle: 'medium', timeStyle: 'short' }).format(date); }
export function shortDate(value: string): string { return new Intl.DateTimeFormat('en-NG', { timeZone: 'Africa/Lagos', day: 'numeric', month: 'short' }).format(new Date(value)); }
export function statementRange(from: string, to: string, now = new Date()): { from: string; to: string } {
  for (const d of [from, to]) { if (!/^\d{4}-\d{2}-\d{2}$/.test(d) || new Date(d + 'T00:00:00Z').toISOString().slice(0, 10) !== d) throw new Error('Select valid dates.'); }
  const start = new Date(from + 'T00:00:00+01:00');
  const endOfDay = new Date(new Date(to + 'T00:00:00+01:00').getTime() + 86400000);
  const end = new Date(Math.min(endOfDay.getTime(), now.getTime()));
  if (end <= start || end.getTime() - start.getTime() > 366 * 86400000 || to < from) throw new Error('Select a past or current range of no more than 366 days.');
  return { from: start.toISOString(), to: end.toISOString() };
}
export function download(name: string, content: string | Blob, type = 'application/json'): void {
  const blob = typeof content === 'string' ? new Blob([content], { type }) : content;
  const url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href = url; a.download = name; a.click(); setTimeout(() => URL.revokeObjectURL(url), 1000);
}
