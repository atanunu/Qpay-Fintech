import { createRoot } from 'react-dom/client';
import { BrowserRouter, HashRouter, MemoryRouter } from 'react-router-dom';
import App from './App';
import { AppProvider, createTransport } from './state';
import './styles/app.css';

const Router = import.meta.env.VITE_REVIEW_ROUTER === 'memory' && import.meta.env.VITE_API_MODE === 'review' ? MemoryRouter : import.meta.env.VITE_REVIEW_ROUTER === 'hash' && import.meta.env.VITE_API_MODE === 'review' ? HashRouter : BrowserRouter;
const root = createRoot(document.getElementById('root')!);
createTransport().then(client => root.render(<Router><AppProvider client={client}><App/></AppProvider></Router>)).catch(() => {
  root.render(<main className="fatal-screen"><h1>The application could not start.</h1><p>Check the reviewed environment configuration. No payment was submitted. This application does not fall back to sample data when its API is unavailable.</p><button className="button primary" onClick={() => window.location.reload()}>Reload</button></main>);
});
