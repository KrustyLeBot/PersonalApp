import { platform } from '../platform/index.js';

// Yahoo rejects requests without a browser-like User-Agent.
const HEADERS = {
  'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36',
  Accept: 'application/json',
};

// Yahoo throttles bursts (HTTP 429) and sometimes only one of its two hosts answers.
const HOSTS = ['query1.finance.yahoo.com', 'query2.finance.yahoo.com'];

// Runs fn over items, at most `limit` at a time; settles like Promise.allSettled, in the same order.
export async function mapLimited(items, fn, limit = 4) {
  const results = new Array(items.length);
  let next = 0;
  async function worker() {
    while (next < items.length) {
      const i = next++;
      try {
        results[i] = { status: 'fulfilled', value: await fn(items[i]) };
      } catch (reason) {
        results[i] = { status: 'rejected', reason };
      }
    }
  }
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, worker));
  return results;
}

async function chartFrom(host, ticker, query) {
  const url = `https://${host}/v8/finance/chart/${encodeURIComponent(ticker)}?${query}`;
  const res = await platform.httpGet(url, HEADERS);
  let parsed;
  try {
    parsed = JSON.parse(res.body);
  } catch {
    throw new Error(`réponse illisible pour ${ticker} (HTTP ${res.status})`);
  }
  if (parsed.chart?.error) throw new Error(`yahoo ${ticker}: ${parsed.chart.error.description}`);
  const result = parsed.chart?.result?.[0];
  if (!result) throw new Error(`aucune donnée pour ${ticker}`);
  return result;
}

async function chart(ticker, query) {
  let failure;
  for (const host of HOSTS) {
    try {
      return await chartFrom(host, ticker, query);
    } catch (e) {
      failure ??= e;
      // A ticker Yahoo does not know will not be known by the other host either.
      if (/not found|No data found/i.test(e.message)) break;
    }
  }
  throw failure;
}

// Current price plus the day's first quotation (for the intraday variation).
export async function fetchPrice(ticker) {
  const result = await chart(ticker, 'range=1d&interval=1d');
  return {
    ticker,
    price: result.meta.regularMarketPrice,
    currency: result.meta.currency,
    day_open: result.indicators?.quote?.[0]?.open?.[0] || 0,
  };
}

// Pure part of the CAGR computation, split out so it can be tested.
export function cagrFromSeries(timestamps, closes, ticker = '') {
  if (!timestamps || !closes || timestamps.length < 2 || closes.length < 2) {
    throw new Error(`données insuffisantes pour ${ticker}`);
  }
  let first = -1;
  let last = -1;
  closes.forEach((v, i) => {
    if (v > 0) {
      if (first === -1) first = i;
      last = i;
    }
  });
  if (first === -1 || first === last) throw new Error(`aucun prix valide pour ${ticker}`);

  const years = (timestamps[last] - timestamps[first]) / 3600 / 8760;
  if (years < 1) throw new Error(`moins d'un an d'historique pour ${ticker}`);

  const cagr = (Math.pow(closes[last] / closes[first], 1 / years) - 1) * 100;
  return { cagr, years: Math.round(years) };
}

// Annualised growth over the maximum available history (monthly points).
export async function fetchCagr(ticker) {
  const result = await chart(ticker, 'range=max&interval=1mo');
  return cagrFromSeries(result.timestamp, result.indicators?.quote?.[0]?.close, ticker);
}
