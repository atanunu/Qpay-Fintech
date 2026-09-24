import { loadEnv } from 'vite';
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { resolve } from 'node:path';
export default defineConfig(({mode})=>{
 const env=loadEnv(mode,process.cwd(),'');const review=env.VITE_ADMIN_MODE==='review';
 if(env.VITE_ADMIN_MODE && !['api','review'].includes(env.VITE_ADMIN_MODE))throw new Error('Invalid admin mode');
 if(review && env.VITE_REVIEW_ACK!=='synthetic-data-only')throw new Error('Review mode requires explicit acknowledgement');
 return {plugins:[react()],resolve:{alias:{'@transport':resolve(process.cwd(),review?'src/review.ts':'src/api.ts')}},define:{__ADMIN_REVIEW__:JSON.stringify(review)},server:{host:'127.0.0.1',port:5174,strictPort:true},preview:{host:'127.0.0.1',port:4174,strictPort:true},build:{target:'es2022',sourcemap:false},test:{include:['tests/**/*.test.ts'],environment:'node'}};
});
