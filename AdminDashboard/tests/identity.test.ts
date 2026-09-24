import {expect, it} from 'vitest';
import {identityFields} from '../src/identity';
it('projects submitted identity as individual fields without unapproved properties', () => {
  const raw = {owner_id: 'synthetic', identity: {legal_name: 'Synthetic Legal Customer', date_of_birth: '1990-01-01', address: 'Synthetic address', document_type: 'passport', consent: true, version: 3, secret: 'must-not-display', upload_ids: ['private-reference']}};
  expect(identityFields(raw)).toEqual({legal_name: 'Synthetic Legal Customer', date_of_birth: '1990-01-01', address: 'Synthetic address', document_type: 'passport', consent: true, version: 3});
  expect(raw.identity.secret).toBe('must-not-display');
});
for (const identity of [null, 'invalid', [], 1]) it('fails closed for an invalid identity envelope: ' + JSON.stringify(identity), () => expect(() => identityFields({identity})).toThrow(/invalid/));
it('does not recursively display arbitrary nested identity content', () => expect(identityFields({identity: {legal_name: {unexpected: 'secret'}, consent: false}})).toEqual({consent: false}));
