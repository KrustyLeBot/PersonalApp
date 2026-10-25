import { getState, save } from './store.js';
import { HttpError } from './errors.js';
import { round2 } from './finance.js';

const ymd = (d) => d.toISOString().slice(0, 10);
const utc = (y, m, d) => new Date(Date.UTC(y, m - 1, d));
const addDays = (d, n) => new Date(d.getTime() + n * 86400000);

function localToday(now) {
  return utc(now.getFullYear(), now.getMonth() + 1, now.getDate());
}

function sumRange(entries, from, to) {
  let total = 0;
  for (const [date, km] of Object.entries(entries)) if (date >= from && date <= to) total += km;
  return total;
}

export function computeYear(year, entries, now = new Date()) {
  const today = localToday(now);
  const days = [];
  const monthTotals = new Array(12).fill(0);
  let yearTotal = 0;
  let activeDays = 0;
  let best = { date: '', km: 0 };

  for (let d = utc(year, 1, 1); d.getUTCFullYear() === year; d = addDays(d, 1)) {
    const date = ymd(d);
    const km = entries[date] || 0;
    if (km > 0) {
      yearTotal += km;
      monthTotals[d.getUTCMonth()] += km;
      activeDays++;
      if (km > best.km) best = { date, km };
    }
    days.push({ date, weekday: d.getUTCDay(), km, is_future: d > today });
  }

  // Week and month totals describe the current calendar week/month, even across a year boundary.
  const weekStart = addDays(today, -((today.getUTCDay() + 6) % 7));
  const monthStart = utc(today.getUTCFullYear(), today.getUTCMonth() + 1, 1);
  const monthEnd = utc(today.getUTCFullYear(), today.getUTCMonth() + 2, 0);
  const weekTotal = sumRange(entries, ymd(weekStart), ymd(addDays(weekStart, 6)));
  const monthTotal = sumRange(entries, ymd(monthStart), ymd(monthEnd));

  return {
    year,
    days,
    week_total: round2(weekTotal),
    month_total: round2(monthTotal),
    year_total: round2(yearTotal),
    month_totals: monthTotals.map(round2),
    active_days: activeDays,
    daily_avg: activeDays > 0 ? round2(yearTotal / activeDays) : 0,
    best_day: best,
  };
}

export function summary(year) {
  const current = new Date().getFullYear();
  if (!Number.isInteger(year) || year < current - 5 || year > current) throw new HttpError(400, 'invalid year');
  return computeYear(year, getState().walk.entries);
}

// Sets one day's distance; km <= 0 clears the entry.
export async function setDay(body) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(body.date || '')) throw new HttpError(400, 'invalid date');
  const [y, m, d] = body.date.split('-').map(Number);
  const date = utc(y, m, d);
  const now = new Date();
  if (Number.isNaN(date.getTime()) || ymd(date) !== body.date) throw new HttpError(400, 'invalid date');
  if (y < now.getFullYear() - 5 || date > localToday(now)) throw new HttpError(400, 'date out of range');
  const km = Number(body.km);
  if (!(km >= 0 && km <= 1000)) throw new HttpError(400, 'invalid distance');

  const { entries } = getState().walk;
  if (km <= 0) delete entries[body.date];
  else entries[body.date] = km;
  await save('walk');
}
