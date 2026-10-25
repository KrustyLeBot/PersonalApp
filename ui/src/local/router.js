// The UI was written against an HTTP API. Instead of a server, requests to /api/*
// are answered here from the local data files, so the components stay unchanged.
import { HttpError } from './errors.js';
import { save } from './store.js';
import * as portfolio from './portfolio.js';
import * as projection from './projection.js';
import * as rates from './rates.js';
import * as telework from './telework.js';
import * as walk from './walk.js';

const routes = [];

function route(method, pattern, handler) {
  const re = new RegExp('^' + pattern.replace(/\{(\w+)\}/g, '(?<$1>[^/]+)') + '$');
  routes.push({ method, re, handler });
}

const int = (v) => Number.parseInt(v, 10);

route('GET', '/api/portfolio/assets', () => portfolio.listAssets());
route('POST', '/api/portfolio/assets', ({ body }) => portfolio.createAsset(body));
route('PUT', '/api/portfolio/assets/{id}', ({ params, body }) => portfolio.updateAsset(int(params.id), body));
route('DELETE', '/api/portfolio/assets/{id}', ({ params }) => portfolio.deleteAsset(int(params.id)));

route('GET', '/api/portfolio/assets/{id}/holdings', ({ params }) => portfolio.listHoldings(int(params.id)));
route('POST', '/api/portfolio/assets/{id}/holdings', ({ params, body }) => portfolio.createHolding(int(params.id), body));
route('PUT', '/api/portfolio/holdings/{id}', ({ params, body }) => portfolio.updateHolding(int(params.id), body));
route('DELETE', '/api/portfolio/holdings/{id}', ({ params }) => portfolio.deleteHolding(int(params.id)));

route('PUT', '/api/portfolio/tickers/{ticker}/category', ({ params, body }) => portfolio.setCategory(params.ticker, body.category));
route('DELETE', '/api/portfolio/tickers/{ticker}/category', ({ params }) => portfolio.deleteCategory(params.ticker));

route('GET', '/api/portfolio/summary', () => portfolio.summary());
route('POST', '/api/portfolio/refresh', async () => {
  await portfolio.refreshTickers();
  return { status: 'ok' };
});

route('PUT', '/api/portfolio/projection/rates/{key}/rate-override', ({ params, body }) =>
  projection.setRateOverride(params.key, body.rate),
);
route('GET', '/api/portfolio/projection/summary', async () => {
  const result = projection.computeProjection();
  if (rates.takeDirty()) await save('portfolio');
  return result;
});

route('GET', '/api/telework/summary/{year}', ({ params }) => telework.summary(int(params.year)));
route('GET', '/api/telework/preset', () => telework.getPreset());
route('PUT', '/api/telework/preset', ({ body }) => telework.savePreset(body));
route('GET', '/api/telework/overrides/{year}', ({ params }) => telework.getOverrides(int(params.year)));
route('PUT', '/api/telework/overrides/{year}', ({ params, body }) => telework.setOverrides(int(params.year), body));

route('GET', '/api/walk/summary/{year}', ({ params }) => walk.summary(int(params.year)));
route('PUT', '/api/walk/day', ({ body }) => walk.setDay(body));

export async function handle(method, path, body) {
  for (const r of routes) {
    if (r.method !== method) continue;
    const m = r.re.exec(path);
    if (!m) continue;
    const params = {};
    for (const [k, v] of Object.entries(m.groups || {})) params[k] = decodeURIComponent(v);
    return r.handler({ params, body });
  }
  throw new HttpError(404, `route inconnue : ${method} ${path}`);
}

export function installFetchRouter() {
  const nativeFetch = window.fetch.bind(window);
  window.fetch = async (input, init = {}) => {
    const url = typeof input === 'string' ? input : input.url;
    if (!url.startsWith('/api/')) return nativeFetch(input, init);

    const method = (init.method || 'GET').toUpperCase();
    const path = url.split('?')[0];
    try {
      const body = typeof init.body === 'string' && init.body ? JSON.parse(init.body) : undefined;
      const data = await handle(method, path, body);
      if (data === undefined) return new Response(null, { status: 204 });
      const status = method === 'POST' && data && typeof data.id === 'number' ? 201 : 200;
      return new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } });
    } catch (e) {
      const status = e instanceof HttpError ? e.status : 500;
      return new Response(e.message, { status });
    }
  };
}
