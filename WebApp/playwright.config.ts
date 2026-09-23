import { defineConfig, devices } from '@playwright/test';
const api = process.env.QPF_TEST_MODE === 'api';
export default defineConfig({
  testDir: './e2e', testMatch: api ? 'api.spec.ts' : 'review.spec.ts',
  timeout: 45000, expect: { timeout: 12000 }, fullyParallel: false, workers: process.env.CI ? 2 : 1, retries: 0,
  reporter: [['list'], ['json', { outputFile: api ? 'test-results/api-report.json' : 'test-results/review-report.json' }], ['html', { outputFolder: api ? 'test-results/api-html' : 'test-results/review-html', open: 'never' }]],
  use: { baseURL: 'http://localhost:5173', trace: 'retain-on-failure', screenshot: 'only-on-failure', video: 'off' },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'], viewport: { width:1440,height:1000 } } }],
  webServer: { command: api ? 'npm run dev -- --host 127.0.0.1' : 'npm run dev:review -- --host 127.0.0.1', url: 'http://localhost:5173/login', reuseExistingServer: !process.env.CI, timeout: 60000 },
});
