import type {Row} from './types';

/** Explicit display projection; unknown API fields must never reveal extra PII. */
export function identityFields(envelope: Row): Row {
  const value = envelope.identity;
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('Identity response is invalid. No additional evidence will be shown.');
  }
  const fields: Row = {};
  for (const key of ['legal_name', 'date_of_birth', 'address', 'document_type', 'consent', 'version']) {
    if (Object.hasOwn(value, key) && ['string', 'number', 'boolean'].includes(typeof value[key])) {
      fields[key] = value[key];
    }
  }
  return fields;
}
