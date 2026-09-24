import {closeSync, existsSync, openSync, readFileSync, renameSync, unlinkSync, writeFileSync} from 'node:fs';

/**
 * Reserve before submitting. Playwright restarts workers after failures, so a
 * process-global counter would reuse already-consumed MFA recovery credentials.
 * Only non-secret counters are persisted in the disposable fixture directory.
 * The exclusive lock also fails closed if a future runner enables parallel use.
 */
export function reserveRecoverySlot(file: string, role: string, available: number): number {
  const lock = file + '.lock';
  const fd = openSync(lock, 'wx', 0o600);
  try {
    const counters: Record<string, number> = existsSync(file) ? JSON.parse(readFileSync(file, 'utf8')) : {};
    const index = Object.hasOwn(counters, role) ? counters[role] : 0;
    if (!Number.isSafeInteger(index) || index < 0 || index >= available) {
      throw new Error('Synthetic recovery proofs exhausted; provision a fresh isolated fixture.');
    }
    Object.defineProperty(counters, role, {value: index + 1, writable: true, enumerable: true, configurable: true});
    writeFileSync(file + '.tmp', JSON.stringify(counters), {mode: 0o600});
    renameSync(file + '.tmp', file);
    return index;
  } finally {
    closeSync(fd);
    unlinkSync(lock);
  }
}
