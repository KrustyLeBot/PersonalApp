// OpenF1 API client. OpenF1 dropped browser CORS support, so we go through our
// own backend relay (/api/f1/openf1/<endpoint>) instead of hitting
// api.openf1.org directly — the server forwards the request and the query string
// verbatim. We only use the free historical REST endpoints (no MQTT/WebSocket).

const BASE = '/api/f1/openf1';

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

// Returns the *Race* session for a given calendar date ("YYYY-MM-DD"). Used to
// replay a finished Grand Prix from its results page: OpenF1 indexes sessions by
// date_start, so we match the race day and pick the Race session of that meeting.
export async function fetchRaceSessionByDate(dateStr) {
  const day = (dateStr ?? '').slice(0, 10);
  if (!day) return null;
  const year = Number(day.slice(0, 4));
  // OpenF1 supports date filtering on date_start; bound the query to the race day.
  const rows = await get('/sessions', {
    year,
    'date_start>': `${day}T00:00:00`,
    'date_start<': `${day}T23:59:59`,
  });
  if (!rows.length) return null;
  return rows.find(s => s.session_type === 'Race') ?? rows[rows.length - 1];
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
// `anchorMs` is the window end — a past instant, since we replay finished races.
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
