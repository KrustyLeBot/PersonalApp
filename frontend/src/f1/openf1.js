// OpenF1 API client — public API (api.openf1.org), no key, CORS-enabled.
// All live race data is fetched client-side. Historical data is free; we only
// use the free REST endpoints here (no MQTT/WebSocket streaming).

const BASE = 'https://api.openf1.org/v1';

// OpenF1 free tier allows 30 requests/MINUTE (= 0.5 req/s). We serialize all
// requests through a single queue with a minimum spacing so bursts (initial
// load, multi-endpoint refresh) never exceed the limit. 2100ms base spacing →
// ~28 req/min, safely under 30.
const BASE_SPACING_MS = 2100;
const MAX_SPACING_MS = 30000;
let queueTail = Promise.resolve();
let lastStart = 0;

// Dynamic spacing: grows on 429 (exponential backoff), shrinks back toward the
// base on success. While rate-limited, every queued call waits longer, so the
// whole app naturally throttles until OpenF1 stops returning 429.
let spacing = BASE_SPACING_MS;

function onRateLimited() {
  spacing = Math.min(MAX_SPACING_MS, Math.max(spacing * 2, 1000));
}
function onSuccess() {
  // Ease back gradually so we don't immediately slam the limit again.
  if (spacing > BASE_SPACING_MS) spacing = Math.max(BASE_SPACING_MS, spacing * 0.8);
}

function schedule(fn) {
  const run = queueTail.then(async () => {
    const wait = Math.max(0, lastStart + spacing - Date.now());
    if (wait > 0) await new Promise(r => setTimeout(r, wait));
    lastStart = Date.now();
    return fn();
  });
  // Keep the chain alive even if a call rejects.
  queueTail = run.catch(() => {});
  return run;
}

async function get(path, params = {}) {
  return schedule(async () => {
    const qs = new URLSearchParams(params).toString();
    const res = await fetch(`${BASE}${path}?${qs}`);
    if (res.status === 429) {
      onRateLimited();
      throw new Error(`OpenF1 ${path}: rate limited`);
    }
    if (!res.ok) throw new Error(`OpenF1 ${path}: HTTP ${res.status}`);
    onSuccess();
    return res.json();
  });
}

// --- Sessions ---

// Returns the session object for the given key (or 'latest').
export async function fetchSession(sessionKey = 'latest') {
  const rows = await get('/sessions', { session_key: sessionKey });
  return rows[rows.length - 1] ?? null;
}

// Returns the latest *Race* session of the most recent meeting — used for the
// static demo so we land on a race rather than a practice/qualifying session.
export async function fetchLatestRaceSession() {
  const latest = await fetchSession('latest');
  if (!latest) return null;
  const rows = await get('/sessions', { meeting_key: latest.meeting_key });
  return rows.find(s => s.session_type === 'Race') ?? latest;
}

// --- Drivers ---

// Returns a map: driver_number -> { number, acronym, fullName, team, color }
export async function fetchDrivers(sessionKey) {
  const rows = await get('/drivers', { session_key: sessionKey });
  const map = {};
  for (const d of rows) {
    map[d.driver_number] = {
      number: d.driver_number,
      acronym: d.name_acronym,
      fullName: d.full_name,
      team: d.team_name,
      color: d.team_colour ? `#${d.team_colour}` : '#94a3b8',
    };
  }
  return map;
}

// --- Live classification ---

// Latest position per driver. OpenF1 returns the full history, so we keep the
// last record per driver_number.
export async function fetchPositions(sessionKey) {
  const rows = await get('/position', { session_key: sessionKey });
  const latest = {};
  for (const r of rows) {
    if (r.position == null) continue;
    latest[r.driver_number] = r.position;
  }
  return latest; // { driver_number: position }
}

// Latest interval/gap per driver.
export async function fetchIntervals(sessionKey) {
  const rows = await get('/intervals', { session_key: sessionKey });
  const latest = {};
  for (const r of rows) {
    latest[r.driver_number] = { gapToLeader: r.gap_to_leader, interval: r.interval };
  }
  return latest; // { driver_number: { gapToLeader, interval } }
}

// Latest completed lap per driver (lap_number + lap_duration).
export async function fetchLatestLaps(sessionKey) {
  const rows = await get('/laps', { session_key: sessionKey });
  const latest = {};
  for (const r of rows) {
    const cur = latest[r.driver_number];
    if (!cur || r.lap_number > cur.lapNumber) {
      latest[r.driver_number] = { lapNumber: r.lap_number, lapDuration: r.lap_duration };
    }
  }
  return latest; // { driver_number: { lapNumber, lapDuration } }
}

// --- Track positions (telemetry x/y) ---

// ALL x/y samples per driver in a time window — not just the latest. The caller
// buffers these on a timeline and renders with a fixed delay, interpolating
// between samples for smooth 60fps motion despite the slow (~4s) poll rate.
//
// OpenF1 /location is sampled ~3.7Hz; we request a window slightly larger than
// the poll interval so consecutive fetches overlap and no samples are missed.
// `anchorMs` is the window end: Date.now() live, or a past instant for the demo.
export async function fetchLocations(sessionKey, anchorMs = Date.now(), windowSec = 15) {
  const since = new Date(anchorMs - windowSec * 1000).toISOString();
  const until = new Date(anchorMs).toISOString();
  const rows = await get('/location', {
    session_key: sessionKey,
    'date>': since,
    'date<': until,
  });
  const byDriver = {};
  for (const r of rows) {
    if (r.x === 0 && r.y === 0) continue; // skip uninitialised samples
    (byDriver[r.driver_number] ??= []).push({
      x: r.x, y: r.y, t: new Date(r.date).getTime(),
    });
  }
  // Ensure each driver's samples are time-ordered for the interpolator.
  for (const num in byDriver) byDriver[num].sort((a, b) => a.t - b.t);
  return byDriver; // { driver_number: [{ x, y, t }...] }
}

// Builds the track shape from telemetry: one driver's first ~2 minutes of
// /location samples trace a full lap. Because the outline and the live cars use
// the SAME telemetry coordinate space, cars align perfectly with the track —
// no GPS projection, no per-circuit rotation to guess. Returns [{x,y}...].
export async function fetchTrackOutline(sessionKey, dateStart) {
  // A lap is ~90s; grab a 3-min window from the green flag to guarantee a full
  // loop, using one driver to keep the response small.
  const start = new Date(dateStart).getTime();
  const since = new Date(start).toISOString();
  const until = new Date(start + 180_000).toISOString();
  const rows = await get('/location', {
    session_key: sessionKey,
    driver_number: 1,
    'date>': since,
    'date<': until,
  });
  const pts = rows
    .filter(r => !(r.x === 0 && r.y === 0))
    .map(r => ({ x: r.x, y: r.y }));
  return downsample(pts, 400);
}

// Evenly thins an ordered point list to at most `max` points.
function downsample(pts, max) {
  if (pts.length <= max) return pts;
  const step = pts.length / max;
  const out = [];
  for (let i = 0; i < pts.length; i += step) out.push(pts[Math.floor(i)]);
  return out;
}

// --- Race control (flags / safety car) ---

// Returns the most recent track-wide flag/safety-car state.
export async function fetchRaceControl(sessionKey) {
  const rows = await get('/race_control', { session_key: sessionKey });
  let flag = null;
  for (const r of rows) {
    if (r.category === 'SafetyCar') {
      flag = { type: 'SC', message: r.message, date: r.date };
    } else if (r.flag && r.scope === 'Track') {
      flag = { type: r.flag, message: r.message, date: r.date };
    }
  }
  return flag; // { type, message, date } | null
}
