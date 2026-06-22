<script>
  import { onMount } from 'svelte';
  import Live from './f1/Live.svelte';

  let races = [];
  let drivers = [];
  let constructors = [];
  let season = new Date().getFullYear();
  let lastRefresh = null;
  let loading = true;
  let refreshing = false;
  let error = '';

  // Detail panel
  let selectedRace = null; // Race object
  let raceResults = [];
  let resultsLoading = false;
  let resultsError = '';
  let qualifyingResults = [];
  let qualifyingLoading = false;
  let qualifyingError = '';
  let detailTab = 'race'; // 'race' | 'qualifying'

  // Sub-tab: 'calendar' | 'standings'
  let subTab = 'calendar';

  // Constructor colors for visual identification
  const TEAM_COLORS = {
    red_bull:    '#3671c6',
    ferrari:     '#e8002d',
    mercedes:    '#27f4d2',
    mclaren:     '#ff8000',
    aston_martin:'#229971',
    alpine:      '#ff87bc',
    williams:    '#64c4ff',
    rb:          '#6692ff',
    kick_sauber: '#52e252',
    haas:        '#b6babd',
  };

  function teamColor(constructorId) {
    return TEAM_COLORS[constructorId] || '#94a3b8';
  }

  // Country to flag emoji
  const COUNTRY_FLAGS = {
    'Australia':    '🇦🇺', 'China':        '🇨🇳', 'Japan':        '🇯🇵',
    'Bahrain':      '🇧🇭', 'Saudi Arabia':  '🇸🇦', 'USA':          '🇺🇸',
    'United States':'🇺🇸', 'Italy':        '🇮🇹', 'Monaco':       '🇲🇨',
    'Canada':       '🇨🇦', 'Spain':        '🇪🇸', 'Austria':      '🇦🇹',
    'UK':           '🇬🇧', 'Hungary':      '🇭🇺', 'Belgium':      '🇧🇪',
    'Netherlands':  '🇳🇱', 'Azerbaijan':   '🇦🇿', 'Singapore':    '🇸🇬',
    'Mexico':       '🇲🇽', 'Brazil':       '🇧🇷', 'UAE':          '🇦🇪',
    'Qatar':        '🇶🇦', 'Las Vegas':    '🇺🇸', 'Miami':        '🇺🇸',
  };

  function flag(country) {
    return COUNTRY_FLAGS[country] || '🏁';
  }

  function formatDate(dateStr) {
    if (!dateStr) return '';
    // Accept both "YYYY-MM-DD" and "YYYY-MM-DDT..." formats
    const plain = dateStr.length === 10 ? dateStr : dateStr.slice(0, 10);
    const d = new Date(plain + 'T00:00:00Z');
    return d.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', timeZone: 'UTC' });
  }

  function formatStatus(status) {
    if (!status) return '';
    const s = status.toLowerCase();
    if (s === 'finished') return 'Terminé';
    if (s.includes('accident') || s.includes('collision')) return 'Accident';
    if (s.includes('disqualified')) return 'Disqualifié';
    if (s.includes('retired') || s.includes('mechanical') || s.includes('engine')
        || s.includes('gearbox') || s.includes('hydraulics') || s.includes('power unit')
        || s.includes('brakes') || s.includes('electrical') || s.includes('overheating')) return 'Abandon';
    if (s.includes('lapped') || s.includes('+ lap')) return 'Doublé';
    return status;
  }

  onMount(async () => {
    await Promise.all([loadCalendar(), loadStandings()]);
  });

  async function loadCalendar() {
    loading = true;
    error = '';
    try {
      const res = await fetch('/api/f1/races');
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      races = data.races ?? [];
      season = data.season ?? season;
      lastRefresh = data.lastRefresh ?? null;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function loadStandings() {
    try {
      const res = await fetch('/api/f1/standings');
      if (!res.ok) return;
      const data = await res.json();
      drivers = data.drivers ?? [];
      constructors = data.constructors ?? [];
    } catch {}
  }

  async function forceRefresh() {
    refreshing = true;
    try {
      await fetch('/api/f1/refresh', { method: 'POST' });
      selectedRace = null;
      await Promise.all([loadCalendar(), loadStandings()]);
    } finally {
      refreshing = false;
    }
  }

  async function openRace(race) {
    if (!race.isPast) return;
    if (selectedRace?.round === race.round) {
      selectedRace = null;
      return;
    }
    selectedRace = race;
    detailTab = 'race';
    loadRaceResults(race);
    loadQualifying(race);
  }

  async function loadRaceResults(race) {
    raceResults = [];
    resultsLoading = true;
    resultsError = '';
    try {
      const res = await fetch(`/api/f1/races/${race.season}/${race.round}/results`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      raceResults = await res.json();
    } catch (e) {
      resultsError = e.message;
    } finally {
      resultsLoading = false;
    }
  }

  async function loadQualifying(race) {
    qualifyingResults = [];
    qualifyingLoading = true;
    qualifyingError = '';
    try {
      const res = await fetch(`/api/f1/races/${race.season}/${race.round}/qualifying`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      qualifyingResults = await res.json();
    } catch (e) {
      qualifyingError = e.message;
    } finally {
      qualifyingLoading = false;
    }
  }

  $: pastRaces = races.filter(r => r.isPast).reverse();
  $: upcomingRaces = races.filter(r => !r.isPast);
  $: nextRace = upcomingRaces[0] ?? null;

  // A race is "live" if now is within the race window. raceTime is UTC ("HH:MM:SS"
  // or ""); we open a generous window [start-1h, start+4h] to cover the whole event.
  function isRaceLive(race) {
    if (!race) return false;
    const datePart = (race.raceDate ?? '').slice(0, 10);
    if (!datePart) return false;
    const timePart = race.raceTime && race.raceTime.length >= 5 ? race.raceTime.slice(0, 8) : '13:00:00';
    const start = new Date(`${datePart}T${timePart}Z`).getTime();
    if (Number.isNaN(start)) return false;
    const now = Date.now();
    return now >= start - 3600_000 && now <= start + 4 * 3600_000;
  }

  // The live race is the next upcoming race if it's currently within its window.
  $: liveRace = isRaceLive(nextRace) ? nextRace : null;

  // Auto-switch to the Live tab once, on first load, if a race is in progress —
  // so opening the page during a GP lands straight on the live view.
  let autoSwitched = false;
  $: if (liveRace && !autoSwitched) {
    autoSwitched = true;
    subTab = 'live';
  }

  // Demo mode flag: when no race is live, the user can still preview the live UI
  // with the last finished session's static data.
  let showDemo = false;

  function positionLabel(pos) {
    if (pos === 1) return '🥇';
    if (pos === 2) return '🥈';
    if (pos === 3) return '🥉';
    return pos + '.';
  }
</script>

<div class="f1-page">
  <!-- Header -->
  <div class="header">
    <div class="header-left">
      <h2>F1 {season}</h2>
      {#if lastRefresh}
        <span class="last-refresh">
          Sync {new Date(lastRefresh).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })}
          {new Date(lastRefresh).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}
        </span>
      {/if}
    </div>
    <button class="btn-refresh" on:click={forceRefresh} disabled={refreshing}>
      {refreshing ? 'Actualisation…' : 'Actualiser'}
    </button>
  </div>

  <!-- Sub-tabs -->
  <div class="subtabbar">
    {#if liveRace}
      <button class="subtab live-tab {subTab === 'live' ? 'active' : ''}" on:click={() => subTab = 'live'}>
        <span class="live-dot"></span> Live
      </button>
    {/if}
    <button class="subtab {subTab === 'calendar' ? 'active' : ''}" on:click={() => subTab = 'calendar'}>
      Calendrier
    </button>
    <button class="subtab {subTab === 'standings' ? 'active' : ''}" on:click={() => subTab = 'standings'}>
      Classements
    </button>
  </div>

  {#if loading}
    <div class="state-msg">Chargement…</div>
  {:else if error}
    <div class="state-msg error">{error}</div>
  {:else if subTab === 'live' && liveRace}

    <!-- ── LIVE TAB ── -->
    <Live demo={false} />

  {:else if subTab === 'calendar'}

    <!-- ── CALENDAR TAB ── -->
    <div class="calendar-layout">
      <!-- Race list -->
      <div class="race-list">

        <!-- Next race highlight -->
        {#if nextRace}
          <div class="section-label">Prochaine course</div>
          <!-- svelte-ignore a11y-no-static-element-interactions -->
          <div class="race-card next-race">
            <div class="race-flag">{flag(nextRace.country)}</div>
            <div class="race-info">
              <div class="race-name">{nextRace.raceName}</div>
              <div class="race-meta">{nextRace.circuitName} · {nextRace.locality}</div>
            </div>
            <div class="race-date next-date">{formatDate(nextRace.raceDate)}</div>
          </div>
        {/if}

        <!-- Past races -->
        {#if pastRaces.length > 0}
          <div class="section-label">Courses passées</div>
          {#each pastRaces as race}
            <!-- svelte-ignore a11y-no-static-element-interactions -->
            <div
              class="race-card past {selectedRace?.round === race.round ? 'selected' : ''}"
              on:click={() => openRace(race)}
              on:keydown={e => e.key === 'Enter' && openRace(race)}
              role="button"
              tabindex="0"
            >
              <div class="race-flag">{flag(race.country)}</div>
              <div class="race-info">
                <div class="race-name">{race.raceName}</div>
                <div class="race-meta">{race.circuitName}</div>
              </div>
              <div class="race-date">{formatDate(race.raceDate)}</div>
              <div class="race-chevron">{selectedRace?.round === race.round ? '▲' : '▼'}</div>
            </div>

            <!-- Inline results panel -->
            {#if selectedRace?.round === race.round}
              <div class="results-panel">
                <!-- Detail sub-tabs -->
                <div class="detail-tabs">
                  <button class="detail-tab {detailTab === 'race' ? 'active' : ''}" on:click={() => detailTab = 'race'}>Course</button>
                  <button class="detail-tab {detailTab === 'qualifying' ? 'active' : ''}" on:click={() => detailTab = 'qualifying'}>Qualifications</button>
                </div>

                {#if detailTab === 'race'}
                  {#if resultsLoading}
                    <div class="results-state">Chargement…</div>
                  {:else if resultsError}
                    <div class="results-state error">{resultsError}</div>
                  {:else if raceResults.length === 0}
                    <div class="results-state">Résultats non disponibles.</div>
                  {:else}
                    <!-- Podium -->
                    {#if raceResults.length >= 3}
                      {@const p1 = raceResults[0]}
                      {@const p2 = raceResults[1]}
                      {@const p3 = raceResults[2]}
                      <div class="podium">
                        <div class="podium-col">
                          <div class="podium-info">
                            <div class="podium-driver">{p2.driverGivenName} <strong>{p2.driverFamilyName}</strong></div>
                            <div class="podium-team" style="color:{teamColor(p2.constructorId)}">{p2.constructorName}</div>
                            <div class="podium-pts">{p2.points} pts</div>
                          </div>
                          <div class="podium-block p2"><span class="podium-medal">🥈</span><span class="podium-rank">2</span></div>
                        </div>
                        <div class="podium-col">
                          <div class="podium-info">
                            <div class="podium-driver">{p1.driverGivenName} <strong>{p1.driverFamilyName}</strong></div>
                            <div class="podium-team" style="color:{teamColor(p1.constructorId)}">{p1.constructorName}</div>
                            <div class="podium-pts">{p1.points} pts</div>
                          </div>
                          <div class="podium-block p1"><span class="podium-medal">🥇</span><span class="podium-rank">1</span></div>
                        </div>
                        <div class="podium-col">
                          <div class="podium-info">
                            <div class="podium-driver">{p3.driverGivenName} <strong>{p3.driverFamilyName}</strong></div>
                            <div class="podium-team" style="color:{teamColor(p3.constructorId)}">{p3.constructorName}</div>
                            <div class="podium-pts">{p3.points} pts</div>
                          </div>
                          <div class="podium-block p3"><span class="podium-medal">🥉</span><span class="podium-rank">3</span></div>
                        </div>
                      </div>
                    {/if}

                    <!-- Race results table -->
                    <div class="results-table-wrap">
                      <table class="results-table">
                        <thead>
                          <tr><th>Pos</th><th>Pilote</th><th>Écurie</th><th>Pts</th><th>Statut</th></tr>
                        </thead>
                        <tbody>
                          {#each raceResults as r}
                            <tr class="pos-row {r.position <= 3 ? 'top3' : ''} {r.fastestLapRank === 1 ? 'fastest' : ''}">
                              <td class="col-pos">{r.position}</td>
                              <td class="col-driver">
                                <span class="driver-code" style="border-left:3px solid {teamColor(r.constructorId)}">{r.driverCode}</span>
                                {r.driverGivenName} {r.driverFamilyName}
                                {#if r.fastestLapRank === 1}<span class="fastest-badge" title="Meilleur tour">⚡</span>{/if}
                              </td>
                              <td class="col-team" style="color:{teamColor(r.constructorId)}">{r.constructorName}</td>
                              <td class="col-pts">{r.points}</td>
                              <td class="col-status {r.status?.toLowerCase() === 'finished' ? 'ok' : 'dnf'}">{formatStatus(r.status)}</td>
                            </tr>
                          {/each}
                        </tbody>
                      </table>
                    </div>
                  {/if}

                {:else}
                  {#if qualifyingLoading}
                    <div class="results-state">Chargement…</div>
                  {:else if qualifyingError}
                    <div class="results-state error">{qualifyingError}</div>
                  {:else if qualifyingResults.length === 0}
                    <div class="results-state">Qualifications non disponibles.</div>
                  {:else}
                    <div class="results-table-wrap">
                      <table class="results-table">
                        <thead>
                          <tr><th>Pos</th><th>Pilote</th><th>Écurie</th><th>Q1</th><th>Q2</th><th>Q3</th></tr>
                        </thead>
                        <tbody>
                          {#each qualifyingResults as r}
                            <tr class="pos-row {r.position <= 3 ? 'top3' : ''}">
                              <td class="col-pos">{r.position}</td>
                              <td class="col-driver">
                                <span class="driver-code" style="border-left:3px solid {teamColor(r.constructorId)}">{r.driverCode}</span>
                                {r.driverGivenName} {r.driverFamilyName}
                              </td>
                              <td class="col-team" style="color:{teamColor(r.constructorId)}">{r.constructorName}</td>
                              <td class="col-qtime">{r.q1 || '—'}</td>
                              <td class="col-qtime">{r.q2 || '—'}</td>
                              <td class="col-qtime {r.position === 1 ? 'pole' : ''}">{r.q3 || '—'}</td>
                            </tr>
                          {/each}
                        </tbody>
                      </table>
                    </div>
                  {/if}
                {/if}
              </div>
            {/if}
          {/each}
        {/if}

        <!-- Future races (excluding next) -->
        {#if upcomingRaces.length > 1}
          <div class="section-label">À venir</div>
          {#each upcomingRaces.slice(1) as race}
            <div class="race-card future">
              <div class="race-flag">{flag(race.country)}</div>
              <div class="race-info">
                <div class="race-name">{race.raceName}</div>
                <div class="race-meta">{race.circuitName}</div>
              </div>
              <div class="race-date">{formatDate(race.raceDate)}</div>
            </div>
          {/each}
        {/if}

        <!-- Live UI preview (static): demo with the last finished session's data -->
        <div class="demo-section">
          <div class="section-label">Aperçu live (démo)</div>
          {#if showDemo}
            <Live demo={true} />
            <button class="btn-demo" on:click={() => showDemo = false}>Masquer l'aperçu</button>
          {:else}
            <button class="btn-demo" on:click={() => showDemo = true}>
              Afficher un aperçu de l'interface live (dernière course)
            </button>
          {/if}
        </div>

      </div>
    </div>

  {:else}

    <!-- ── STANDINGS TAB ── -->
    <div class="standings-layout">

      <!-- Driver standings -->
      <div class="standings-card">
        <h3 class="standings-title">Pilotes</h3>
        {#if drivers.length === 0}
          <div class="results-state">Aucun classement disponible.</div>
        {:else}
          <table class="standings-table">
            <thead>
              <tr><th>Pos</th><th>Pilote</th><th>Écurie</th><th>Pts</th><th>Victoires</th></tr>
            </thead>
            <tbody>
              {#each drivers as d}
                <tr class="{d.position === 1 ? 'leader' : ''}">
                  <td class="col-pos">{d.position}</td>
                  <td class="col-driver">
                    <span class="driver-code" style="border-left:3px solid {teamColor(
                      constructors.find(c => c.constructorName === d.constructorName)?.constructorId ?? ''
                    )}">{d.driverCode}</span>
                    {d.driverGivenName} {d.driverFamilyName}
                  </td>
                  <td class="col-team" style="color:{teamColor(
                    constructors.find(c => c.constructorName === d.constructorName)?.constructorId ?? ''
                  )}">{d.constructorName}</td>
                  <td class="col-pts">{d.points}</td>
                  <td class="col-wins">{d.wins}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>

      <!-- Constructor standings -->
      <div class="standings-card">
        <h3 class="standings-title">Constructeurs</h3>
        {#if constructors.length === 0}
          <div class="results-state">Aucun classement disponible.</div>
        {:else}
          <table class="standings-table">
            <thead>
              <tr><th>Pos</th><th>Écurie</th><th>Pts</th><th>Victoires</th></tr>
            </thead>
            <tbody>
              {#each constructors as c}
                <tr class="{c.position === 1 ? 'leader' : ''}">
                  <td class="col-pos">{c.position}</td>
                  <td class="col-team">
                    <span class="team-dot" style="background:{teamColor(c.constructorId)}"></span>
                    {c.constructorName}
                  </td>
                  <td class="col-pts">{c.points}</td>
                  <td class="col-wins">{c.wins}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>

    </div>
  {/if}
</div>

<style>
  .f1-page { max-width: 860px; margin: 0 auto; }

  /* Header */
  .header {
    display: flex; align-items: flex-start; justify-content: space-between;
    gap: 1rem; margin-bottom: 1rem; flex-wrap: wrap;
  }
  .header-left { display: flex; flex-direction: column; gap: .25rem; }
  h2 { margin: 0; font-size: 1.4rem; color: #f1f5f9; }
  .last-refresh { font-size: .75rem; color: #475569; }
  .btn-refresh {
    background: #3f0909; border: 1px solid #e10600; color: #fca5a5;
    padding: .45rem 1rem; border-radius: 8px; font-size: .88rem; cursor: pointer;
    white-space: nowrap;
  }
  .btn-refresh:hover:not(:disabled) { background: #7f1d1d; }
  .btn-refresh:disabled { opacity: .5; cursor: default; }

  /* Sub-tabs */
  .subtabbar {
    display: flex; border-bottom: 1px solid #334155; margin-bottom: 1.25rem; gap: .1rem;
  }
  .subtab {
    background: none; border: none; color: #64748b;
    padding: .55rem 1.1rem; font-size: .9rem; cursor: pointer;
    border-bottom: 2px solid transparent; transition: color .15s, border-color .15s;
  }
  .subtab:hover { color: #f1f5f9; }
  .subtab.active { color: #e10600; border-bottom-color: #e10600; }
  .live-tab { display: flex; align-items: center; gap: .4rem; color: #fca5a5; }
  .live-dot {
    width: 8px; height: 8px; border-radius: 50%; background: #ef4444;
    animation: live-pulse 1.4s infinite;
  }
  @keyframes live-pulse { 0%,100% { opacity: 1; } 50% { opacity: .3; } }

  /* Demo preview */
  .demo-section { margin-top: 1.5rem; padding-top: 1rem; border-top: 1px dashed #334155; }
  .btn-demo {
    width: 100%; margin-top: .5rem;
    background: #1e293b; border: 1px solid #334155; color: #94a3b8;
    padding: .6rem 1rem; border-radius: 8px; font-size: .85rem; cursor: pointer;
    transition: background .15s, color .15s;
  }
  .btn-demo:hover { background: #263247; color: #f1f5f9; }

  /* Section labels */
  .section-label {
    font-size: .7rem; font-weight: 700; letter-spacing: .08em; text-transform: uppercase;
    color: #475569; margin: 1rem 0 .4rem;
  }

  /* Race list */
  .race-list { display: flex; flex-direction: column; gap: .4rem; }

  .race-card {
    display: flex; align-items: center; gap: .75rem;
    background: #1e293b; border: 1px solid #334155;
    border-radius: 10px; padding: .65rem 1rem;
  }
  .race-card.past {
    cursor: pointer; border-left: 3px solid #e10600;
    transition: border-color .15s, background .15s;
  }
  .race-card.past:hover { background: #263247; }
  .race-card.past.selected { background: #263247; border-color: #e10600; }
  .race-card.next-race { border-left: 3px solid #e10600; background: #1a1010; }
  .race-card.future { opacity: .55; }

  .race-flag { font-size: 1.4rem; flex-shrink: 0; width: 2rem; text-align: center; }
  .race-info { flex: 1; min-width: 0; }
  .race-name { font-size: .92rem; font-weight: 600; color: #f1f5f9;
               white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .race-meta { font-size: .75rem; color: #64748b; margin-top: .15rem; }
  .race-date { font-size: .82rem; color: #94a3b8; white-space: nowrap; }
  .next-date { color: #fca5a5; font-weight: 600; }
  .race-chevron { font-size: .7rem; color: #64748b; margin-left: .25rem; }

  /* Results panel */
  .results-panel {
    background: #0f172a; border: 1px solid #334155; border-top: none;
    border-radius: 0 0 10px 10px; padding: 1rem;
    margin-top: -4px;
  }

  /* Detail sub-tabs (race / qualifying) */
  .detail-tabs {
    display: flex; gap: .1rem; margin-bottom: .75rem;
    border-bottom: 1px solid #1e293b; padding-bottom: 0;
  }
  .detail-tab {
    background: none; border: none; color: #475569;
    padding: .35rem .75rem; font-size: .8rem; cursor: pointer;
    border-bottom: 2px solid transparent; transition: color .15s, border-color .15s;
  }
  .detail-tab:hover { color: #94a3b8; }
  .detail-tab.active { color: #e10600; border-bottom-color: #e10600; }
  .results-state { text-align: center; padding: 1.5rem; color: #64748b; }
  .results-state.error { color: #f87171; }

  /* Podium */
  .podium {
    display: flex; align-items: flex-end; gap: 0;
    margin-bottom: 1.25rem; height: 130px;
  }
  .podium-col {
    flex: 1; display: flex; flex-direction: column; align-items: center;
    justify-content: flex-end;
  }
  .podium-info {
    text-align: center; margin-bottom: .4rem; padding: 0 .25rem;
  }
  .podium-driver { font-size: .75rem; color: #cbd5e1; line-height: 1.3; }
  .podium-driver strong { color: #f1f5f9; }
  .podium-team { font-size: .68rem; margin-top: .1rem; }
  .podium-pts { font-size: .68rem; color: #64748b; margin-top: .1rem; }
  .podium-block {
    width: 100%; display: flex; flex-direction: column;
    align-items: center; justify-content: center;
    border-radius: 6px 6px 0 0; gap: .2rem;
  }
  .podium-medal { font-size: 1.1rem; }
  .podium-rank { font-size: 1rem; font-weight: 800; }
  .podium-block.p1 { background: #78500a; border: 1px solid #f59e0b; border-bottom: none; height: 72px; }
  .podium-block.p1 .podium-rank { color: #fcd34d; }
  .podium-block.p2 { background: #334155; border: 1px solid #94a3b8; border-bottom: none; height: 52px; }
  .podium-block.p2 .podium-rank { color: #cbd5e1; }
  .podium-block.p3 { background: #4a2e10; border: 1px solid #c2793d; border-bottom: none; height: 38px; }
  .podium-block.p3 .podium-rank { color: #d97706; }

  /* Results table */
  .results-table-wrap { overflow-x: auto; }
  .results-table {
    width: 100%; border-collapse: collapse; font-size: .82rem;
  }
  .results-table th {
    text-align: left; color: #475569; font-weight: 600; font-size: .7rem;
    letter-spacing: .05em; text-transform: uppercase;
    padding: .35rem .5rem; border-bottom: 1px solid #1e293b;
  }
  .results-table td { padding: .3rem .5rem; color: #cbd5e1; }
  .results-table tr:hover td { background: #1e293b; }
  .pos-row.top3 td { color: #f1f5f9; }

  .col-pos { width: 2.5rem; font-weight: 700; color: #94a3b8; }
  .col-driver { display: flex; align-items: center; gap: .5rem; white-space: nowrap; }
  .driver-code {
    font-size: .72rem; font-weight: 700; color: #94a3b8;
    padding: .1rem .4rem; background: #0f172a; border-radius: 3px;
    padding-left: .5rem; flex-shrink: 0;
  }
  .fastest-badge { font-size: .75rem; margin-left: .2rem; }
  .col-team { white-space: nowrap; font-size: .8rem; }
  .col-pts { font-weight: 600; white-space: nowrap; }
  .col-status.ok { color: #4ade80; }
  .col-status.dnf { color: #f87171; }
  .col-wins { color: #fbbf24; }
  .col-qtime { font-size: .78rem; color: #94a3b8; font-variant-numeric: tabular-nums; white-space: nowrap; }
  .col-qtime.pole { color: #c084fc; font-weight: 700; }

  /* Standings */
  .standings-layout { display: flex; flex-direction: column; gap: 1.5rem; }
  .standings-card {
    background: #1e293b; border: 1px solid #334155; border-radius: 12px;
    padding: 1rem 1.25rem;
  }
  h3.standings-title { margin: 0 0 .75rem; font-size: 1rem; color: #f1f5f9; }
  .standings-table {
    width: 100%; border-collapse: collapse; font-size: .84rem;
  }
  .standings-table th {
    text-align: left; color: #475569; font-weight: 600; font-size: .7rem;
    letter-spacing: .05em; text-transform: uppercase;
    padding: .35rem .5rem; border-bottom: 1px solid #334155;
  }
  .standings-table td { padding: .4rem .5rem; color: #cbd5e1; }
  .standings-table tr.leader td { color: #f1f5f9; font-weight: 600; }
  .standings-table tr:hover td { background: #0f172a; }

  .team-dot {
    display: inline-block; width: 10px; height: 10px;
    border-radius: 50%; margin-right: .45rem; vertical-align: middle; flex-shrink: 0;
  }

  /* States */
  .state-msg { text-align: center; padding: 3rem; color: #64748b; font-size: .95rem; }
  .state-msg.error { color: #f87171; }

  @media (min-width: 640px) {
    .standings-layout { flex-direction: row; align-items: flex-start; }
    .standings-card { flex: 1; }
  }
</style>
