<script>
  import { onMount, onDestroy } from 'svelte';
  import TrackMap from './TrackMap.svelte';
  import {
    fetchRaceSessionByDate, fetchDrivers,
    fetchPositions, fetchIntervals, fetchLatestLaps,
    fetchLocations, fetchRaceControl, fetchTrackOutline,
  } from './openf1.js';

  // Replay a finished Grand Prix's Race session as if it were live: we resolve
  // the session from the race date, then run a clock shifted into the past so the
  // historical telemetry plays back at 1× — identical feel to a live session.
  // OpenF1's free tier only serves historical data, so live (in-progress) sessions
  // aren't available; replaying a past race gives the same live UI for free.
  export let race; // { season, round, raceDate, raceName, ... }

  let session = null;
  let drivers = {};       // number -> driver info
  let positions = {};     // number -> position
  let intervals = {};     // number -> { gapToLeader, interval }
  let laps = {};          // number -> { lapNumber, lapDuration }
  let trackOutline = [];  // [{x,y}...] one lap of telemetry → the track shape
  let flag = null;
  let loading = true;
  let error = '';

  // --- Smooth playback buffer ---
  // We poll /location every few seconds but each response carries a window of
  // timestamped samples. We append them to a per-driver timeline, then render
  // RENDER_DELAY_MS in the past, interpolating between the two samples that
  // bracket the playback clock. Trade-off: a few seconds of latency for fully
  // smooth 60fps motion instead of cars teleporting every poll.
  // Render well behind real time so the buffer always has samples ahead of the
  // playback clock, even if several polls are delayed by the 30 req/min budget
  // or a 429 backoff. Big margin = no freezes, at the cost of more latency.
  const RENDER_DELAY_MS = 12000;
  const BUFFER_KEEP_MS = 40000; // keep a deep history so reads never run dry
  let buffer = {};        // number -> [{ x, y, t }...] (time-ordered)
  let carPositions = {};  // number -> { x, y } interpolated, updated each frame
  let rafId = null;

  // Initial-load checklist shown in the spinner. Each step flips to 'done' (or
  // 'fail') as its API call resolves, so the user sees progress instead of a
  // blank spinner — and which call is the slow one under the 30 req/min budget.
  const STEP_LABELS = {
    session:        'Session',
    drivers:        'Pilotes',
    track:          'Tracé du circuit',
    classification: 'Classement',
    locations:      'Positions',
    slow:           'Tours & drapeaux',
  };
  let steps = {
    session: 'pending', drivers: 'pending', track: 'pending',
    classification: 'pending', locations: 'pending', slow: 'pending',
  };
  function mark(step, state) { steps[step] = state; steps = steps; }

  // Replay clock: maps real wall-clock time onto the past race at 1× speed.
  // The anchor is always `Date.now() - replayOffsetMs`, so it advances in real
  // time — identical feel to a live session, just shifted into the past race.
  let replayOffsetMs = 0;

  // Start the replay this far into the race: lap 1 chaos is over, the field has
  // settled and gaps are meaningful.
  const REPLAY_START_OFFSET_MS = 120_000; // minute 2

  // Staggered polling cadences (ms). Requests go through openf1.js's throttle
  // queue (≥400ms apart), so even when timers coincide we stay under 3 req/s.
  let timers = [];

  // Set when the component is destroyed (e.g. user closes the replay). The
  // sequential init below can be mid-flight when that happens; we check this flag
  // after each await so no further fetch fires once the component is gone.
  let destroyed = false;

  onMount(async () => {
    try {
      session = await fetchRaceSessionByDate(race?.raceDate);
      if (destroyed) return;
      if (!session) throw new Error('Replay indisponible pour cette course');
      mark('session', 'done');

      drivers = await fetchDrivers(session.session_key);
      if (destroyed) return;
      mark('drivers', 'done');

      // Start the replay at minute 2; the fixed offset keeps the replay clock
      // tracking real time at 1× speed.
      const replayStart = new Date(session.date_start).getTime() + REPLAY_START_OFFSET_MS;
      replayOffsetMs = Date.now() - replayStart;

      // Build the track shape from one lap of telemetry — same coordinate space
      // as the cars, so positions align perfectly (no GPS/rotation mismatch).
      trackOutline = await fetchTrackOutline(session.session_key, session.date_start)
        .catch(() => []);
      if (destroyed) return;
      mark('track', trackOutline.length ? 'done' : 'fail');

      // Initial load: sequential (queue spaces them out) so the first paint
      // doesn't fire a burst that trips the rate limit.
      await refreshClassification();
      if (destroyed) return;
      mark('classification', Object.keys(positions).length ? 'done' : 'fail');
      await refreshLocations();
      if (destroyed) return;
      mark('locations', Object.keys(buffer).length ? 'done' : 'fail');
      await refreshSlow();
      if (destroyed) return;
      mark('slow', 'done');
      loading = false;

      // Start the smooth playback loop.
      rafId = requestAnimationFrame(tick);

      // OpenF1 free tier is 30 req/min. Budget (≈29/min): map every 4s (15/min),
      // classification every 12s (~10/min, 2 calls), flags every 30s (~4/min,
      // 2 calls). The clock is anchored in the past, so every poll fetches the
      // replay window — same cadence and feel as a live session.
      timers.push(setInterval(refreshLocations, 4000));
      timers.push(setInterval(refreshClassification, 12000));
      timers.push(setInterval(refreshSlow, 30000));
    } catch (e) {
      if (!destroyed) error = e.message;
      loading = false;
    }
  });

  onDestroy(() => {
    destroyed = true;
    timers.forEach(clearInterval);
    if (rafId) cancelAnimationFrame(rafId);
  });

  function anchor() {
    return Date.now() - replayOffsetMs;
  }

  // Append a freshly fetched window of samples into each driver's timeline,
  // de-duplicating by timestamp and trimming anything too old to matter.
  function ingestLocations(byDriver) {
    const cutoff = anchor() - BUFFER_KEEP_MS;
    for (const num in byDriver) {
      const tl = buffer[num] ??= [];
      const lastT = tl.length ? tl[tl.length - 1].t : -Infinity;
      for (const s of byDriver[num]) {
        if (s.t > lastT) tl.push(s); // only genuinely newer samples
      }
      // Trim old samples.
      let i = 0;
      while (i < tl.length && tl[i].t < cutoff) i++;
      if (i > 0) tl.splice(0, i);
    }
    buffer = buffer;
  }

  // Playback loop: for each driver, find where they were RENDER_DELAY_MS ago and
  // linearly interpolate between the bracketing samples. Runs every frame.
  function tick() {
    const playClock = anchor() - RENDER_DELAY_MS;
    const out = {};
    for (const num in buffer) {
      const tl = buffer[num];
      if (tl.length === 0) continue;
      out[num] = sampleAt(tl, playClock);
    }
    carPositions = out;
    rafId = requestAnimationFrame(tick);
  }

  // Linear interpolation of a time-ordered [{x,y,t}] timeline at time `t`.
  function sampleAt(tl, t) {
    if (t <= tl[0].t) return { x: tl[0].x, y: tl[0].y };
    const last = tl[tl.length - 1];
    if (t >= last.t) return { x: last.x, y: last.y };
    // Binary search for the bracketing pair.
    let lo = 0, hi = tl.length - 1;
    while (hi - lo > 1) {
      const mid = (lo + hi) >> 1;
      if (tl[mid].t <= t) lo = mid; else hi = mid;
    }
    const a = tl[lo], b = tl[hi];
    const f = (t - a.t) / (b.t - a.t || 1);
    return { x: a.x + (b.x - a.x) * f, y: a.y + (b.y - a.y) * f };
  }

  async function refreshClassification() {
    const key = session.session_key;
    positions = await fetchPositions(key).catch(() => positions);
    intervals = await fetchIntervals(key).catch(() => intervals);
  }

  async function refreshLocations() {
    const byDriver = await fetchLocations(session.session_key, anchor()).catch(() => null);
    if (byDriver) ingestLocations(byDriver);
  }

  async function refreshSlow() {
    const key = session.session_key;
    laps = await fetchLatestLaps(key).catch(() => laps);
    flag = await fetchRaceControl(key).catch(() => flag);
  }

  function fmtGap(num) {
    if (num == null) return '';
    // OpenF1 sends a string like "+1 LAP" for lapped cars; pass it through.
    if (typeof num === 'string') return num;
    if (typeof num !== 'number' || Number.isNaN(num)) return '';
    if (num === 0) return '—';
    return `+${num.toFixed(3)}`;
  }

  function fmtLap(sec) {
    if (typeof sec !== 'number' || Number.isNaN(sec)) return '';
    const m = Math.floor(sec / 60);
    const s = (sec % 60).toFixed(3).padStart(6, '0');
    return `${m}:${s}`;
  }

  // Build the sorted classification from positions + driver/interval/lap data.
  $: classification = Object.entries(positions)
    .map(([num, position]) => {
      const n = Number(num);
      const d = drivers[n] ?? { number: n, acronym: String(n), team: '', color: '#94a3b8' };
      return {
        ...d,
        position,
        gapToLeader: intervals[n]?.gapToLeader,
        interval: intervals[n]?.interval,
        lap: laps[n],
      };
    })
    .sort((a, b) => a.position - b.position);

  // Cars for the map use the interpolated playback positions (updated each
  // frame), so motion is smooth even though /location is polled every few sec.
  $: cars = classification
    .filter(c => carPositions[c.number])
    .map(c => ({
      number: c.number,
      acronym: c.acronym,
      color: c.color,
      position: c.position,
      x: carPositions[c.number].x,
      y: carPositions[c.number].y,
    }));

  $: leadLap = classification.reduce((m, c) => Math.max(m, c.lap?.lapNumber ?? 0), 0);

  function flagLabel(f) {
    if (!f) return null;
    if (f.type === 'SC') return { text: 'Safety Car', cls: 'sc' };
    if (f.type === 'VSC') return { text: 'Virtual SC', cls: 'sc' };
    if (f.type === 'RED') return { text: 'Drapeau rouge', cls: 'red' };
    if (f.type === 'YELLOW') return { text: 'Drapeau jaune', cls: 'yellow' };
    if (f.type === 'CHEQUERED') return { text: 'Damier', cls: 'chequered' };
    if (f.type === 'GREEN' || f.type === 'CLEAR') return { text: 'Piste dégagée', cls: 'green' };
    return { text: f.type, cls: '' };
  }
  $: flagState = flagLabel(flag);
</script>

<div class="live">
  <div class="replay-banner">
    Replay {race?.raceName ? `du ${race.raceName}` : 'de la course'} — rejoué comme en direct depuis la minute 2
  </div>

  {#if loading}
    <div class="loading-checklist">
      <div class="spinner"></div>
      <ul class="checklist">
        {#each Object.entries(STEP_LABELS) as [key, label]}
          <li class="check-item {steps[key]}">
            <span class="check-icon">
              {#if steps[key] === 'done'}✓{:else if steps[key] === 'fail'}✕{:else}○{/if}
            </span>
            {label}
          </li>
        {/each}
      </ul>
    </div>
  {:else if error}
    <div class="state-msg error">{error}</div>
  {:else}
    <!-- Status bar -->
    <div class="status-bar">
      <div class="session-name">
        <span class="live-dot"></span>
        {session.circuit_short_name} · {session.session_name}
      </div>
      <div class="status-right">
        {#if leadLap > 0}<span class="lap-count">Tour {leadLap}</span>{/if}
        {#if flagState}<span class="flag-badge {flagState.cls}">{flagState.text}</span>{/if}
      </div>
    </div>

    <!-- Track map -->
    {#if cars.length > 0}
      <TrackMap outline={trackOutline} {cars} />
    {:else}
      <div class="no-map">Positions des voitures indisponibles pour cette session.</div>
    {/if}

    <!-- Live classification -->
    <div class="class-table-wrap">
      <table class="class-table">
        <thead>
          <tr><th>Pos</th><th>Pilote</th><th>Écurie</th><th>Écart</th><th>Intervalle</th><th>Dernier tour</th></tr>
        </thead>
        <tbody>
          {#each classification as c (c.number)}
            <tr>
              <td class="col-pos">{c.position}</td>
              <td class="col-driver">
                <span class="driver-code" style="border-left:3px solid {c.color}">{c.acronym}</span>
              </td>
              <td class="col-team" style="color:{c.color}">{c.team}</td>
              <td class="col-gap">{c.position === 1 ? 'Leader' : fmtGap(c.gapToLeader)}</td>
              <td class="col-itv">{c.position === 1 ? '' : fmtGap(c.interval)}</td>
              <td class="col-lap">{c.lap ? fmtLap(c.lap.lapDuration) : ''}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .live { display: flex; flex-direction: column; gap: 1rem; }

  .state-msg { text-align: center; padding: 3rem; color: #64748b; font-size: .95rem; }
  .state-msg.error { color: #f87171; }

  /* Loading checklist */
  .loading-checklist {
    display: flex; flex-direction: column; align-items: center; gap: 1.25rem;
    padding: 2.5rem 1rem;
  }
  .spinner {
    width: 32px; height: 32px; border-radius: 50%;
    border: 3px solid #1e293b; border-top-color: #e10600;
    animation: spin .8s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }
  .checklist { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: .45rem; }
  .check-item {
    display: flex; align-items: center; gap: .6rem;
    font-size: .85rem; color: #475569; transition: color .2s;
  }
  .check-item.done { color: #cbd5e1; }
  .check-item.fail { color: #94a3b8; }
  .check-icon {
    display: inline-flex; align-items: center; justify-content: center;
    width: 18px; height: 18px; border-radius: 50%;
    font-size: .7rem; font-weight: 800; flex-shrink: 0;
    background: #1e293b; color: #475569;
  }
  .check-item.done .check-icon { background: #14532d; color: #4ade80; }
  .check-item.fail .check-icon { background: #3f1d1d; color: #f87171; }
  .check-item.pending .check-icon { animation: blink 1.2s infinite; }
  @keyframes blink { 0%,100% { opacity: .4; } 50% { opacity: 1; } }

  .replay-banner {
    font-size: .78rem; color: #fbbf24; background: #1f1a05;
    border: 1px solid #78500a; border-radius: 8px; padding: .5rem .8rem;
  }

  .status-bar {
    display: flex; align-items: center; justify-content: space-between;
    gap: 1rem; flex-wrap: wrap;
  }
  .session-name {
    display: flex; align-items: center; gap: .5rem;
    font-size: .95rem; font-weight: 600; color: #f1f5f9;
  }
  .live-dot {
    width: 9px; height: 9px; border-radius: 50%; background: #ef4444;
    animation: pulse 1.4s infinite;
  }
  @keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: .3; } }

  .status-right { display: flex; align-items: center; gap: .6rem; }
  .lap-count { font-size: .85rem; color: #94a3b8; font-weight: 600; }
  .flag-badge {
    font-size: .72rem; font-weight: 700; padding: .25rem .6rem; border-radius: 6px;
    text-transform: uppercase; letter-spacing: .04em;
  }
  .flag-badge.sc { background: #78500a; color: #fcd34d; }
  .flag-badge.red { background: #7f1d1d; color: #fca5a5; }
  .flag-badge.yellow { background: #713f12; color: #fde047; }
  .flag-badge.green { background: #14532d; color: #86efac; }
  .flag-badge.chequered { background: #1e293b; color: #e2e8f0; }

  .no-map {
    text-align: center; padding: 2rem; color: #64748b; font-size: .85rem;
    background: #0f172a; border: 1px solid #334155; border-radius: 12px;
  }

  .class-table-wrap { overflow-x: auto; }
  .class-table { width: 100%; border-collapse: collapse; font-size: .82rem; }
  .class-table th {
    text-align: left; color: #475569; font-weight: 600; font-size: .7rem;
    letter-spacing: .05em; text-transform: uppercase;
    padding: .35rem .5rem; border-bottom: 1px solid #1e293b;
  }
  .class-table td { padding: .3rem .5rem; color: #cbd5e1; }
  .class-table tr:hover td { background: #1e293b; }
  .col-pos { width: 2.5rem; font-weight: 700; color: #94a3b8; }
  .driver-code {
    font-size: .72rem; font-weight: 700; color: #f1f5f9;
    padding: .1rem .4rem .1rem .5rem; background: #0f172a; border-radius: 3px;
  }
  .col-team { white-space: nowrap; font-size: .8rem; }
  .col-gap, .col-itv, .col-lap {
    font-variant-numeric: tabular-nums; white-space: nowrap; color: #94a3b8;
  }
  .col-gap { color: #cbd5e1; }
</style>
