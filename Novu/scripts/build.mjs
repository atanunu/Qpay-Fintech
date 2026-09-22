/** Reproducible, offline preview/export build. Does not sync or send notifications. */
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { catalogue, fixture, payloadSchema, renderEmail, themes } from '../src/content.mjs';
const dir = fileURLToPath(new URL('../dist/', import.meta.url));
const check = process.argv.includes('--check');
const counts = {};
for (const event of catalogue) counts[event.scope] = (counts[event.scope] || 0) + 1;
const digest = createHash('sha256').update(readFileSync(new URL('../catalogue/emails.psv', import.meta.url))).digest('hex');
const summary = { schema_version: 1, catalogue_sha256: digest, total: catalogue.length, scopes: counts, themes: Object.keys(themes), code_challenges: catalogue.filter(x => x.tokens.includes('code')).length, status: 'content-built; live-setup-unverified', live_workflows_enabled: 0 };
const summaryPath = new URL('../docs/CONTENT-SUMMARY.json', import.meta.url);
if (check) {
  if (readFileSync(summaryPath, 'utf8') !== JSON.stringify(summary, null, 2) + '\n') throw new Error('Content summary is stale; run npm run build and commit it');
} else {
  mkdirSync(dir, { recursive: true });
  writeFileSync(summaryPath, JSON.stringify(summary, null, 2) + '\n');
}
for (const event of catalogue) {
  const payload = fixture(event), rendered = renderEmail(event, payload);
  if (!check) {
    for (const [ext, content] of [['html', rendered.html], ['txt', rendered.text], ['schema.json', JSON.stringify(payloadSchema(event), null, 2)], ['fixture.json', JSON.stringify(payload, null, 2)]]) writeFileSync(`${dir}${event.workflowId}.${ext}`, content);
  }
}
if (!check) {
  writeFileSync(`${dir}catalogue.json`, JSON.stringify(catalogue, null, 2));
  writeFileSync(`${dir}index.html`, `<!doctype html><html lang="en"><meta charset="utf-8"><title>Qpay-Fintech email catalogue</title><body><h1>Built-in email content previews</h1><p>Synthetic data. Not Novu delivery or email-client acceptance.</p><ul>${catalogue.map(e => `<li><a href="${e.workflowId}.html">${e.workflowId}</a> — ${e.scope}; ${e.audience}; ${e.theme}</li>`).join('')}</ul></body></html>`);
}
console.log(JSON.stringify({ ...summary, mode: check ? 'check' : 'build', rendered: catalogue.length, output: check ? null : dir }, null, 2));
