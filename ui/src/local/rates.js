// Projection rates, keyed like the old projection_rates table:
// "category_<slug>" / "ticker_<TICKER>" for bourse, "<type>_<assetID>" for single assets.
import { getState } from './store.js';

export const SINGLE_RATE_TYPES = ['livret', 'fond_euro', 'structure', 'immobilier'];

export function categoryKey(category) {
  return 'category_' + category.toLowerCase().replaceAll(' ', '_');
}
export function tickerKey(ticker) {
  return 'ticker_' + ticker;
}
export function accountKey(type, id) {
  return `${type}_${id}`;
}

function list() {
  return getState().portfolio.projection_rates;
}

export function getRate(key) {
  return list().find((r) => r.key === key) || null;
}

export function overrideOf(key) {
  const r = getRate(key);
  return r && r.rate_override != null ? r.rate_override : null;
}

export function effectiveRate(rate) {
  return rate.rate_override != null ? rate.rate_override : rate.rate;
}

// Insert or update; the user override is preserved.
export function upsertRate({ key, label, rate, source_url }) {
  const existing = getRate(key);
  if (existing) {
    existing.label = label;
    existing.rate = rate;
    existing.source_url = source_url;
  } else {
    list().push({ key, label, rate, source_url, rate_override: null });
  }
}

export function setOverride(key, override) {
  const r = getRate(key);
  if (r) r.rate_override = override;
}

// Creates the row first if needed (defaultRate is what clearing the override falls back to).
export function setAssetOverride(key, label, defaultRate, override) {
  if (!getRate(key)) list().push({ key, label, rate: defaultRate, source_url: '', rate_override: null });
  setOverride(key, override);
}

let dirty = false;

// Lazily materialises a per-asset rate; flags the store as needing a save.
export function ensureRate(key, label, defaultRate) {
  const r = getRate(key);
  if (r) {
    if (r.label !== label) {
      r.label = label;
      dirty = true;
    }
    return r;
  }
  const created = { key, label, rate: defaultRate, source_url: '', rate_override: null };
  list().push(created);
  dirty = true;
  return created;
}

export function takeDirty() {
  const was = dirty;
  dirty = false;
  return was;
}

export function hasCategoryRates() {
  return list().some((r) => r.key.startsWith('category_'));
}

export function removeAssetRates(type, id) {
  const key = accountKey(type, id);
  const rates = getState().portfolio;
  rates.projection_rates = rates.projection_rates.filter((r) => r.key !== key);
}

// Shape returned to the UI (matches the old API: rate_override omitted when unset).
export function toApi(r) {
  const out = { key: r.key, label: r.label, rate: r.rate, source_url: r.source_url };
  if (r.rate_override != null) out.rate_override = r.rate_override;
  return out;
}
