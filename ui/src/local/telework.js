import { getState, save } from './store.js';
import { HttpError } from './errors.js';

const HALF_TYPES = ['leave', 'remote', 'office'];
const ymd = (d) => d.toISOString().slice(0, 10);
const utc = (y, m, d) => new Date(Date.UTC(y, m - 1, d));
const addDays = (d, n) => new Date(d.getTime() + n * 86400000);

// Easter Sunday (Meeus/Jones/Butcher).
export function easterDate(year) {
  const a = year % 19;
  const b = Math.floor(year / 100);
  const c = year % 100;
  const d = Math.floor(b / 4);
  const e = b % 4;
  const f = Math.floor((b + 8) / 25);
  const g = Math.floor((b - f + 1) / 3);
  const h = (19 * a + b - d - g + 15) % 30;
  const i = Math.floor(c / 4);
  const k = c % 4;
  const l = (32 + 2 * e + 2 * i - h - k) % 7;
  const m = Math.floor((a + 11 * h + 22 * l) / 451);
  const month = Math.floor((h + l - 7 * m + 114) / 31);
  const day = ((h + l - 7 * m + 114) % 31) + 1;
  return utc(year, month, day);
}

// Thursday after the first Sunday of September.
export function jeuneGenevois(year) {
  const sep1 = utc(year, 9, 1);
  const untilSunday = (7 - sep1.getUTCDay()) % 7;
  return addDays(sep1, untilSunday + 4);
}

export function genevaHolidays(year) {
  const easter = easterDate(year);
  return [
    { date: ymd(utc(year, 1, 1)), label: 'Nouvel An' },
    { date: ymd(utc(year, 12, 25)), label: 'Noël' },
    { date: ymd(utc(year, 12, 31)), label: 'Restauration genevoise' },
    { date: ymd(utc(year, 8, 1)), label: 'Fête nationale' },
    { date: ymd(addDays(easter, -2)), label: 'Vendredi Saint' },
    { date: ymd(easter), label: 'Pâques' },
    { date: ymd(addDays(easter, 1)), label: 'Lundi de Pâques' },
    { date: ymd(addDays(easter, 39)), label: 'Ascension' },
    { date: ymd(addDays(easter, 49)), label: 'Pentecôte' },
    { date: ymd(addDays(easter, 50)), label: 'Lundi de Pentecôte' },
    { date: ymd(jeuneGenevois(year)), label: 'Jeûne genevois' },
  ];
}

function checkYear(year) {
  const current = new Date().getFullYear();
  if (!Number.isInteger(year) || year < current - 5 || year > current) throw new HttpError(400, 'invalid year');
}

function effectiveHalf(override, presetRemote) {
  if (override) return override;
  return presetRemote ? 'remote' : 'office';
}

export function computeYear(year, { remote_days, overrides }) {
  const holidays = genevaHolidays(year);
  const holidaySet = new Set(holidays.map((h) => h.date));
  const remoteSet = new Set(remote_days);

  const days = [];
  let worked = 0;
  let remote = 0;
  let leave = 0;
  for (let d = utc(year, 1, 1); d.getUTCFullYear() === year; d = addDays(d, 1)) {
    const date = ymd(d);
    const weekday = d.getUTCDay();
    const isWeekend = weekday === 0 || weekday === 6;
    const isHoliday = holidaySet.has(date);
    const ov = overrides[date] || { am: '', pm: '' };

    let amState = '';
    let pmState = '';
    if (!isWeekend && !isHoliday) {
      amState = effectiveHalf(ov.am, remoteSet.has(weekday));
      pmState = effectiveHalf(ov.pm, remoteSet.has(weekday));
      for (const st of [amState, pmState]) {
        if (st === 'leave') leave++;
        else if (st === 'remote') {
          worked++;
          remote++;
        } else if (st === 'office') worked++;
      }
    }
    days.push({
      date,
      weekday,
      is_weekend: isWeekend,
      is_holiday: isHoliday,
      am_state: amState,
      pm_state: pmState,
      am_override: ov.am || '',
      pm_override: ov.pm || '',
    });
  }

  const totalWorked = worked / 2;
  const totalRemote = remote / 2;
  let remotePct = 0;
  let onSitePct = 0;
  if (worked > 0) {
    remotePct = Math.round((remote / worked) * 10000) / 100;
    onSitePct = Math.round(((worked - remote) / worked) * 10000) / 100;
  }
  const overThreshold = remotePct > 40;

  // Halves to convert to on-site so that remote ≤ 40% of worked halves.
  let daysToRecover = 0;
  if (overThreshold && worked > 0) {
    daysToRecover = Math.max(0, Math.ceil(remote - 0.4 * worked)) / 2;
  }

  return {
    year,
    days,
    holidays,
    total_worked: totalWorked,
    total_remote: totalRemote,
    total_on_site: totalWorked - totalRemote,
    total_leave: leave / 2,
    remote_pct: remotePct,
    on_site_pct: onSitePct,
    days_to_recover: daysToRecover,
    over_threshold: overThreshold,
  };
}

export function summary(year) {
  checkYear(year);
  return computeYear(year, getState().telework);
}

export function getPreset() {
  return { remote_days: getState().telework.remote_days };
}

export async function savePreset(body) {
  if (!Array.isArray(body.remote_days)) throw new HttpError(400, 'invalid body');
  getState().telework.remote_days = body.remote_days.map(Number);
  await save('telework');
}

export function getOverrides(year) {
  checkYear(year);
  const out = {};
  for (const [date, ov] of Object.entries(getState().telework.overrides)) {
    if (date.startsWith(`${year}-`)) out[date] = { date, am: ov.am, pm: ov.pm };
  }
  return out;
}

// Replaces every override of the year. An entry whose halves are both empty
// means "follow the preset" and is dropped.
export async function setOverrides(year, body) {
  checkYear(year);
  for (const ov of Object.values(body)) {
    for (const t of [ov.am, ov.pm]) {
      if (t && !HALF_TYPES.includes(t)) throw new HttpError(400, `invalid override type: ${t}`);
    }
  }
  const tw = getState().telework;
  for (const date of Object.keys(tw.overrides)) {
    if (date.startsWith(`${year}-`)) delete tw.overrides[date];
  }
  for (const [date, ov] of Object.entries(body)) {
    if (!ov.am && !ov.pm) continue;
    tw.overrides[date] = { am: ov.am || '', pm: ov.pm || '' };
  }
  await save('telework');
}
