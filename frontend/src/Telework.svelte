<script>
  import { onMount } from 'svelte';

  const CURRENT_YEAR = new Date().getFullYear();
  const YEARS = Array.from({ length: 6 }, (_, i) => CURRENT_YEAR - i);
  const MONTH_NAMES = [
    'Janvier', 'Février', 'Mars', 'Avril', 'Mai', 'Juin',
    'Juillet', 'Août', 'Septembre', 'Octobre', 'Novembre', 'Décembre'
  ];
  const WEEKDAYS = [
    { value: 1, label: 'Lun' },
    { value: 2, label: 'Mar' },
    { value: 3, label: 'Mer' },
    { value: 4, label: 'Jeu' },
    { value: 5, label: 'Ven' },
  ];

  let selectedYear = CURRENT_YEAR;
  let summary = null;
  let preset = { remote_days: [4, 5] };
  let loading = true;
  let error = '';

  // Drag state
  let dragActive = false;
  let dragStart = null; // YYYY-MM-DD
  let dragEnd = null;   // YYYY-MM-DD

  // Reactive set of highlighted dates — recomputed whenever dragStart/dragEnd change
  $: dragRangeSet = (() => {
    if (!dragActive || !dragStart || !dragEnd || !summary) return new Set();
    const a = dragStart < dragEnd ? dragStart : dragEnd;
    const b = dragStart < dragEnd ? dragEnd   : dragStart;
    return new Set(
      summary.days
        .filter(d => d.date >= a && d.date <= b)
        .map(d => d.date)
    );
  })();

  // Modal state
  let modalOpen = false;
  let modalDates = []; // ouvrables sélectionnées (non-weekend, non-fériés)
  let modalSaving = false;
  let modalType = 'leave'; // 'leave' | 'remote' | 'office' | 'clear'

  // Preset editing
  let editingPreset = false;
  let draftPresetDays = [];

  // Calendar container ref for mousemove tracking
  let calendarEl;

  onMount(() => {
    loadAll();
    window.addEventListener('mouseup', onWindowMouseUp);
    return () => window.removeEventListener('mouseup', onWindowMouseUp);
  });

  async function loadAll() {
    loading = true;
    error = '';
    try {
      const [presetRes, summaryRes] = await Promise.all([
        fetch('/api/telework/preset'),
        fetch(`/api/telework/summary/${selectedYear}`)
      ]);
      if (!presetRes.ok || !summaryRes.ok) throw new Error('Erreur serveur');
      preset = await presetRes.json();
      summary = await summaryRes.json();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function reloadSummary() {
    try {
      const res = await fetch(`/api/telework/summary/${selectedYear}`);
      if (!res.ok) throw new Error('Erreur serveur');
      summary = await res.json();
    } catch (e) {
      error = e.message;
    }
  }

  function changeYear(y) {
    selectedYear = y;
    dragActive = false; dragStart = null; dragEnd = null;
    closeModal();
    reloadSummary();
  }

  $: dayMap = summary
    ? Object.fromEntries(summary.days.map(d => [d.date, d]))
    : {};

  $: holidayLabels = summary
    ? Object.fromEntries(summary.holidays.map(h => [h.date, h.label]))
    : {};

  $: totalLeaves = summary
    ? summary.days.filter(d => d.is_leave).length
    : 0;

  $: months = summary ? buildMonths(summary.days) : [];

  function buildMonths(days) {
    const result = Array.from({ length: 12 }, (_, i) => ({ month: i, days: [], grid: [] }));
    for (const day of days) {
      const m = new Date(day.date + 'T00:00:00Z').getUTCMonth();
      result[m].days.push(day);
    }
    for (const mo of result) mo.grid = buildGrid(mo.days);
    return result;
  }

  function buildGrid(days) {
    if (!days.length) return [];
    const first = new Date(days[0].date + 'T00:00:00Z');
    const offset = (first.getUTCDay() + 6) % 7; // Mon=0…Sun=6
    const cells = Array(offset).fill(null).concat(days);
    const rows = [];
    for (let i = 0; i < cells.length; i += 7)
      rows.push(cells.slice(i, i + 7).concat(Array(7).fill(null)).slice(0, 7));
    return rows;
  }

  // --- Drag ---


  function dateFromElement(el) {
    // Walk up to find a [data-date] attribute
    let cur = el;
    while (cur && cur !== calendarEl) {
      if (cur.dataset?.date) return cur.dataset.date;
      cur = cur.parentElement;
    }
    return null;
  }

  function onCalendarMouseDown(e) {
    const date = dateFromElement(e.target);
    if (!date) return;
    const day = dayMap[date];
    if (!day || day.is_weekend || day.is_holiday) return;
    e.preventDefault();
    dragActive = true;
    dragStart = date;
    dragEnd = date;
  }

  function onCalendarClick(e) {
    // Single-day click (no drag occurred): open modal pre-configured for that day.
    if (dragActive) return; // drag is in progress, handled by mouseup
    const date = dateFromElement(e.target);
    if (!date) return;
    const day = dayMap[date];
    if (!day || day.is_weekend || day.is_holiday) return;
    openModal([date], true);
  }

  function onCalendarMouseMove(e) {
    if (!dragActive) return;
    const date = dateFromElement(e.target);
    if (date && date !== dragEnd) dragEnd = date; // assignment triggers Svelte reactivity
  }

  function onWindowMouseUp(e) {
    if (!dragActive) return;

    const from = dragStart < dragEnd ? dragStart : dragEnd;
    const to   = dragStart < dragEnd ? dragEnd   : dragStart;
    const wasSingleCell = from === to;
    dragActive = false;
    dragStart = null;
    dragEnd = null;

    if (!summary || !from || !to) return;

    const rangeDates = summary.days
      .filter(d => d.date >= from && d.date <= to && !d.is_weekend && !d.is_holiday)
      .map(d => d.date);

    if (!rangeDates.length) return;
    openModal(rangeDates, wasSingleCell);
  }

  // --- Modal ---

  function openModal(dates, singleDay = false) {
    modalDates = dates;
    // Pre-select based on current state: single day uses its own state,
    // multi-day defaults to 'leave' unless all days share the same override.
    if (singleDay && dates.length === 1) {
      const day = dayMap[dates[0]];
      if (day?.override_type) {
        modalType = day.override_type;
      } else if (day?.is_remote) {
        modalType = 'remote';
      } else {
        modalType = 'office';
      }
    } else {
      // Check if all selected days share the same override — if so, pre-select it
      const types = new Set(dates.map(d => dayMap[d]?.override_type || ''));
      if (types.size === 1 && [...types][0]) {
        modalType = [...types][0];
      } else {
        modalType = 'leave';
      }
    }
    modalOpen = true;
  }

  function closeModal() {
    modalOpen = false;
    modalDates = [];
  }

  async function modalApply() {
    modalSaving = true;
    // Build the full overrides map from current summary state
    const overrides = {};
    for (const d of summary.days) {
      if (d.override_type) overrides[d.date] = d.override_type;
    }
    if (modalType === 'clear') {
      for (const d of modalDates) delete overrides[d];
    } else {
      for (const d of modalDates) overrides[d] = modalType;
    }
    try {
      const res = await fetch(`/api/telework/overrides/${selectedYear}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(overrides)
      });
      if (!res.ok) throw new Error('Erreur sauvegarde');
      await reloadSummary();
      closeModal();
    } catch (e) {
      error = e.message;
    } finally {
      modalSaving = false;
    }
  }

  // --- Preset ---

  function startEditPreset() {
    draftPresetDays = [...preset.remote_days];
    editingPreset = true;
  }

  function toggleDraftDay(d) {
    draftPresetDays = draftPresetDays.includes(d)
      ? draftPresetDays.filter(x => x !== d)
      : [...draftPresetDays, d];
  }

  async function savePreset() {
    try {
      const res = await fetch('/api/telework/preset', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ remote_days: draftPresetDays })
      });
      if (!res.ok) throw new Error('Erreur sauvegarde preset');
      preset = { remote_days: [...draftPresetDays] };
      editingPreset = false;
      await reloadSummary();
    } catch (e) {
      error = e.message;
    }
  }

  function fmt(pct) {
    return pct.toFixed(2).replace('.', ',');
  }

  function dayTitle(day) {
    if (!day) return '';
    if (day.is_holiday) return holidayLabels[day.date] || 'Férié';
    if (day.is_leave) return 'Congé';
    if (day.override_type === 'remote') return 'Télétravail (override)';
    if (day.override_type === 'office') return 'Bureau (override)';
    if (day.is_remote) return 'Télétravail (preset)';
    if (!day.is_weekend) return 'Bureau (preset)';
    return '';
  }

  function fmtDate(d) {
    const [y, m, day] = d.split('-');
    return `${day}/${m}/${y}`;
  }

  function modalTitle() {
    if (modalDates.length === 1) return fmtDate(modalDates[0]);
    return `${fmtDate(modalDates[0])} → ${fmtDate(modalDates[modalDates.length - 1])}`;
  }
</script>

<!-- svelte-ignore a11y-no-static-element-interactions -->
<div class="telework">
  <div class="tw-header">
    <div class="tw-title-row">
      <h2>Calculateur Télétravail</h2>
      <span class="canton-badge">Canton de Genève</span>
      <select class="year-select" value={selectedYear} on:change={e => changeYear(+e.target.value)}>
        {#each YEARS as y}
          <option value={y}>{y}</option>
        {/each}
      </select>
    </div>

    {#if error}
      <div class="tw-error">{error}</div>
    {/if}

    {#if summary}
      <div class="stats-bar">
        <div class="stat">
          <span class="stat-label">Jours travaillés</span>
          <span class="stat-val">{summary.total_worked}</span>
        </div>
        <div class="stat">
          <span class="stat-label">En présence</span>
          <span class="stat-val">{summary.total_on_site}</span>
        </div>
        <div class="stat">
          <span class="stat-label">Télétravail</span>
          <span class="stat-val">{summary.total_remote}</span>
        </div>
        <div class="stat">
          <span class="stat-label">Congés</span>
          <span class="stat-val">{totalLeaves}</span>
        </div>
        <div class="stat" class:over-pct={summary.over_threshold}>
          <span class="stat-label">% TT</span>
          <span class="stat-val">{fmt(summary.remote_pct)} %</span>
        </div>
        <div class="stat">
          <span class="stat-label">% Présence</span>
          <span class="stat-val">{fmt(summary.on_site_pct)} %</span>
        </div>
        {#if summary.over_threshold}
          <div class="alert-recover">
            ⚠ TT > 40 % — encore <strong>{summary.days_to_recover}</strong> jour{summary.days_to_recover > 1 ? 's' : ''} en présence à rattraper
          </div>
        {:else}
          <div class="alert-ok">✓ TT ≤ 40 %</div>
        {/if}
      </div>
    {/if}

    <div class="preset-bar">
      <span class="preset-label">Preset TT :</span>
      {#if !editingPreset}
        <div class="preset-days">
          {#each WEEKDAYS as wd}
            <span class="preset-day" class:active={preset.remote_days.includes(wd.value)}>{wd.label}</span>
          {/each}
        </div>
        <button class="btn-sm" on:click={startEditPreset}>Modifier</button>
      {:else}
        <div class="preset-days">
          {#each WEEKDAYS as wd}
            <button
              class="preset-day-btn"
              class:active={draftPresetDays.includes(wd.value)}
              on:click={() => toggleDraftDay(wd.value)}
            >{wd.label}</button>
          {/each}
        </div>
        <button class="btn-sm btn-primary" on:click={savePreset}>Appliquer</button>
        <button class="btn-sm" on:click={() => editingPreset = false}>Annuler</button>
      {/if}
    </div>

    <div class="legend-row">
      <div class="legend">
        <span class="leg-item"><span class="leg-swatch leg-weekend"></span>Weekend</span>
        <span class="leg-item"><span class="leg-swatch leg-holiday"></span>Férié</span>
        <span class="leg-item"><span class="leg-swatch leg-leave"></span>Congé</span>
        <span class="leg-item"><span class="leg-swatch leg-remote"></span>Télétravail</span>
        <span class="leg-item"><span class="leg-swatch leg-office"></span>Bureau</span>
        <span class="leg-item"><span class="leg-override-dot"></span>Override manuel</span>
      </div>
      <div class="range-hint">Cliquer pour éditer un jour · Glisser pour sélectionner une plage</div>
    </div>
  </div>

  {#if loading}
    <div class="tw-loading">Chargement…</div>
  {:else if summary}
    <div
      class="calendar-grid"
      class:dragging={dragActive}
      bind:this={calendarEl}
      on:mousedown={onCalendarMouseDown}
      on:mousemove={onCalendarMouseMove}
      on:click={onCalendarClick}
      on:keydown={() => {}}
      role="grid"
      tabindex="0"
    >
      {#each months as mo, mi}
        <div class="month-block">
          <div class="month-name">{MONTH_NAMES[mi]}</div>
          <div class="dow-row">
            {#each ['L','M','M','J','V','S','D'] as d}
              <div class="dow">{d}</div>
            {/each}
          </div>
          {#each mo.grid as week}
            <div class="week-row">
              {#each week as day}
                {#if day}
                  <div
                    class="day-cell"
                    class:weekend={day.is_weekend}
                    class:holiday={day.is_holiday}
                    class:leave={day.is_leave && !day.is_holiday}
                    class:remote={day.is_remote && !day.is_leave && !day.is_holiday}
                    class:office={!day.is_weekend && !day.is_holiday && !day.is_leave && !day.is_remote}
                    class:override={!!day.override_type && !day.is_holiday}
                    class:selectable={!day.is_weekend && !day.is_holiday}
                    class:in-drag={dragRangeSet.has(day.date)}
                    title={dayTitle(day)}
                    data-date={day.date}
                  >
                    <span class="day-num">{new Date(day.date + 'T00:00:00Z').getUTCDate()}</span>
                  </div>
                {:else}
                  <div class="day-cell empty"></div>
                {/if}
              {/each}
            </div>
          {/each}
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- Modal -->
{#if modalOpen}
  <div class="modal-backdrop" on:click|self={closeModal} on:keydown={() => {}} role="presentation">
    <div class="modal">
      <div class="modal-header">
        <span class="modal-title">{modalTitle()}</span>
        <span class="modal-count">{modalDates.length} jour{modalDates.length > 1 ? 's' : ''} ouvrable{modalDates.length > 1 ? 's' : ''}</span>
      </div>

      <div class="modal-body">
        <label class="modal-label" for="modal-type-select">Type</label>
        <select
          id="modal-type-select"
          class="modal-select"
          bind:value={modalType}
          disabled={modalSaving}
        >
          <option value="leave">Congé</option>
          <option value="remote">Télétravail</option>
          <option value="office">Bureau</option>
          {#if modalDates.some(d => dayMap[d]?.override_type)}
            <option value="clear">Réinitialiser (suivre le preset)</option>
          {/if}
        </select>
      </div>

      <div class="modal-footer">
        <button class="btn-primary-full" on:click={modalApply} disabled={modalSaving}>
          {modalSaving ? 'Sauvegarde…' : 'Appliquer'}
        </button>
        <button class="btn-cancel" on:click={closeModal} disabled={modalSaving}>Annuler</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .telework {
    max-width: 1400px;
    margin: 0 auto;
  }

  .tw-header {
    display: flex;
    flex-direction: column;
    gap: .75rem;
    margin-bottom: 1.5rem;
  }

  .tw-title-row {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  h2 {
    margin: 0;
    font-size: 1.4rem;
    font-weight: 600;
    color: #f1f5f9;
  }

  .canton-badge {
    font-size: .75rem;
    color: #64748b;
    border: 1px solid #334155;
    border-radius: 4px;
    padding: .2rem .55rem;
  }

  .year-select {
    background: #1e293b;
    color: #f1f5f9;
    border: 1px solid #334155;
    border-radius: 6px;
    padding: .35rem .7rem;
    font-size: .95rem;
    cursor: pointer;
  }

  .tw-error {
    background: #450a0a;
    color: #fca5a5;
    padding: .5rem 1rem;
    border-radius: 6px;
    font-size: .9rem;
  }

  /* Stats */
  .stats-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 1.5rem;
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 10px;
    padding: .85rem 1.2rem;
  }

  .stat { display: flex; flex-direction: column; gap: .1rem; }

  .stat-label {
    font-size: .72rem;
    color: #94a3b8;
    text-transform: uppercase;
    letter-spacing: .04em;
  }

  .stat-val {
    font-size: 1.25rem;
    font-weight: 700;
    color: #f1f5f9;
  }

  .over-pct .stat-val { color: #f87171; }

  .alert-recover {
    margin-left: auto;
    background: #450a0a;
    color: #fca5a5;
    border: 1px solid #7f1d1d;
    border-radius: 8px;
    padding: .5rem 1rem;
    font-size: .88rem;
  }

  .alert-ok {
    margin-left: auto;
    color: #34d399;
    font-size: .9rem;
    font-weight: 600;
  }

  /* Preset */
  .preset-bar { display: flex; align-items: center; gap: .75rem; flex-wrap: wrap; }
  .preset-label { font-size: .85rem; color: #94a3b8; }
  .preset-days { display: flex; gap: .35rem; }

  .preset-day {
    padding: .2rem .5rem;
    border-radius: 4px;
    font-size: .8rem;
    background: #1e293b;
    color: #64748b;
    border: 1px solid #334155;
  }
  .preset-day.active { background: #1e3a5f; color: #60a5fa; border-color: #3b82f6; }

  .preset-day-btn {
    padding: .2rem .55rem;
    border-radius: 4px;
    font-size: .8rem;
    background: #1e293b;
    color: #64748b;
    border: 1px solid #334155;
    cursor: pointer;
  }
  .preset-day-btn.active { background: #1e3a5f; color: #60a5fa; border-color: #3b82f6; }

  .btn-sm {
    padding: .25rem .7rem;
    border-radius: 5px;
    font-size: .82rem;
    background: #1e293b;
    color: #94a3b8;
    border: 1px solid #334155;
    cursor: pointer;
  }
  .btn-sm:hover { border-color: #64748b; color: #f1f5f9; }
  .btn-primary { background: #1e3a5f; color: #60a5fa; border-color: #3b82f6; }

  /* Legend */
  .legend-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: .5rem;
  }

  .legend {
    display: flex;
    align-items: center;
    gap: .75rem;
    font-size: .8rem;
    color: #94a3b8;
    flex-wrap: wrap;
  }

  .leg-item { display: flex; align-items: center; gap: .3rem; }

  .leg-swatch {
    display: inline-block;
    width: 12px;
    height: 12px;
    border-radius: 3px;
    flex-shrink: 0;
  }

  .leg-weekend { background: #0f1e30; border: 1px solid #334155; }
  .leg-holiday { background: #4a1d4e; border: 1px solid #7e22a3; }
  .leg-leave   { background: #1c3b2e; border: 1px solid #16a34a; }
  .leg-remote  { background: #1e3a5f; border: 1px solid #3b82f6; }
  .leg-office  { background: #1c1c12; border: 1px solid #78716c; }

  .leg-override-dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #f59e0b;
    flex-shrink: 0;
  }

  .range-hint { font-size: .8rem; color: #64748b; }
  .tw-loading { text-align: center; color: #64748b; padding: 3rem; }

  /* Calendar */
  .calendar-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1.25rem;
  }

  .calendar-grid.dragging { cursor: crosshair; }

  @media (max-width: 1100px) { .calendar-grid { grid-template-columns: repeat(3, 1fr); } }
  @media (max-width: 750px)  { .calendar-grid { grid-template-columns: repeat(2, 1fr); } }
  @media (max-width: 450px)  { .calendar-grid { grid-template-columns: 1fr; } }

  .month-block {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 10px;
    padding: .75rem;
  }

  .month-name {
    font-weight: 600;
    font-size: .9rem;
    color: #cbd5e1;
    margin-bottom: .5rem;
    text-align: center;
  }

  .dow-row {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    margin-bottom: .2rem;
  }

  .dow {
    text-align: center;
    font-size: .65rem;
    color: #64748b;
    padding: .15rem 0;
    font-weight: 600;
  }

  .week-row {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 1px;
    margin-bottom: 1px;
  }

  .day-cell {
    aspect-ratio: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    position: relative;
    font-size: .75rem;
    color: #94a3b8;
    user-select: none;
  }

  .day-cell.empty { background: transparent; }

  .day-cell.weekend {
    color: #475569;
    background:
      linear-gradient(to bottom right, transparent calc(50% - 0.5px), #334155 calc(50% - 0.5px), #334155 calc(50% + 0.5px), transparent calc(50% + 0.5px)),
      linear-gradient(to bottom left,  transparent calc(50% - 0.5px), #334155 calc(50% - 0.5px), #334155 calc(50% + 0.5px), transparent calc(50% + 0.5px)),
      #0f1e30;
  }

  .day-cell.holiday {
    background: #4a1d4e;
    color: #e879f9;
    font-weight: 600;
  }

  .day-cell.leave {
    background: #1c3b2e;
    color: #4ade80;
  }

  .day-cell.remote {
    background: #1e3a5f;
    color: #93c5fd;
  }

  .day-cell.office {
    background: #1c1c12;
    color: #a8a29e;
  }

  .day-cell.selectable { cursor: pointer; }
  .day-cell.selectable:hover { filter: brightness(1.35); }

  /* Small dot in top-right corner to signal a manual override */
  .day-cell.override::after {
    content: '';
    position: absolute;
    top: 2px;
    right: 2px;
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: #f59e0b;
  }

  .day-cell.in-drag {
    outline: 2px solid #f59e0b;
    filter: brightness(1.3);
  }

  .day-num { line-height: 1; }

  /* Modal */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,.55);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }

  .modal {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 12px;
    padding: 1.5rem;
    min-width: 300px;
    max-width: 420px;
    width: 90%;
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  .modal-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 1rem;
  }

  .modal-title {
    font-size: 1.05rem;
    font-weight: 600;
    color: #f1f5f9;
  }

  .modal-count {
    font-size: .82rem;
    color: #64748b;
  }

  .modal-body {
    display: flex;
    flex-direction: column;
    gap: .5rem;
  }

  .modal-label {
    font-size: .78rem;
    color: #94a3b8;
    text-transform: uppercase;
    letter-spacing: .04em;
  }

  .modal-select {
    background: #0f172a;
    color: #f1f5f9;
    border: 1px solid #334155;
    border-radius: 6px;
    padding: .5rem .75rem;
    font-size: .95rem;
    cursor: pointer;
    width: 100%;
  }
  .modal-select:focus { outline: none; border-color: #3b82f6; }

  .modal-footer {
    display: flex;
    gap: .6rem;
    justify-content: flex-end;
    flex-wrap: wrap;
  }

  .btn-primary-full {
    background: #1e3a5f;
    color: #60a5fa;
    border: 1px solid #3b82f6;
    border-radius: 6px;
    padding: .45rem 1rem;
    font-size: .9rem;
    cursor: pointer;
    font-weight: 500;
  }
  .btn-primary-full:hover { background: #1d4ed8; color: #fff; }
  .btn-primary-full:disabled { opacity: .5; cursor: default; }

  .btn-cancel {
    background: transparent;
    color: #64748b;
    border: 1px solid #334155;
    border-radius: 6px;
    padding: .45rem 1rem;
    font-size: .9rem;
    cursor: pointer;
  }
  .btn-cancel:hover { color: #94a3b8; border-color: #64748b; }
  .btn-cancel:disabled { opacity: .5; cursor: default; }
</style>
