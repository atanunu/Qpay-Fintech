/** Creates a standalone, synthetic-only review page. Never bundles the live API. */
import { readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
const files = readdirSync('dist/assets');
const scripts = files.filter(x=>x.endsWith('.js'));
if (scripts.length !== 1) throw new Error('Offline export must use one fully inlined review bundle');
const js = readFileSync(resolve('dist/assets',scripts[0]),'utf8');
if (!js.includes('review@qpay.example.invalid')) throw new Error('Offline bundle is missing the explicit synthetic fixture');
const css = files.filter(x=>x.endsWith('.css')).map(x=>readFileSync(resolve('dist/assets',x),'utf8')).join('\n');
const icon = readFileSync('public/icon.svg').toString('base64');
const html = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex,nofollow"><meta name="referrer" content="no-referrer"><title>Qpay · Offline synthetic review</title><link rel="icon" href="data:image/svg+xml;base64,${icon}"><style>${css.replaceAll('</style','<\\/style')}</style></head><body><div id="root"></div><script type="module">${js.replaceAll('</script','<\\/script')}</script></body></html>`;
writeFileSync('dist/qpay-offline-review.html',html);
console.log('Generated standalone synthetic review. No backend or provider is included.');
