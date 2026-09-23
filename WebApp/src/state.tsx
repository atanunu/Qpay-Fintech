import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { APIError, HttpTransport, message, restoreUser } from './api/client';
import type { Capabilities, Session, Transport, User } from './api/types';
import type { ReviewControls } from './review/transport';
import { apiBase, isReview } from './api/config';

interface State {
  client: Transport; review?: ReviewControls; user: User | null; capabilities: Capabilities | null;
  status: 'loading' | 'signed-out' | 'ready' | 'unavailable'; error: unknown; revision: number;
  refresh(): void; reloadUser(): Promise<void>; signIn(body: Record<string, string>): Promise<void>; signOut(): Promise<void>;
  invalidate(): void; toast(value: string): void;
}
const Context = createContext<State | null>(null);
export function useApp() { const value = useContext(Context); if (!value) throw new Error('App context is missing'); return value; }
export async function createTransport(): Promise<Transport & Partial<ReviewControls>> {
  if (isReview) { const { ReviewTransport } = await import('./review/transport'); return new ReviewTransport(); }
  return new HttpTransport(apiBase);
}
export function AppProvider({ client, children }: { client: Transport & Partial<ReviewControls>; children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [capabilities, setCapabilities] = useState<Capabilities | null>(null);
  const [status, setStatus] = useState<State['status']>('loading');
  const [error, setError] = useState<unknown>();
  const [revision, setRevision] = useState(0);
  const [notice, setNotice] = useState('');
  const generation = useRef(0);
  const channel = useRef<BroadcastChannel | null>(null);
  const refresh = useCallback(() => {
    const g = ++generation.current; setStatus('loading'); setError(undefined);
    void (async () => {
      try {
        const cap = await client.request<Capabilities>('GET', '/v1/capabilities');
        if (g !== generation.current) return; setCapabilities(cap);
        const me = await restoreUser(client);
        if (g !== generation.current) return; setUser(me); setStatus('ready');
      } catch (e) {
        if (g !== generation.current) return;
        setUser(null);
        if (e instanceof APIError && e.status === 401) setStatus('signed-out');
        else { setError(e); setStatus('unavailable'); }
      }
    })();
  }, [client]);
  useEffect(() => { refresh(); return () => { generation.current++; }; }, [refresh]);
  useEffect(() => {
    channel.current = typeof BroadcastChannel !== 'undefined' ? new BroadcastChannel('qpf-auth') : null;
    if (channel.current) channel.current.onmessage = () => { client.clear(); refresh(); };
    return () => channel.current?.close();
  }, [refresh, client]);
  useEffect(() => { if (!notice) return; const timer = setTimeout(() => setNotice(''), 6500); return () => clearTimeout(timer); }, [notice]);
  const value: State = {
    client, review: isReview ? client as Transport & ReviewControls : undefined,
    user, capabilities, status, error, revision, refresh,
    reloadUser: async () => { setUser(await client.request<User>('GET', '/v1/me')); setRevision(v => v + 1); },
    signIn: async body => { const session = await client.request<Session>('POST', '/v1/auth/login', { body: { ...body, client: 'web', device: 'Qpay Web · ' + (navigator.userAgent.includes('Mobile') ? 'mobile browser' : 'desktop browser') } }); setUser(session.user); setStatus('ready'); setRevision(v => v + 1); channel.current?.postMessage('changed'); },
    signOut: async () => { await client.request('POST', '/v1/auth/logout'); client.clear(); setUser(null); setStatus('signed-out'); setRevision(v => v + 1); channel.current?.postMessage('changed'); },
    invalidate: () => setRevision(v => v + 1), toast: setNotice,
  };
  return <Context.Provider value={value}>{children}<div className="toast-region" aria-live="polite" aria-atomic="true">{notice && <div className="toast">{notice}<button type="button" aria-label="Dismiss message" onClick={() => setNotice('')}>×</button></div>}</div></Context.Provider>;
}
export function useResource<T>(path: string | null) {
  const { client, revision } = useApp();
  const [data, setData] = useState<T>(); const [error, setError] = useState<unknown>(); const [loading, setLoading] = useState(true);
  const [nonce, setNonce] = useState(0);
  useEffect(() => {
    const abort = new AbortController(); let active = true;
    if (!path) { setLoading(false); setData(undefined); return; }
    setLoading(true); setError(undefined); setData(undefined);
    client.request<T>('GET', path, { signal: abort.signal }).then(value => { if (active) setData(value); }).catch(e => { if (active && !abort.signal.aborted) setError(e); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; abort.abort(); };
  }, [client, path, revision, nonce]);
  return { data, error, loading, reload: () => setNonce(v => v + 1), setData };
}
export function useAction() {
  const [busy, setBusy] = useState(false); const [error, setError] = useState<unknown>(); const lock = useRef(false);
  const run = async (fn: () => Promise<void>) => {
    if (lock.current) return; lock.current = true; setBusy(true); setError(undefined);
    try { await fn(); } catch (e) { setError(e); } finally { setBusy(false); lock.current = false; }
  };
  return { busy, error, run, setError, clear: () => setError(undefined) };
}
export { message };
