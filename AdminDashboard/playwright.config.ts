import {defineConfig,devices} from '@playwright/test';
const api=process.env.QPF_ADMIN_TEST_MODE==='api';
export default defineConfig({testDir:'./e2e',testMatch:api?'*.api.spec.ts':'*.review.spec.ts',timeout:60000,expect:{timeout:12000},fullyParallel:false,workers:1,retries:0,reporter:[['list'],['json',{outputFile:api?'test-results/api-report.json':'test-results/review-report.json'}],['html',{outputFolder:api?'playwright-report/api':'playwright-report/review',open:'never'}]],
 use:{baseURL:'http://localhost:5174',launchOptions:process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE?{executablePath:process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE}:{},trace:'off',screenshot:'off',video:'off'},
 projects:[{name:'chromium',use:{...devices['Desktop Chrome'],viewport:{width:1440,height:1000}}}],
 webServer:{command:api?'npm run dev -- --host 127.0.0.1':'npm run dev:review -- --host 127.0.0.1',url:'http://localhost:5174/login',reuseExistingServer:false,timeout:60000}});
// No traces containing passwords, cookies, invitation tokens or private evidence are archived.
