<script>
  import { onMount, onDestroy } from 'svelte';

  let matches = [];
  let lastRefresh = null;
  let enabledLeagues = []; // leagues saved in DB (for header pills)
  let loading = true;
  let refreshing = false;
  let error = '';
  let subTab = 'upcoming'; // 'upcoming' | 'results'

  // League picker modal
  let showPicker = false;
  let pickerLeagues = []; // all 41 from API, with enabled state merged
  let pickerLoading = false;

  const LEAGUE_COLORS = {
    lec:         { bg: '#1a3a6b', accent: '#4a9eff', label: '#7ec8ff' },
    lck:         { bg: '#6b1a1a', accent: '#ff4a4a', label: '#ff9090' },
    lpl:         { bg: '#6b5a1a', accent: '#ffc84a', label: '#ffe090' },
    msi:         { bg: '#1a6b3a', accent: '#4affa0', label: '#90ffc8' },
    worlds:      { bg: '#4a1a6b', accent: '#c84aff', label: '#e090ff' },
    first_stand: { bg: '#6b3a1a', accent: '#ff8c4a', label: '#ffb890' },
  };

  function leagueColor(slug) {
    return LEAGUE_COLORS[slug] || { bg: '#1e293b', accent: '#94a3b8', label: '#cbd5e1' };
  }

  onMount(async () => {
    await Promise.all([loadSchedule(), loadEnabledLeagues()]);
  });

  async function loadSchedule() {
    loading = true;
    error = '';
    try {
      const res = await fetch('/api/lol-calendar/schedule');
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      matches = data.matches ?? [];
      lastRefresh = data.lastRefresh ?? null;
      schedulePoll();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function loadEnabledLeagues() {
    try {
      const res = await fetch('/api/lol-calendar/leagues');
      if (!res.ok) return;
      enabledLeagues = (await res.json()) ?? [];
    } catch {}
  }

  async function openPicker() {
    showPicker = true;
    if (pickerLeagues.length > 0) return; // already loaded
    pickerLoading = true;
    try {
      const res = await fetch('/api/lol-calendar/leagues/available');
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      pickerLeagues = await res.json();
    } catch (e) {
      error = e.message;
    } finally {
      pickerLoading = false;
    }
  }

  async function togglePickerLeague(league) {
    league.enabled = !league.enabled;
    pickerLeagues = [...pickerLeagues];
    await fetch(`/api/lol-calendar/leagues/${league.slug}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(league),
    });
    await loadEnabledLeagues();
  }

  async function forceRefresh() {
    refreshing = true;
    try {
      await fetch('/api/lol-calendar/refresh', { method: 'POST' });
      revealSteps = new Map();
      await loadSchedule();
    } finally {
      refreshing = false;
    }
  }

  // Reveal steps for completed matches:
  //   isSpoiler : 0 = teams+score hidden → 1 = teams visible, score hidden → 2 = fully revealed
  //   !isSpoiler: 0 = score hidden → 2 = score visible (teams always shown)
  // Using a Map triggers Svelte reactivity reliably via reassignment.
  let revealSteps = new Map(); // matchId -> 0|1|2

  function revealStep(m) {
    return revealSteps.get(m.matchId) ?? 0;
  }

  function advanceReveal(matchId, isSpoiler) {
    const current = revealSteps.get(matchId) ?? 0;
    const next = isSpoiler ? Math.min(current + 1, 2) : 2;
    revealSteps.set(matchId, next);
    revealSteps = new Map(revealSteps);
    if (next === 2) {
      const m = matches.find(x => x.matchId === matchId);
      if (m?.state === 'completed') {
        fetch(`/api/lol-calendar/matches/${matchId}/dismiss`, { method: 'POST' });
      }
    }
  }

  function groupByDay(list) {
    const groups = [];
    let currentDay = null;
    for (const m of list) {
      const day = new Date(m.scheduledAt).toLocaleDateString('fr-FR', {
        weekday: 'long', day: 'numeric', month: 'long', year: 'numeric'
      });
      if (day !== currentDay) {
        currentDay = day;
        groups.push({ day, matches: [] });
      }
      groups[groups.length - 1].matches.push(m);
    }
    return groups;
  }

  function formatTime(iso) {
    return new Date(iso).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' });
  }

  function isToday(iso) {
    const d = new Date(iso);
    const now = new Date();
    return d.getFullYear() === now.getFullYear() &&
           d.getMonth() === now.getMonth() &&
           d.getDate() === now.getDate();
  }

  // Live polling
  const LIVE_WINDOW_MS = 30 * 60 * 1000;
  const POLL_INTERVAL_MS = 30 * 1000;
  let pollTimer = null;

  function hasLiveOrNearMatch(list) {
    const now = Date.now();
    return list.some(m => {
      if (m.state === 'inProgress') return true;
      const t = new Date(m.scheduledAt).getTime();
      // Poll 30min before scheduled start, or up to 3h after (covers long BO5s still in progress)
      return t - now <= LIVE_WINDOW_MS && now - t <= 3 * 60 * 60 * 1000;
    });
  }

  function mergeLiveMatches(live) {
    const updated = new Map(matches.map(m => [m.matchId, m]));
    for (const m of live) updated.set(m.matchId, m);
    matches = [...updated.values()].sort((a, b) =>
      new Date(a.scheduledAt) - new Date(b.scheduledAt)
    );
  }

  async function pollLive() {
    try {
      await fetch('/api/lol-calendar/refresh-live', { method: 'POST' });
      const res = await fetch('/api/lol-calendar/live');
      if (!res.ok) return;
      const live = await res.json();
      mergeLiveMatches(live);
    } catch {}
    schedulePoll();
  }

  function schedulePoll() {
    clearTimeout(pollTimer);
    if (hasLiveOrNearMatch(matches)) {
      pollTimer = setTimeout(pollLive, POLL_INTERVAL_MS);
    }
  }

  onDestroy(() => clearTimeout(pollTimer));

  // VOD modal
  let vodModal = null; // { match, vods, loading, error }

  async function openVODs(m) {
    vodModal = { match: m, vods: [], loading: true, error: '' };
    try {
      const res = await fetch(`/api/lol-calendar/matches/${m.matchId}/vods`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const vods = await res.json();
      vodModal = { ...vodModal, vods, loading: false };
    } catch (e) {
      vodModal = { ...vodModal, loading: false, error: e.message };
    }
  }

  function vodEmbedURL(vod) {
    if (vod.provider === 'youtube') {
      const t = vod.startSecs > 0 ? `&start=${vod.startSecs}` : '';
      return `https://www.youtube.com/embed/${vod.parameter}?autoplay=0${t}`;
    }
    return null; // twitch handled as link
  }

  function vodExternalURL(vod) {
    if (vod.provider === 'youtube') {
      const t = vod.startSecs > 0 ? `?t=${vod.startSecs}` : '';
      return `https://youtu.be/${vod.parameter}${t}`;
    }
    if (vod.provider === 'twitch') {
      return `https://www.twitch.tv/videos/${vod.parameter}`;
    }
    return '#';
  }

  // Group leagues by region for the picker
  $: leaguesByRegion = pickerLeagues.reduce((acc, l) => {
    const r = l.region || 'OTHER';
    if (!acc[r]) acc[r] = [];
    acc[r].push(l);
    return acc;
  }, {});
  $: regionOrder = ['INTERNATIONAL', 'KOREA', 'CHINA', 'EUROPE', 'NORTH AMERICA',
                    'LATIN AMERICA', 'BRAZIL', 'OCEANIA', 'JAPAN', 'SOUTHEAST ASIA', 'OTHER'];
  $: sortedRegions = Object.keys(leaguesByRegion).sort(
    (a, b) => (regionOrder.indexOf(a) + 1 || 99) - (regionOrder.indexOf(b) + 1 || 99)
  );

  $: upcomingMatches = matches.filter(m => m.state !== 'completed' || isToday(m.scheduledAt));
  $: resultMatches   = [...matches.filter(m => m.state === 'completed' && !isToday(m.scheduledAt))].reverse();
  $: upcomingGroups  = groupByDay(upcomingMatches);
  $: resultGroups    = groupByDay(resultMatches);
  $: activeGroups    = subTab === 'upcoming' ? upcomingGroups : resultGroups;
  $: todayIndex      = upcomingGroups.findIndex(g => g.matches.some(m => isToday(m.scheduledAt)));
</script>

<!-- League picker modal -->
{#if showPicker}
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="modal-backdrop" on:click|self={() => showPicker = false} on:keydown={() => {}}>
    <div class="modal">
      <div class="modal-header">
        <h3>Ligues suivies</h3>
        <button class="btn-close" on:click={() => showPicker = false}>✕</button>
      </div>
      <p class="modal-hint">Activez les ligues à inclure dans le calendrier. Les changements s'appliquent au prochain refresh.</p>

      {#if pickerLoading}
        <div class="picker-loading">Chargement…</div>
      {:else}
        <div class="modal-body">
          {#each sortedRegions as region}
            <div class="region-section">
              <div class="region-label">{region}</div>
              <div class="region-leagues">
                {#each leaguesByRegion[region] as league}
                  {@const c = leagueColor(league.slug)}
                  <label class="league-row" class:enabled={league.enabled}>
                    <input type="checkbox" checked={league.enabled} on:change={() => togglePickerLeague(league)} />
                    <img src={league.imageUrl} alt={league.name} class="league-logo"
                         on:error={e => e.target.style.display='none'} />
                    <span class="league-name">{league.name}</span>
                    <span class="league-check" style="background:{league.enabled ? c.accent : 'transparent'}; border-color:{c.accent}">
                      {#if league.enabled}✓{/if}
                    </span>
                  </label>
                {/each}
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
{/if}

<div class="lol-calendar">
  <!-- Header -->
  <div class="header">
    <div class="header-left">
      <h2>Calendrier LoL</h2>
      <div class="league-pills">
        {#each enabledLeagues as l}
          {@const c = leagueColor(l.slug)}
          <span class="league-pill" style="background:{c.bg}; color:{c.label}">
            {l.name}
          </span>
        {/each}
      </div>
    </div>
    <div class="header-actions">
      {#if lastRefresh}
        <span class="last-refresh">
          Sync {new Date(lastRefresh).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })}
          {new Date(lastRefresh).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}
        </span>
      {/if}
      <button class="btn-icon" title="Choisir les ligues" on:click={openPicker}>⚙</button>
      <button class="btn-refresh" on:click={forceRefresh} disabled={refreshing}>
        {refreshing ? 'Actualisation…' : 'Actualiser'}
      </button>
    </div>
  </div>

  <!-- Sub-tabs -->
  <div class="subtabbar">
    <button class="subtab {subTab === 'upcoming' ? 'active' : ''}" on:click={() => subTab = 'upcoming'}>
      À venir
    </button>
    <button class="subtab {subTab === 'results' ? 'active' : ''}" on:click={() => subTab = 'results'}>
      Résultats
    </button>
  </div>

  <!-- Content -->
  {#if loading}
    <div class="state-msg">Chargement…</div>
  {:else if error}
    <div class="state-msg error">{error}</div>
  {:else if activeGroups.length === 0}
    <div class="state-msg">
      {subTab === 'upcoming' ? 'Aucun match à venir.' : 'Aucun résultat disponible. Cliquez sur Actualiser.'}
    </div>
  {:else}
    <div class="schedule">
      {#each activeGroups as group, gi}
        {@const isCurrentDay = subTab === 'upcoming' && gi === todayIndex}
        <div class="day-group" class:today={isCurrentDay}>
          <div class="day-label">
            {#if isCurrentDay}<span class="today-badge">Aujourd'hui</span>{/if}
            {group.day}
          </div>
          <div class="match-list">
            {#each group.matches as m}
              {@const c = leagueColor(m.leagueSlug)}
              {@const completed = m.state === 'completed'}
              {@const live = m.state === 'inProgress'}
              {@const step = revealSteps.get(m.matchId) ?? (m.spoilerDismissed ? 2 : 0)}
              {@const teamsVisible = live || (!m.isSpoiler || step >= 1)}
              {@const scoreVisible = (completed || live) && step >= 2}
              {@const needsScoreReveal = (completed || live) && !scoreVisible}
              <div class="match-card" class:completed class:live style="--accent:{c.accent}">
                <div class="match-league" style="background:{c.bg}; color:{c.label}">{m.leagueName}</div>
                <div class="match-stage">{m.stage}</div>
                <div class="match-time-vod">
                  {#if live}
                    <a class="btn-live" href="https://lolesports.com/live" target="_blank" rel="noopener noreferrer">
                      <span class="live-dot"></span> LIVE
                    </a>
                  {:else}
                    {formatTime(m.scheduledAt)}
                  {/if}
                  {#if completed}
                    <button class="btn-vod" on:click={() => openVODs(m)} title="Voir les VODs">▶ VODs</button>
                  {/if}
                </div>

                <div class="match-teams">
                  <!-- Team 1 -->
                  <div class="team"
                    class:winner={completed && scoreVisible && m.team1.outcome === 'win'}
                    class:loser={completed && scoreVisible && m.team1.outcome === 'loss'}
                    class:hidden-team={!teamsVisible}>
                    {#if teamsVisible}
                      {#if m.team1.imageUrl}<img src={m.team1.imageUrl} alt={m.team1.code} class="team-logo" />{/if}
                      <span class="team-code">{m.team1.code}</span>
                    {:else}
                      <span class="team-placeholder">? ? ?</span>
                    {/if}
                  </div>

                  <!-- Centre -->
                  <div class="match-vs">
                    {#if !teamsVisible}
                      <button class="btn-reveal-spoiler" on:click={() => advanceReveal(m.matchId, m.isSpoiler)}>
                        <span class="eye-icon">👁</span> Voir le match
                      </button>
                    {:else if scoreVisible}
                      <span class="score">{m.team1.gameWins} – {m.team2.gameWins}</span>
                    {:else if needsScoreReveal}
                      <span class="bo">BO{m.bestOf}</span>
                      <button class="btn-reveal-score" on:click={() => advanceReveal(m.matchId, m.isSpoiler)}>
                        <span class="eye-icon">👁</span> Score
                      </button>
                    {:else}
                      <span class="bo">BO{m.bestOf}</span>
                    {/if}
                  </div>

                  <!-- Team 2 -->
                  <div class="team team-right"
                    class:winner={completed && scoreVisible && m.team2.outcome === 'win'}
                    class:loser={completed && scoreVisible && m.team2.outcome === 'loss'}
                    class:hidden-team={!teamsVisible}>
                    {#if teamsVisible}
                      <span class="team-code">{m.team2.code}</span>
                      {#if m.team2.imageUrl}<img src={m.team2.imageUrl} alt={m.team2.code} class="team-logo" />{/if}
                    {:else}
                      <span class="team-placeholder">? ? ?</span>
                    {/if}
                  </div>
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- VOD modal -->
{#if vodModal}
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <div class="modal-backdrop" on:click|self={() => vodModal = null} on:keydown={() => {}}>
    <div class="modal vod-modal">
      <div class="modal-header">
        {#if vodModal.match.state === 'completed'}
          {@const step = revealSteps.get(vodModal.match.matchId) ?? (vodModal.match.spoilerDismissed ? 2 : 0)}
          {@const teamsVisible = !vodModal.match.isSpoiler || step >= 1}
          <h3>
            {#if teamsVisible}
              {vodModal.match.team1.code} vs {vodModal.match.team2.code}
            {:else}
              VODs — {vodModal.match.leagueName}
            {/if}
          </h3>
        {:else}
          <h3>VODs — {vodModal.match.leagueName}</h3>
        {/if}
        <button class="btn-close" on:click={() => vodModal = null}>✕</button>
      </div>

      <div class="vod-body">
        {#if vodModal.loading}
          <div class="vod-state">Chargement…</div>
        {:else if vodModal.error}
          <div class="vod-state error">{vodModal.error}</div>
        {:else if vodModal.vods.length === 0}
          <div class="vod-state">Aucune VOD disponible pour ce match.</div>
        {:else}
          {#each vodModal.vods as vod}
            {@const embedURL = vodEmbedURL(vod)}
            <div class="vod-game">
              <div class="vod-game-header">
                <span class="vod-game-label">Game {vod.gameNumber}</span>
                <a href={vodExternalURL(vod)} target="_blank" rel="noopener noreferrer" class="vod-ext-link">
                  ↗ Ouvrir
                </a>
              </div>
              {#if embedURL}
                <iframe
                  src={embedURL}
                  title="Game {vod.gameNumber}"
                  class="vod-iframe"
                  frameborder="0"
                  allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
                  allowfullscreen
                ></iframe>
              {:else}
                <a href={vodExternalURL(vod)} target="_blank" rel="noopener noreferrer" class="vod-twitch-link">
                  Voir sur Twitch ↗
                </a>
              {/if}
            </div>
          {/each}
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .lol-calendar { max-width: 860px; margin: 0 auto; }

  /* Header */
  .header {
    display: flex; align-items: flex-start; justify-content: space-between;
    gap: 1rem; margin-bottom: 1rem; flex-wrap: wrap;
  }
  .header-left { display: flex; flex-direction: column; gap: .5rem; }
  h2 { margin: 0; font-size: 1.4rem; color: #f1f5f9; }
  .league-pills { display: flex; flex-wrap: wrap; gap: .4rem; }
  .league-pill {
    font-size: .72rem; font-weight: 600; padding: .2rem .55rem;
    border-radius: 999px; letter-spacing: .04em;
  }
  .header-actions { display: flex; gap: .5rem; align-items: center; }
  .last-refresh { font-size: .75rem; color: #475569; white-space: nowrap; }
  .btn-icon {
    background: #1e293b; border: 1px solid #334155; color: #94a3b8;
    width: 36px; height: 36px; border-radius: 8px; cursor: pointer;
    font-size: 1rem; display: flex; align-items: center; justify-content: center;
  }
  .btn-icon:hover { border-color: #64748b; color: #f1f5f9; }
  .btn-refresh {
    background: #1e3a5f; border: 1px solid #2563eb; color: #93c5fd;
    padding: .45rem 1rem; border-radius: 8px; font-size: .88rem; cursor: pointer;
  }
  .btn-refresh:hover:not(:disabled) { background: #1e40af; }
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
  .subtab.active { color: #60a5fa; border-bottom-color: #60a5fa; }

  /* Modal */
  .modal-backdrop {
    position: fixed; inset: 0; background: rgba(0,0,0,.65);
    display: flex; align-items: center; justify-content: center;
    z-index: 100; padding: 1rem;
  }
  .modal {
    background: #1e293b; border: 1px solid #334155; border-radius: 12px;
    width: 100%; max-width: 580px; max-height: 80vh;
    display: flex; flex-direction: column;
  }
  .modal-header {
    display: flex; align-items: center; justify-content: space-between;
    padding: 1rem 1.25rem; border-bottom: 1px solid #334155; flex-shrink: 0;
  }
  .modal-header h3 { margin: 0; font-size: 1.05rem; color: #f1f5f9; }
  .btn-close {
    background: none; border: none; color: #64748b; cursor: pointer;
    font-size: 1rem; padding: .2rem .4rem; border-radius: 4px;
  }
  .btn-close:hover { color: #f1f5f9; background: #334155; }
  .modal-hint {
    padding: .6rem 1.25rem 0; font-size: .78rem; color: #64748b; flex-shrink: 0;
  }
  .picker-loading { padding: 2rem; text-align: center; color: #64748b; }
  .modal-body { overflow-y: auto; padding: .75rem 1.25rem 1.25rem; flex: 1; }

  /* Region sections */
  .region-section { margin-bottom: 1.1rem; }
  .region-label {
    font-size: .7rem; font-weight: 700; letter-spacing: .08em;
    color: #475569; text-transform: uppercase; margin-bottom: .4rem;
  }
  .region-leagues { display: flex; flex-direction: column; gap: .3rem; }

  .league-row {
    display: flex; align-items: center; gap: .7rem;
    padding: .45rem .7rem; border-radius: 8px; cursor: pointer;
    border: 1px solid #334155; background: #0f172a;
    transition: border-color .15s, background .15s;
  }
  .league-row:hover { background: #1e293b; border-color: #475569; }
  .league-row.enabled { border-color: #334155; }
  .league-row input { display: none; }
  .league-logo { width: 22px; height: 22px; object-fit: contain; flex-shrink: 0; }
  .league-name { flex: 1; font-size: .88rem; color: #cbd5e1; }
  .league-check {
    width: 18px; height: 18px; border-radius: 4px; border: 1.5px solid;
    display: flex; align-items: center; justify-content: center;
    font-size: .7rem; font-weight: 700; color: #0f172a; flex-shrink: 0;
    transition: background .15s;
  }

  /* Schedule */
  .schedule { display: flex; flex-direction: column; gap: 1.5rem; }
  .day-label {
    font-size: .8rem; font-weight: 600; color: #64748b;
    text-transform: capitalize; letter-spacing: .05em;
    margin-bottom: .6rem; display: flex; align-items: center; gap: .5rem;
  }
  .today .day-label { color: #60a5fa; }
  .today-badge {
    background: #1e3a5f; color: #60a5fa; font-size: .7rem;
    padding: .1rem .4rem; border-radius: 4px; font-weight: 700;
  }
  .match-list { display: flex; flex-direction: column; gap: .5rem; }

  /* Match card */
  .match-card {
    background: #1e293b; border: 1px solid #334155;
    border-left: 3px solid var(--accent); border-radius: 10px;
    padding: .6rem 1rem;
    display: grid; grid-template-columns: auto 1fr auto;
    grid-template-rows: auto auto; align-items: center; gap: .2rem .75rem;
  }
  .match-card.live { border-left-color: #ef4444; }
  .match-card.completed { opacity: .85; }

  .match-league {
    grid-column: 1; grid-row: 1;
    font-size: .68rem; font-weight: 700; letter-spacing: .05em;
    padding: .15rem .45rem; border-radius: 4px; white-space: nowrap;
  }
  .match-stage {
    grid-column: 2; grid-row: 1;
    font-size: .8rem; color: #94a3b8;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .match-time-vod {
    grid-column: 3; grid-row: 1;
    display: flex; align-items: center; gap: .5rem;
    justify-content: flex-end; white-space: nowrap;
    font-size: .88rem; color: #cbd5e1; font-weight: 600;
  }
  .btn-vod {
    background: #1e293b; border: 1px solid #334155; color: #64748b;
    padding: .15rem .5rem; border-radius: 5px; font-size: .72rem;
    cursor: pointer; white-space: nowrap; font-weight: 600;
  }
  .btn-vod:hover { border-color: #60a5fa; color: #60a5fa; }
  .btn-live {
    display: inline-flex; align-items: center; gap: .35rem;
    background: #3f0f0f; border: 1px solid #ef4444; color: #fca5a5;
    padding: .2rem .6rem; border-radius: 5px; font-size: .75rem; font-weight: 700;
    text-decoration: none; letter-spacing: .04em; white-space: nowrap;
  }
  .btn-live:hover { background: #7f1d1d; }
  .live-dot {
    width: 7px; height: 7px; border-radius: 50%; background: #ef4444;
    animation: pulse 1.2s ease-in-out infinite;
    flex-shrink: 0;
  }
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: .3; }
  }

  .match-teams {
    grid-column: 1 / -1; grid-row: 2;
    display: flex; align-items: center; gap: .5rem;
  }
  .team { display: flex; align-items: center; gap: .45rem; flex: 1; }
  .team-right { flex-direction: row-reverse; }
  .team-logo { width: 28px; height: 28px; object-fit: contain; }
  .team-code { font-size: .92rem; font-weight: 700; color: #e2e8f0; }
  .team.winner .team-code { color: #4ade80; }
  .team.loser  .team-code { color: #475569; }

  .match-vs {
    display: flex; flex-direction: column; align-items: center;
    min-width: 80px; gap: .2rem;
  }
  .score { font-size: 1.05rem; font-weight: 700; color: #f1f5f9; letter-spacing: .05em; }
  .bo    { font-size: .8rem; color: #475569; }

  .team-placeholder { font-size: .82rem; color: #334155; font-weight: 600; letter-spacing: .1em; }
  .hidden-team { opacity: .4; }

  .btn-reveal-spoiler {
    display: flex; align-items: center; gap: .3rem;
    background: #1e3a5f; border: 1px solid #3b82f6; color: #93c5fd;
    padding: .25rem .6rem; border-radius: 6px; font-size: .75rem;
    cursor: pointer; white-space: nowrap;
  }
  .btn-reveal-spoiler:hover { background: #1e40af; }

  .btn-reveal-score {
    display: flex; align-items: center; gap: .3rem;
    background: #1e293b; border: 1px solid #475569; color: #94a3b8;
    padding: .2rem .5rem; border-radius: 6px; font-size: .72rem;
    cursor: pointer; white-space: nowrap;
  }
  .btn-reveal-score:hover { border-color: #64748b; color: #f1f5f9; }

  .eye-icon { font-size: .85rem; }


  /* VOD modal */
  .vod-modal { max-width: 760px; }
  .vod-body { overflow-y: auto; padding: 1rem 1.25rem 1.25rem; flex: 1; display: flex; flex-direction: column; gap: 1.5rem; }
  .vod-game { display: flex; flex-direction: column; gap: .5rem; }
  .vod-game-header { display: flex; align-items: center; justify-content: space-between; }
  .vod-game-label { font-size: .85rem; font-weight: 700; color: #94a3b8; }
  .vod-ext-link {
    font-size: .78rem; color: #60a5fa; text-decoration: none;
    border: 1px solid #1e3a5f; padding: .15rem .5rem; border-radius: 5px;
  }
  .vod-ext-link:hover { background: #1e3a5f; }
  .vod-iframe {
    width: 100%; aspect-ratio: 16/9; border-radius: 8px; border: 1px solid #334155;
  }
  .vod-twitch-link {
    display: block; padding: .75rem 1rem; background: #1e293b; border: 1px solid #334155;
    border-radius: 8px; color: #a78bfa; text-decoration: none; text-align: center;
  }
  .vod-twitch-link:hover { background: #1e293b; border-color: #a78bfa; }
  .vod-state { text-align: center; padding: 2rem; color: #64748b; }
  .vod-state.error { color: #f87171; }

  /* States */
  .state-msg { text-align: center; padding: 3rem; color: #64748b; font-size: .95rem; }
  .state-msg.error { color: #f87171; }
</style>
