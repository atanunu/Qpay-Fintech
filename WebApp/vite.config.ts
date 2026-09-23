import { loadEnv } from 'vite';
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const apiMode = env.VITE_API_MODE || 'api';
  if (!['api', 'review'].includes(apiMode)) throw new Error('VITE_API_MODE must be api or review');
  if (apiMode === 'review' && env.VITE_REVIEW_ACK !== 'synthetic-data-only') throw new Error('Review builds require explicit synthetic-data-only acknowledgement');
  return {
    plugins: [react()],
    server: { host: '127.0.0.1', port: 5173, strictPort: true },
    preview: { host: '127.0.0.1', port: 4173, strictPort: true },
    build: { target: 'es2022', sourcemap: false, ...(process.env.QPF_OFFLINE_EXPORT === 'true' ? { rollupOptions: { output: { inlineDynamicImports: true } } } : {}) },
    test: { include: ['tests/**/*.test.ts'], environment: 'node' },
  };
});
