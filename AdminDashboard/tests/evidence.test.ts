import {describe, expect, it} from 'vitest';
import {mkdtempSync, readFileSync, rmSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {evidenceFilename} from '../src/evidence';
import {reserveRecoverySlot} from '../e2e/recovery-slots';

describe('neutral private evidence filenames', () => {
  for (const [mime, name] of [['image/png', 'private-evidence.png'], ['image/jpeg', 'private-evidence.jpg'], ['application/pdf', 'private-evidence.pdf'], ['IMAGE/PNG; charset=binary', 'private-evidence.png']]) {
    it(mime, () => expect(evidenceFilename(mime)).toBe(name));
  }
  for (const mime of ['text/html', 'image/svg+xml', 'application/javascript', '', '__proto__']) {
    it('rejects ' + mime, () => expect(() => evidenceFilename(mime)).toThrow());
  }
});
it('reserves distinct proofs across simulated worker restarts without persisting secrets', () => {
  const dir = mkdtempSync(join(tmpdir(), 'admin-proof-test-'));
  const file = join(dir, 'usage.json');
  try {
    expect(reserveRecoverySlot(file, 'admin', 2)).toBe(0);
    expect(JSON.parse(readFileSync(file, 'utf8'))).toEqual({admin: 1});
    expect(reserveRecoverySlot(file, 'admin', 2)).toBe(1);
    expect(() => reserveRecoverySlot(file, 'admin', 2)).toThrow(/exhausted/);
    expect(reserveRecoverySlot(file, 'support', 2)).toBe(0);
    writeFileSync(file + '.lock', '');
    expect(() => reserveRecoverySlot(file, 'support', 2)).toThrow();
  } finally { rmSync(dir, {recursive: true, force: true}); }
});
