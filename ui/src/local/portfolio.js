import { getState, save, newHoldingId } from './store.js';
import { HttpError } from './errors.js';
import { detteInfo } from './finance.js';
import * as rates from './rates.js';
import { fetchPrice, mapLimited } from './yahoo.js';
import { refreshTickerCagrs, bootstrapCagrs } from './projection.js';

export const TYPE_BOURSE = 'bourse';
export const TYPE_CRYPTO = 'crypto';
export const TYPE_DETTE = 'dette';

const hasTickerHoldings = (type) => type === TYPE_BOURSE || type === TYPE_CRYPTO;
const nowIso = () => new Date().toISOString();

function assetsSorted() {
  const { assets } = getState().portfolio;
  return [...assets].sort((a, b) => a.type.localeCompare(b.type) || a.name.localeCompare(b.name));
}

function findAsset(id) {
  const a = getState().portfolio.assets.find((x) => x.id === id);
  if (!a) throw new HttpError(404, 'actif introuvable');
  return a;
}

function holdingApi(assetId, h) {
  return {
    id: h.id,
    asset_id: assetId,
    ticker: h.ticker,
    shares: h.shares,
    created_at: h.created_at,
    updated_at: h.updated_at,
  };
}

function assetApi(a) {
  return {
    id: a.id,
    type: a.type,
    name: a.name,
    value: a.value,
    created_at: a.created_at,
    updated_at: a.updated_at,
  };
}

export function listAssets() {
  return assetsSorted().map(assetApi);
}

function normalizeDette(d) {
  return {
    start_date: String(d.start_date),
    duration_months: Number(d.duration_months),
    taeg: Number(d.taeg),
    amount_borrowed: Number(d.amount_borrowed),
  };
}

// Persists the per-asset rate override for the editable single-asset types.
function applyAssetRate(type, name, id, rate) {
  if (!rates.SINGLE_RATE_TYPES.includes(type)) return;
  rates.setAssetOverride(rates.accountKey(type, id), name, 0, rate ?? null);
}

export async function createAsset(body) {
  if (!body.type || !body.name) throw new HttpError(400, 'type and name are required');
  const { assets } = getState().portfolio;
  const id = assets.reduce((max, a) => Math.max(max, a.id), 0) + 1;
  const now = nowIso();
  assets.push({
    id,
    type: body.type,
    name: body.name,
    value: Number(body.value) || 0,
    created_at: now,
    updated_at: now,
    holdings: [],
    dette: body.type === TYPE_DETTE && body.dette ? normalizeDette(body.dette) : null,
  });
  applyAssetRate(body.type, body.name, id, body.rate);
  await save('portfolio');
  return { id };
}

export async function updateAsset(id, body) {
  const a = findAsset(id);
  a.type = body.type;
  a.name = body.name;
  a.value = Number(body.value) || 0;
  a.updated_at = nowIso();
  if (body.type === TYPE_DETTE && body.dette) a.dette = normalizeDette(body.dette);
  applyAssetRate(body.type, body.name, id, body.rate);
  await save('portfolio');
}

export async function deleteAsset(id) {
  const a = findAsset(id);
  const p = getState().portfolio;
  p.assets = p.assets.filter((x) => x.id !== id);
  rates.removeAssetRates(a.type, id);
  await save('portfolio');
}

// --- Holdings ---

export function listHoldings(assetId) {
  const a = findAsset(assetId);
  return [...a.holdings].sort((x, y) => x.ticker.localeCompare(y.ticker)).map((h) => holdingApi(assetId, h));
}

function cleanTicker(raw) {
  return String(raw || '').trim().toUpperCase();
}

export async function createHolding(assetId, body) {
  const a = findAsset(assetId);
  const ticker = cleanTicker(body.ticker);
  if (!ticker) throw new HttpError(400, 'ticker is required');
  const now = nowIso();
  const id = newHoldingId();
  a.holdings.push({ id, ticker, shares: Number(body.shares) || 0, created_at: now, updated_at: now });
  await save('portfolio');
  return { id };
}

function findHolding(id) {
  for (const a of getState().portfolio.assets) {
    const h = a.holdings.find((x) => x.id === id);
    if (h) return { asset: a, holding: h };
  }
  throw new HttpError(404, 'position introuvable');
}

export async function updateHolding(id, body) {
  const { holding } = findHolding(id);
  holding.ticker = cleanTicker(body.ticker);
  holding.shares = Number(body.shares) || 0;
  holding.updated_at = nowIso();
  await save('portfolio');
}

export async function deleteHolding(id) {
  const { asset, holding } = findHolding(id);
  asset.holdings = asset.holdings.filter((h) => h !== holding);
  await save('portfolio');
}

// --- Ticker categories ---

export async function setCategory(ticker, category) {
  const trimmed = String(category || '').trim();
  if (!trimmed) throw new HttpError(400, 'category is required');
  getState().portfolio.ticker_categories[ticker] = trimmed;
  await save('portfolio');
}

export async function deleteCategory(ticker) {
  delete getState().portfolio.ticker_categories[ticker];
  await save('portfolio');
}

// --- Summary ---

function buildRates(assets, holdings, categories, byCategory) {
  const accountRates = [];
  for (const a of assets) {
    if (!rates.SINGLE_RATE_TYPES.includes(a.type)) continue;
    const key = rates.accountKey(a.type, a.id);
    // An explicit override wins; a non-zero stored rate also counts as "set".
    let rate = 0;
    let isSet = false;
    const ov = rates.overrideOf(key);
    const stored = rates.getRate(key);
    if (ov != null) {
      rate = ov;
      isSet = true;
    } else if (stored && stored.rate !== 0) {
      rate = stored.rate;
      isSet = true;
    }
    accountRates.push({ asset_id: a.id, key, rate, is_set: isSet });
  }

  const keyForLabel = {};
  for (const a of assets) {
    if (a.type !== TYPE_BOURSE) continue;
    for (const h of holdings[a.id] || []) {
      const cat = categories[h.ticker];
      if (cat) keyForLabel[cat] = rates.categoryKey(cat);
      else keyForLabel[h.ticker] = rates.tickerKey(h.ticker);
    }
  }

  const categoryRates = [];
  for (const label of Object.keys(byCategory)) {
    const key = keyForLabel[label];
    if (!key) continue;
    const stored = rates.getRate(key);
    if (!stored) continue;
    categoryRates.push({
      category: label,
      key,
      rate: stored.rate,
      ...(stored.rate_override != null ? { override: stored.rate_override } : {}),
    });
  }
  categoryRates.sort((x, y) => (x.category < y.category ? -1 : x.category > y.category ? 1 : 0));
  return { accountRates, categoryRates };
}

export function summary(now = new Date()) {
  const { portfolio, cache } = getState();
  const assets = assetsSorted();
  const prices = {};
  const dayChanges = {};
  for (const [ticker, p] of Object.entries(cache.ticker_prices)) {
    prices[ticker] = p.price;
    if (p.day_open > 0) dayChanges[ticker] = ((p.price - p.day_open) / p.day_open) * 100;
  }

  const holdings = {};
  const dettes = {};
  for (const a of portfolio.assets) {
    if (a.holdings.length > 0) {
      holdings[a.id] = [...a.holdings].sort((x, y) => x.ticker.localeCompare(y.ticker)).map((h) => holdingApi(a.id, h));
    }
    if (a.dette) dettes[a.id] = detteInfo(a.id, a.dette, now);
  }

  const categories = portfolio.ticker_categories;
  const byType = {};
  const byCategory = {};
  const accountValues = {};
  let total = 0;

  for (const a of assets) {
    let val = 0;
    if (hasTickerHoldings(a.type)) {
      for (const h of holdings[a.id] || []) {
        const pos = (prices[h.ticker] || 0) * h.shares;
        val += pos;
        if (a.type === TYPE_BOURSE) {
          const label = categories[h.ticker] || h.ticker;
          byCategory[label] = (byCategory[label] || 0) + pos;
        }
      }
      accountValues[a.id] = val;
    } else if (a.type === TYPE_DETTE) {
      const d = dettes[a.id];
      val = d ? -d.remaining_capital : 0;
      accountValues[a.id] = val;
    } else {
      val = a.value;
    }
    byType[a.type] = (byType[a.type] || 0) + val;
    total += val;
  }

  const { accountRates, categoryRates } = buildRates(assets, holdings, categories, byCategory);

  return {
    total,
    by_type: byType,
    by_category: byCategory,
    assets: assets.map(assetApi),
    holdings,
    account_values: accountValues,
    dettes,
    ticker_prices: prices,
    ticker_day_changes: dayChanges,
    ticker_categories: categories,
    account_rates: accountRates,
    category_rates: categoryRates,
    last_refresh: cache.last_refresh,
  };
}

// --- Refresh ---

export function distinctTickers({ bourseOnly = false } = {}) {
  const set = new Set();
  for (const a of getState().portfolio.assets) {
    if (bourseOnly && a.type !== TYPE_BOURSE) continue;
    for (const h of a.holdings) if (h.ticker) set.add(h.ticker);
  }
  return [...set];
}

// Fetches prices for every holding, then the CAGRs of the bourse tickers.
// Unknown tickers are skipped; if every request fails (offline) nothing is
// recorded, so the next launch retries.
export async function refreshTickers() {
  const { cache } = getState();
  const tickers = distinctTickers();
  if (tickers.length > 0) {
    const results = await mapLimited(tickers, fetchPrice);
    const fresh = results.filter((r) => r.status === 'fulfilled').map((r) => r.value);
    if (fresh.length === 0) {
      const first = results.find((r) => r.status === 'rejected');
      throw new HttpError(502, `actualisation impossible : ${first?.reason?.message ?? 'réseau indisponible'}`);
    }
    for (const p of fresh) {
      cache.ticker_prices[p.ticker] = { price: p.price, currency: p.currency, day_open: p.day_open };
    }
    await refreshTickerCagrs(distinctTickers({ bourseOnly: true }));
  }
  cache.last_refresh = nowIso();
  await save('cache');
}

// Populates the category rates on first launch if none exist yet.
export function bootstrap() {
  return bootstrapCagrs(distinctTickers({ bourseOnly: true }));
}
