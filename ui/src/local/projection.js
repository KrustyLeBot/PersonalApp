import { getState, save } from './store.js';
import { detteInfo, remainingCapital, parseDate, round2 } from './finance.js';
import * as rates from './rates.js';
import { fetchCagr, mapLimited } from './yahoo.js';

export const PROJECTION_YEARS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 20];

function compoundValues(start, annualRatePct) {
  const r = annualRatePct / 100;
  const out = {};
  for (const y of PROJECTION_YEARS) out[y] = round2(start * Math.pow(1 + r, y));
  return out;
}

function flatValues(value) {
  const out = {};
  for (const y of PROJECTION_YEARS) out[y] = value;
  return out;
}

// One CAGR per category (the ticker with the longest history is the representative),
// and one per uncategorised ticker.
export async function refreshTickerCagrs(tickers) {
  const categories = getState().portfolio.ticker_categories;
  const results = await mapLimited(tickers, fetchCagr);

  const byTicker = {};
  results.forEach((r, i) => {
    if (r.status === 'fulfilled') byTicker[tickers[i]] = r.value;
  });

  const bestByCategory = {};
  for (const [ticker, res] of Object.entries(byTicker)) {
    const cat = categories[ticker];
    if (!cat) continue;
    if (!bestByCategory[cat] || res.years > bestByCategory[cat].years) {
      bestByCategory[cat] = { ...res, ticker };
    }
  }
  for (const [cat, best] of Object.entries(bestByCategory)) {
    rates.upsertRate({
      key: rates.categoryKey(cat),
      label: `${cat} (${best.years} ans)`,
      rate: round2(best.cagr),
      source_url: `https://finance.yahoo.com/quote/${best.ticker}/history/`,
    });
  }

  for (const ticker of tickers) {
    if (categories[ticker]) continue;
    const res = byTicker[ticker];
    if (!res) continue;
    rates.upsertRate({
      key: rates.tickerKey(ticker),
      label: `${ticker} (${res.years} ans)`,
      rate: round2(res.cagr),
      source_url: `https://finance.yahoo.com/quote/${ticker}/history/`,
    });
  }
  await save('portfolio');
}

export async function bootstrapCagrs(bourseTickers) {
  if (bourseTickers.length === 0 || rates.hasCategoryRates()) return;
  await refreshTickerCagrs(bourseTickers);
}

export async function setRateOverride(key, rate) {
  rates.setOverride(key, rate ?? null);
  await save('portfolio');
}

function addYears(date, years) {
  const d = new Date(date.getTime());
  d.setUTCFullYear(d.getUTCFullYear() + years);
  return d;
}

export function computeProjection(now = new Date()) {
  const { portfolio, cache } = getState();
  const prices = {};
  for (const [ticker, p] of Object.entries(cache.ticker_prices)) prices[ticker] = p.price;
  const categories = portfolio.ticker_categories;
  const projAssets = [];
  const holdingValue = (a) => a.holdings.reduce((sum, h) => sum + (prices[h.ticker] || 0) * h.shares, 0);

  const singleAsset = (a, defaultRate) => {
    const r = rates.ensureRate(rates.accountKey(a.type, a.id), a.name, defaultRate);
    const applied = rates.effectiveRate(r);
    return { rateKey: r.key, applied };
  };

  const sorted = [...portfolio.assets].sort((a, b) => a.type.localeCompare(b.type) || a.name.localeCompare(b.name));
  for (const a of sorted) {
    let current = 0;
    let applied = 0;
    let rateKey = '';

    switch (a.type) {
      case 'crypto': {
        current = holdingValue(a);
        projAssets.push({ id: a.id, name: a.name, type: a.type, current, values: flatValues(current), rate_key: '', rate: 0 });
        continue;
      }
      case 'dette': {
        if (!a.dette) continue;
        const d = detteInfo(a.id, a.dette, now);
        const start = parseDate(d.start_date);
        const values = {};
        for (const y of PROJECTION_YEARS) {
          values[y] = -remainingCapital(start, d.duration_months, d.taeg, d.amount_borrowed, addYears(now, y));
        }
        projAssets.push({ id: a.id, name: a.name, type: a.type, current: -d.remaining_capital, values, rate_key: '', rate: 0 });
        continue;
      }
      case 'immobilier':
      case 'fond_euro':
      case 'livret':
      case 'structure': {
        current = a.value;
        ({ rateKey, applied } = singleAsset(a, 0));
        break;
      }
      case 'bourse': {
        let total = 0;
        let weighted = 0;
        let dominantKey = '';
        let dominantValue = 0;
        for (const h of a.holdings) {
          const pos = (prices[h.ticker] || 0) * h.shares;
          total += pos;
          current += pos;
          const cat = categories[h.ticker];
          const key = cat ? rates.categoryKey(cat) : rates.tickerKey(h.ticker);
          const r = rates.getRate(key);
          if (r) {
            weighted += rates.effectiveRate(r) * pos;
            if (pos > dominantValue) {
              dominantValue = pos;
              dominantKey = key;
            }
          }
        }
        if (total > 0) applied = round2(weighted / total);
        rateKey = dominantKey;
        break;
      }
    }

    projAssets.push({
      id: a.id,
      name: a.name,
      type: a.type,
      current,
      values: compoundValues(current, applied),
      rate_key: rateKey,
      rate: applied,
    });
  }

  const usedKeys = new Set(projAssets.map((p) => p.rate_key).filter(Boolean));
  for (const a of portfolio.assets) {
    if (a.type !== 'bourse') continue;
    for (const h of a.holdings) {
      const cat = categories[h.ticker];
      usedKeys.add(cat ? rates.categoryKey(cat) : rates.tickerKey(h.ticker));
    }
  }
  const usedRates = [...usedKeys]
    .map((k) => rates.getRate(k))
    .filter(Boolean)
    .sort((x, y) => (x.key < y.key ? -1 : x.key > y.key ? 1 : 0))
    .map(rates.toApi);

  return { years: PROJECTION_YEARS, assets: projAssets, rates: usedRates };
}
