import { safeBaseURL } from './client';
export const isReview = import.meta.env.VITE_API_MODE === 'review';
export const reviewTools = isReview || import.meta.env.VITE_ENABLE_REVIEW_TOOLS === 'true';
export const apiBase = isReview ? '' : safeBaseURL(import.meta.env.VITE_API_BASE_URL || (import.meta.env.DEV ? 'http://localhost:8080' : 'https://api.example.invalid'), import.meta.env.DEV);
