import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import { createApi, networkTransport, type Transport } from './api';
import { createFixtureServer } from './fixtures';
import './styles.css';

// Until the demo program exists the default build answers itself from the fixtures.
// VITE_API=network makes the real calls; that becomes the only path when it is connected.
function fixtureTransport(): Transport {
  const server = createFixtureServer();
  return async (req) => server.handle(req);
}

const transport = import.meta.env.VITE_API === 'network' ? networkTransport : fixtureTransport();

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App api={createApi(transport)} />
  </StrictMode>,
);
