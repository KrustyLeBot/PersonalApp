import { platform } from '../platform/index.js';
import { StorageAccessError } from './errors.js';

export const FORMAT_VERSION = 1;

const FILES = {
  portfolio: 'portfolio.json',
  telework: 'telework.json',
  walk: 'walk.json',
  cache: 'cache.json',
};

let state = null;
let nextHoldingId = 1;
let info = null;
const writeChains = {};

export function defaultPortfolio() {
  return { assets: [], ticker_categories: {}, projection_rates: [] };
}
export function defaultTelework() {
  return { remote_days: [4, 5], overrides: {} };
}
export function defaultWalk() {
  return { entries: {} };
}
export function defaultCache() {
  return { ticker_prices: {}, last_refresh: null };
}

export function newHoldingId() {
  return nextHoldingId++;
}

// Fills defaults and gives every holding an in-memory id (ids are not persisted).
export function normalize(raw) {
  const portfolio = { ...defaultPortfolio(), ...raw.portfolio };
  portfolio.assets = (portfolio.assets || []).map((a) => ({
    id: a.id,
    type: a.type,
    name: a.name,
    value: a.value || 0,
    created_at: a.created_at || new Date().toISOString(),
    updated_at: a.updated_at || new Date().toISOString(),
    holdings: (a.holdings || []).map((h) => ({
      id: newHoldingId(),
      ticker: h.ticker,
      shares: h.shares,
      created_at: a.created_at || '',
      updated_at: a.updated_at || '',
    })),
    dette: a.dette || null,
  }));
  portfolio.ticker_categories = portfolio.ticker_categories || {};
  portfolio.projection_rates = portfolio.projection_rates || [];
  return {
    portfolio,
    telework: { ...defaultTelework(), ...raw.telework },
    walk: { ...defaultWalk(), ...raw.walk },
    cache: { ...defaultCache(), ...raw.cache },
  };
}

// Persisted shape: holding ids and timestamps are in-memory only.
export function serialize(key, data) {
  if (key !== 'portfolio') return data;
  return {
    assets: data.assets.map((a) => {
      const out = {
        id: a.id,
        type: a.type,
        name: a.name,
        value: a.value,
        created_at: a.created_at,
        updated_at: a.updated_at,
        holdings: a.holdings.map((h) => ({ ticker: h.ticker, shares: h.shares })),
      };
      if (a.dette) out.dette = a.dette;
      return out;
    }),
    ticker_categories: data.ticker_categories,
    projection_rates: data.projection_rates,
  };
}

async function readJson(name, fallback) {
  const text = await platform.readFile(name);
  if (text == null || text.trim() === '') return fallback;
  try {
    return JSON.parse(text);
  } catch (e) {
    // Never overwrite a file we could not parse.
    throw new Error(`${name} est illisible : ${e.message}`);
  }
}

export async function getInfo() {
  info = await platform.getInfo();
  return info;
}

export async function load() {
  const i = await getInfo();
  if (!i.hasAccess) throw new StorageAccessError('Accès aux fichiers non autorisé');

  const manifest = await readJson('manifest.json', null);
  if (manifest && manifest.format_version > FORMAT_VERSION) {
    throw new Error(`Format de données v${manifest.format_version} trop récent pour cette version de l'app`);
  }

  nextHoldingId = 1;
  state = normalize({
    portfolio: await readJson(FILES.portfolio, undefined),
    telework: await readJson(FILES.telework, undefined),
    walk: await readJson(FILES.walk, undefined),
    cache: await readJson(FILES.cache, undefined),
  });
  return state;
}

export function getState() {
  if (!state) throw new Error('Données non chargées');
  return state;
}

// Test hook: replace the in-memory state without touching the platform.
export function setState(next) {
  state = next;
}

// Writes are chained per file so two quick saves never interleave.
export function save(key) {
  const body = JSON.stringify(serialize(key, getState()[key]), null, 2) + '\n';
  const prev = writeChains[key] || Promise.resolve();
  const next = prev.catch(() => {}).then(() => platform.writeFile(FILES[key], body));
  writeChains[key] = next;
  return next;
}
