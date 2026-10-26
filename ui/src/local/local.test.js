import './testenv.js';
import { writes } from './testenv.js';
import test from 'node:test';
import assert from 'node:assert/strict';

const { monthlyPayment, remainingCapital, parseDate } = await import('./finance.js');
const store = await import('./store.js');
const telework = await import('./telework.js');
const walk = await import('./walk.js');
const portfolio = await import('./portfolio.js');
const projection = await import('./projection.js');
const { cagrFromSeries } = await import('./yahoo.js');
const { handle } = await import('./router.js');

function freshState(raw = {}) {
  store.setState(store.normalize(raw));
}

test('monthly payment of a 200k loan at 3.5% over 20 years', () => {
  assert.ok(Math.abs(monthlyPayment(200000, 3.5, 240) - 1159.92) < 0.01);
  assert.equal(monthlyPayment(1200, 0, 12), 100);
});

test('remaining capital: full before first payment, zero when repaid', () => {
  const start = parseDate('2020-01-15');
  assert.equal(remainingCapital(start, 240, 3.5, 200000, new Date(Date.UTC(2020, 1, 1) - 1)), 200000);
  const mid = remainingCapital(start, 240, 3.5, 200000, new Date(Date.UTC(2030, 0, 10)));
  assert.ok(mid > 100000 && mid < 150000, `got ${mid}`);
  assert.equal(remainingCapital(start, 240, 3.5, 200000, new Date(Date.UTC(2041, 0, 1))), 0);
});

test('Geneva holidays', () => {
  const h2025 = Object.fromEntries(telework.genevaHolidays(2025).map((h) => [h.label, h.date]));
  assert.equal(h2025['Pâques'], '2025-04-20');
  assert.equal(h2025['Vendredi Saint'], '2025-04-18');
  assert.equal(h2025['Ascension'], '2025-05-29');
  assert.equal(h2025['Lundi de Pentecôte'], '2025-06-09');
  assert.equal(h2025['Jeûne genevois'], '2025-09-11');
  const h2026 = Object.fromEntries(telework.genevaHolidays(2026).map((h) => [h.label, h.date]));
  assert.equal(h2026['Pâques'], '2026-04-05');
  assert.equal(h2026['Jeûne genevois'], '2026-09-10');
});

test('telework year summary', () => {
  const s = telework.computeYear(2025, { remote_days: [4, 5], overrides: { '2025-01-02': { am: 'leave', pm: 'leave' } } });
  assert.equal(s.days.length, 365);
  assert.equal(s.days[1].am_state, 'leave');
  assert.equal(s.days[0].am_state, ''); // New Year
  assert.equal(s.total_leave, 1);
  assert.ok(s.remote_pct > 35 && s.remote_pct < 45, `remote_pct ${s.remote_pct}`);
  assert.equal(s.total_worked, s.total_remote + s.total_on_site);
});

test('walk year summary', () => {
  const now = new Date(2025, 5, 18); // Wed 18 June 2025, local
  const s = walk.computeYear(2025, { '2025-06-16': 5, '2025-06-17': 2.5, '2025-05-31': 10, '2025-01-01': 1 }, now);
  assert.equal(s.week_total, 7.5);
  assert.equal(s.month_total, 7.5);
  assert.equal(s.year_total, 18.5);
  assert.equal(s.active_days, 4);
  assert.deepEqual(s.best_day, { date: '2025-05-31', km: 10 });
  assert.equal(s.month_totals[4], 10);
  assert.equal(s.days.find((d) => d.date === '2025-06-19').is_future, true);
  assert.equal(s.days.find((d) => d.date === '2025-06-18').is_future, false);
});

test('CAGR from a monthly series', () => {
  const t0 = 1_000_000_000;
  const ts = [t0, t0 + 8760 * 3600 * 10];
  const { cagr, years } = cagrFromSeries(ts, [100, 200]);
  assert.equal(years, 10);
  assert.ok(Math.abs(cagr - 7.177) < 0.01);
  assert.throws(() => cagrFromSeries([t0, t0 + 3600], [1, 2]), /moins d'un an/);
});

const sample = () => ({
  portfolio: {
    assets: [
      { id: 1, type: 'livret', name: 'Livret A', value: 1000, holdings: [] },
      { id: 2, type: 'bourse', name: 'PEA', value: 0, holdings: [{ ticker: 'CW8.PA', shares: 2 }, { ticker: 'AAPL', shares: 1 }] },
      { id: 3, type: 'dette', name: 'Prêt', value: 0, holdings: [], dette: { start_date: '2020-01-01', duration_months: 240, taeg: 3.5, amount_borrowed: 100000 } },
    ],
    ticker_categories: { 'CW8.PA': 'Monde' },
    projection_rates: [{ key: 'category_monde', label: 'Monde (10 ans)', rate: 8, source_url: '', rate_override: null }],
  },
  cache: { ticker_prices: { 'CW8.PA': { price: 500, currency: 'EUR', day_open: 495 }, AAPL: { price: 200, currency: 'USD', day_open: 0 } }, last_refresh: null },
});

test('portfolio summary aggregates values', () => {
  freshState(sample());
  const s = portfolio.summary(new Date(Date.UTC(2025, 0, 10)));
  assert.equal(s.by_category.Monde, 1000);
  assert.equal(s.by_category.AAPL, 200);
  assert.equal(s.account_values[2], 1200);
  assert.ok(s.account_values[3] < 0);
  assert.equal(s.total, 1000 + 1200 + s.account_values[3]);
  assert.ok(Math.abs(s.ticker_day_changes['CW8.PA'] - (5 / 495) * 100) < 1e-9);
  assert.equal(s.ticker_day_changes.AAPL, undefined);
  assert.deepEqual(s.category_rates.map((c) => [c.category, c.rate]), [['Monde', 8]]);
  assert.deepEqual(s.account_rates, [{ asset_id: 1, key: 'livret_1', rate: 0, is_set: false }]);
  assert.equal(s.holdings[2].length, 2);
});

test('asset CRUD, rate override and projection', async () => {
  freshState(sample());
  writes.length = 0;
  const created = await handle('POST', '/api/portfolio/assets', { type: 'livret', name: 'LDDS', value: 500, rate: 3 });
  assert.equal(created.id, 4);
  assert.equal((await handle('GET', '/api/portfolio/summary')).account_rates.find((r) => r.asset_id === 4).rate, 3);
  assert.ok(writes.some((w) => w.name === 'portfolio.json'));

  const proj = projection.computeProjection(new Date(Date.UTC(2025, 0, 10)));
  const ldds = proj.assets.find((a) => a.name === 'LDDS');
  assert.equal(ldds.values[1], 515);
  const pea = proj.assets.find((a) => a.name === 'PEA');
  assert.equal(pea.rate, 6.67); // (8*1000 + 0*200) / 1200, AAPL has no rate
  const loan = proj.assets.find((a) => a.name === 'Prêt');
  assert.ok(loan.values[20] === 0 && loan.current < 0);

  await handle('DELETE', '/api/portfolio/assets/4');
  assert.equal(store.getState().portfolio.projection_rates.some((r) => r.key === 'livret_4'), false);
});

test('holdings and categories through the router', async () => {
  freshState(sample());
  const { id } = await handle('POST', '/api/portfolio/assets/2/holdings', { ticker: ' btc-eur ', shares: 0.5 });
  const list = await handle('GET', '/api/portfolio/assets/2/holdings');
  assert.deepEqual(list.map((h) => h.ticker), ['AAPL', 'BTC-EUR', 'CW8.PA']);
  await handle('PUT', `/api/portfolio/holdings/${id}`, { ticker: 'ETH-EUR', shares: 2 });
  assert.equal(store.getState().portfolio.assets[1].holdings.find((h) => h.id === id).ticker, 'ETH-EUR');
  await handle('DELETE', `/api/portfolio/holdings/${id}`);
  await handle('PUT', '/api/portfolio/tickers/AAPL/category', { category: 'US' });
  assert.equal(store.getState().portfolio.ticker_categories.AAPL, 'US');
  await handle('DELETE', '/api/portfolio/tickers/AAPL/category');
  assert.equal(store.getState().portfolio.ticker_categories.AAPL, undefined);
  await assert.rejects(handle('POST', '/api/portfolio/assets/2/holdings', { ticker: '  ' }), /ticker is required/);
});

test('persisted portfolio drops in-memory holding ids and keeps the export format', () => {
  freshState(sample());
  const out = store.serialize('portfolio', store.getState().portfolio);
  assert.deepEqual(out.assets[1].holdings, [{ ticker: 'CW8.PA', shares: 2 }, { ticker: 'AAPL', shares: 1 }]);
  assert.equal(out.assets[2].dette.duration_months, 240);
  assert.equal(out.assets[0].dette, undefined);
  assert.deepEqual(Object.keys(out), ['assets', 'ticker_categories', 'projection_rates']);
});

test('telework overrides are replaced per year', async () => {
  freshState();
  const y = new Date().getFullYear();
  await handle('PUT', `/api/telework/overrides/${y}`, { [`${y}-03-03`]: { am: 'leave', pm: '' }, [`${y}-03-04`]: { am: '', pm: '' } });
  assert.deepEqual(Object.keys(await handle('GET', `/api/telework/overrides/${y}`)), [`${y}-03-03`]);
  await assert.rejects(handle('PUT', `/api/telework/overrides/${y}`, { [`${y}-03-03`]: { am: 'nope', pm: '' } }), /invalid override type/);
  await assert.rejects(handle('GET', '/api/telework/summary/1999'), /invalid year/);
});

test('walk day validation and clearing', async () => {
  freshState();
  const y = new Date().getFullYear();
  await handle('PUT', '/api/walk/day', { date: `${y}-01-01`, km: 4.5 });
  assert.equal(store.getState().walk.entries[`${y}-01-01`], 4.5);
  await handle('PUT', '/api/walk/day', { date: `${y}-01-01`, km: 0 });
  assert.deepEqual(store.getState().walk.entries, {});
  await assert.rejects(handle('PUT', '/api/walk/day', { date: `${y + 1}-01-01`, km: 3 }), /out of range/);
  await assert.rejects(handle('PUT', '/api/walk/day', { date: `${y}-01-01`, km: 5000 }), /invalid distance/);
});

test('changes are refused while Synchrotron is syncing, reads and the prices refresh stay allowed', async () => {
  const { setReadOnly } = await import('./syncGuard.js');
  freshState();
  setReadOnly(true);
  try {
    await assert.rejects(() => handle('PUT', '/api/walk/day', { date: '2025-01-01', km: 3 }), (e) => e.status === 423);
    await handle('GET', '/api/walk/summary/2025');
  } finally {
    setReadOnly(false);
  }
  await handle('PUT', '/api/walk/day', { date: '2025-01-01', km: 3 });
});

test('mapLimited keeps the order, settles every item and never exceeds the limit', async () => {
  const { mapLimited } = await import('./yahoo.js');
  let running = 0;
  let peak = 0;
  const results = await mapLimited([1, 2, 3, 4, 5, 6, 7], async (n) => {
    running++;
    peak = Math.max(peak, running);
    await new Promise((r) => setTimeout(r, 5));
    running--;
    if (n === 3) throw new Error('boom');
    return n * 10;
  }, 3);
  assert.ok(peak <= 3);
  assert.deepEqual(results.map((r) => r.status), ['fulfilled', 'fulfilled', 'rejected', 'fulfilled', 'fulfilled', 'fulfilled', 'fulfilled']);
  assert.equal(results[1].value, 20);
  assert.equal(results[2].reason.message, 'boom');
  assert.deepEqual(await mapLimited([], async () => 1), []);
});
