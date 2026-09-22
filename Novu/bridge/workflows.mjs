/** Import this factory from a qualified @novu/framework bridge. No HTTP server is implied. */
import { catalogue, payloadSchema, renderEmail, validateBrand, validatePayload } from '../src/content.mjs';

export function defineWorkflows({ workflow, authorise, recordSuppression, providerOptions, brand, allowlist = [], approvedScopes = ['core'], now = Date.now }) {
  for (const f of [workflow, authorise, recordSuppression, providerOptions, now]) if (typeof f !== 'function') throw new Error('Explicit framework, policy, suppression, provider and clock bindings required');
  validateBrand(brand, true);
  if (!Array.isArray(allowlist) || new Set(allowlist).size !== allowlist.length) throw new Error('Invalid release allowlist');
  return allowlist.map(id => {
    const event = catalogue.find(x => x.workflowId === id);
    if (!event || !approvedScopes.includes(event.scope)) throw new Error('Unknown or unapproved workflow');
    return workflow(event.workflowId, async ({ step, payload, subscriber }) => {
      validatePayload(event, payload);
      if (subscriber?.subscriberId !== payload.recipient_ref) throw new Error('Subscriber does not match recipient snapshot');
      const rendered = renderEmail(event, payload, brand);
      // The provider adapter maps plain text and unsubscribe headers using the qualified provider API.
      const providers = providerOptions({ event, payload, rendered });
      if (!providers || typeof providers !== 'object' || Array.isArray(providers) || !Object.keys(providers).length) throw new Error('Qualified provider mapping required');
      await step.email('deliver-email', async () => ({ subject: rendered.subject, body: rendered.html }), {
        providers,
        skip: async () => {
          const clock = now();
          if (!Number.isFinite(clock)) throw new Error('Invalid clock');
          const stale = clock >= Date.parse(payload.expires_at) || clock < Date.parse(payload.occurred_at) - 30000;
          const decision = stale ? { allowed: false, reason: 'expired-or-future' } : await authorise({ event, payload });
          if (!decision || typeof decision.allowed !== 'boolean') throw new Error('Invalid authorisation response');
          if (!decision.allowed) await recordSuppression({ notificationId: payload.notification_id, reason: decision.reason || 'policy-denied' });
          return !decision.allowed;
        }
      });
    }, { name: event.subject, description: `Guard: ${event.guard}; audience: ${event.audience}`, tags: ['qpay-fintech', event.scope, event.classification], payloadSchema: payloadSchema(event), preferences: { all: { enabled: false }, channels: { email: { enabled: true } } } });
  });
}
