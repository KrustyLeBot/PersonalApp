<script>
  import { onMount } from 'svelte';

  const CURRENT_YEAR = new Date().getFullYear();
  const YEARS = Array.from({ length: 6 }, (_, i) => CURRENT_YEAR - i);
  const MONTH_NAMES = [
    'Janvier', 'Février', 'Mars', 'Avril', 'Mai', 'Juin',
    'Juillet', 'Août', 'Septembre', 'Octobre', 'Novembre', 'Décembre'
  ];

  let selectedYear = CURRENT_YEAR;
  let summary = null;
  let loading = true;
  let error = '';

  // Quick entry form
  let formDate = todayISO();
  let formKm = '';
  let saving = false;

  // Day edit modal
  let modalOpen = false;
  let modalDate = '';
  let modalKm = '';
  let modalSaving = false;

  onMount(loadSummary);

  function todayISO() {
    const d = new Date();
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  }

  async function loadSummary() {
    error = '';
    try {
      const res = await fetch(`/api/walk/summary/${selectedYear}`);
      if (!res.ok) throw new Error('Erreur serveur');
      summary = await res.json();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  function changeYear(y) {
    selectedYear = y;
    closeModal();
    loadSummary();
  }

  async function saveDay(date, km) {
    const res = await fetch('/api/walk/day', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ date, km })
    });
    if (!res.ok) throw new Error('Erreur sauvegarde');
    await loadSummary();
  }

  function parseKm(raw) {
    return parseFloat(String(raw).trim().replace(',', '.'));
  }

  async function submitForm() {
    const km = parseKm(formKm);
    if (!formDate || Number.isNaN(km)) return;
    saving = true;
    try {
      await saveDay(formDate, km);
      formKm = '';
      // Jump to the year of the entry so the calendar shows it.
      const y = +formDate.slice(0, 4);
      if (y !== selectedYear && YEARS.includes(y)) changeYear(y);
    } catch (e) {
      error = e.message;
    } finally {
      saving = false;
    }
  }

  function openModal(day) {
    if (day.is_future) return;
    modalDate = day.date;
    modalKm = day.km > 0 ? String(day.km).replace('.', ',') : '';
    modalOpen = true;
  }

  function closeModal() {
    modalOpen = false;
    modalDate = '';
  }

  async function modalApply() {
    const raw = String(modalKm).trim();
    const km = raw === '' ? 0 : parseKm(raw);
    if (Number.isNaN(km)) return;
    modalSaving = true;
    try {
      await saveDay(modalDate, km);
      closeModal();
    } catch (e) {
      error = e.message;
    } finally {
      modalSaving = false;
    }
  }

  $: months = summary ? buildMonths(summary.days) : [];

  function buildMonths(days) {
    const result = Array.from({ length: 12 }, () => ({ days: [], grid: [] }));
    for (const day of days) {
      result[+day.date.slice(5, 7) - 1].days.push(day);
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

  function fmtKm(n) {
    return (n ?? 0).toFixed(2).replace(/\.?0+$/, '').replace('.', ',') || '0';
  }

  function fmtDate(d) {
    const [y, m, day] = d.split('-');
    return `${day}/${m}/${y}`;
  }

  function dayTitle(day) {
    return day.km > 0 ? `${fmtDate(day.date)} — ${fmtKm(day.km)} km` : fmtDate(day.date);
  }
</script>

<div class="walk">
  <div class="wk-header">
    <div class="wk-title-row">
      <h2>Marche</h2>
      <span class="unit-badge">km / jour</span>
      <select class="year-select" value={selectedYear} on:change={e => changeYear(+e.target.value)}>
        {#each YEARS as y}
          <option value={y}>{y}</option>
        {/each}
      </select>
    </div>

    {#if error}
      <div class="wk-error">{error}</div>
    {/if}

    <form class="entry-form" on:submit|preventDefault={submitForm}>
      <span class="form-label">Saisie rapide</span>
      <input class="input date" type="date" bind:value={formDate} max={todayISO()} required />
      <input
        class="input km"
        type="text"
        inputmode="decimal"
        placeholder="km"
        bind:value={formKm}
        required
      />
      <button class="btn-primary" type="submit" disabled={saving}>
        {saving ? 'Enregistrement…' : 'Enregistrer'}
      </button>
    </form>

    {#if summary}
      <div class="stats-bar">
        <div class="stat">
          <span class="stat-label">Semaine en cours</span>
          <span class="stat-val">{fmtKm(summary.week_total)} km</span>
        </div>
        <div class="stat">
          <span class="stat-label">Mois en cours</span>
          <span class="stat-val">{fmtKm(summary.month_total)} km</span>
        </div>
        <div class="stat">
          <span class="stat-label">Année {summary.year}</span>
          <span class="stat-val">{fmtKm(summary.year_total)} km</span>
        </div>
        <div class="stat">
          <span class="stat-label">Jours actifs</span>
          <span class="stat-val">{summary.active_days}</span>
        </div>
        <div class="stat">
          <span class="stat-label">Moy. / jour actif</span>
          <span class="stat-val">{fmtKm(summary.daily_avg)} km</span>
        </div>
        {#if summary.best_day.date}
          <div class="stat">
            <span class="stat-label">Meilleur jour</span>
            <span class="stat-val">{fmtKm(summary.best_day.km)} km</span>
            <span class="stat-sub">{fmtDate(summary.best_day.date)}</span>
          </div>
        {/if}
      </div>
    {/if}

    <div class="legend-row">
      <div class="legend">
        <span class="leg-item"><span class="leg-swatch leg-walked"></span>Marché</span>
        <span class="leg-item"><span class="leg-swatch leg-idle"></span>Rien</span>
        <span class="leg-item"><span class="leg-swatch leg-future"></span>À venir</span>
      </div>
      <div class="hint">Cliquer sur un jour pour éditer la distance</div>
    </div>
  </div>

  {#if loading}
    <div class="wk-loading">Chargement…</div>
  {:else if summary}
    <div class="calendar-grid">
      {#each months as mo, mi}
        <div class="month-block">
          <div class="month-head">
            <span class="month-name">{MONTH_NAMES[mi]}</span>
            <span class="month-total">{fmtKm(summary.month_totals[mi])} km</span>
          </div>
          <div class="dow-row">
            {#each ['L','M','M','J','V','S','D'] as d}
              <div class="dow">{d}</div>
            {/each}
          </div>
          {#each mo.grid as week}
            <div class="week-row">
              {#each week as day}
                {#if day}
                  <button
                    class="day-cell"
                    class:walked={day.km > 0}
                    class:future={day.is_future}
                    title={dayTitle(day)}
                    disabled={day.is_future}
                    on:click={() => openModal(day)}
                  >
                    <span class="day-num">{+day.date.slice(8, 10)}</span>
                  </button>
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
        <span class="modal-title">{fmtDate(modalDate)}</span>
      </div>
      <div class="modal-body">
        <label class="modal-label" for="modal-km">Distance (km)</label>
        <!-- svelte-ignore a11y-autofocus -->
        <input
          id="modal-km"
          class="modal-input"
          type="text"
          inputmode="decimal"
          placeholder="0"
          bind:value={modalKm}
          disabled={modalSaving}
          autofocus
          on:keydown={e => e.key === 'Enter' && modalApply()}
        />
        <span class="modal-hint">Laisser vide pour effacer la journée.</span>
      </div>
      <div class="modal-footer">
        <button class="btn-primary" on:click={modalApply} disabled={modalSaving}>
          {modalSaving ? 'Sauvegarde…' : 'Appliquer'}
        </button>
        <button class="btn-cancel" on:click={closeModal} disabled={modalSaving}>Annuler</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .walk { max-width: 1400px; margin: 0 auto; }

  .wk-header {
    display: flex;
    flex-direction: column;
    gap: .75rem;
    margin-bottom: 1.5rem;
  }

  .wk-title-row { display: flex; align-items: center; gap: 1rem; }

  h2 { margin: 0; font-size: 1.4rem; font-weight: 600; color: #f1f5f9; }

  .unit-badge {
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

  .wk-error {
    background: #450a0a;
    color: #fca5a5;
    padding: .5rem 1rem;
    border-radius: 6px;
    font-size: .9rem;
  }

  /* Quick entry */
  .entry-form {
    display: flex;
    align-items: center;
    gap: .6rem;
    flex-wrap: wrap;
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 10px;
    padding: .75rem 1rem;
  }

  .form-label {
    font-size: .72rem;
    color: #94a3b8;
    text-transform: uppercase;
    letter-spacing: .04em;
    margin-right: .25rem;
  }

  .input {
    background: #0f172a;
    color: #f1f5f9;
    border: 1px solid #334155;
    border-radius: 6px;
    padding: .45rem .7rem;
    font-size: .95rem;
    font-family: inherit;
  }
  .input:focus { outline: none; border-color: #3b82f6; }
  .input.km { width: 6.5rem; }

  .btn-primary {
    background: #1e3a5f;
    color: #60a5fa;
    border: 1px solid #3b82f6;
    border-radius: 6px;
    padding: .45rem 1rem;
    font-size: .9rem;
    font-weight: 500;
    font-family: inherit;
    cursor: pointer;
  }
  .btn-primary:hover { background: #1d4ed8; color: #fff; }
  .btn-primary:disabled { opacity: .5; cursor: default; }

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

  .stat-val { font-size: 1.25rem; font-weight: 700; color: #f1f5f9; }
  .stat-sub { font-size: .72rem; color: #64748b; }

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

  .leg-walked { background: #14532d; border: 1px solid #22c55e; }
  .leg-idle   { background: #17304d; border: 1px solid #38bdf8; }
  .leg-future { background: #0f1e30; border: 1px solid #334155; }

  .hint { font-size: .8rem; color: #64748b; }
  .wk-loading { text-align: center; color: #64748b; padding: 3rem; }

  /* Calendar */
  .calendar-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1.25rem;
  }

  @media (max-width: 1100px) { .calendar-grid { grid-template-columns: repeat(3, 1fr); } }
  @media (max-width: 750px)  { .calendar-grid { grid-template-columns: repeat(2, 1fr); } }
  @media (max-width: 450px)  { .calendar-grid { grid-template-columns: 1fr; } }

  .month-block {
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 10px;
    padding: .75rem;
  }

  .month-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: .5rem;
    margin-bottom: .5rem;
  }

  .month-name { font-weight: 600; font-size: .9rem; color: #cbd5e1; }
  .month-total { font-size: .72rem; color: #64748b; }

  .dow-row { display: grid; grid-template-columns: repeat(7, 1fr); margin-bottom: .2rem; }

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
    gap: 2px;
    margin-bottom: 2px;
  }

  .day-cell {
    aspect-ratio: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 4px;
    font-size: .75rem;
    font-family: inherit;
    color: #7dd3fc;
    background: #17304d;
    border: 1px solid #2c5a86;
    padding: 0;
    cursor: pointer;
    user-select: none;
  }

  .day-cell:hover:not(:disabled) { filter: brightness(1.35); }

  .day-cell.walked {
    background: #14532d;
    border-color: #22c55e;
    color: #bbf7d0;
    font-weight: 600;
  }

  .day-cell.future {
    background: #0f1e30;
    border-color: #334155;
    color: #475569;
    cursor: default;
  }

  .day-cell.empty { background: transparent; border: none; cursor: default; }

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
    min-width: 280px;
    max-width: 380px;
    width: 90%;
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  .modal-title { font-size: 1.05rem; font-weight: 600; color: #f1f5f9; }

  .modal-body { display: flex; flex-direction: column; gap: .5rem; }

  .modal-label {
    font-size: .78rem;
    color: #94a3b8;
    text-transform: uppercase;
    letter-spacing: .04em;
  }

  .modal-input {
    background: #0f172a;
    color: #f1f5f9;
    border: 1px solid #334155;
    border-radius: 6px;
    padding: .5rem .75rem;
    font-size: .95rem;
    font-family: inherit;
    width: 100%;
  }
  .modal-input:focus { outline: none; border-color: #3b82f6; }

  .modal-hint { font-size: .75rem; color: #64748b; }

  .modal-footer { display: flex; gap: .6rem; justify-content: flex-end; flex-wrap: wrap; }

  .btn-cancel {
    background: transparent;
    color: #64748b;
    border: 1px solid #334155;
    border-radius: 6px;
    padding: .45rem 1rem;
    font-size: .9rem;
    font-family: inherit;
    cursor: pointer;
  }
  .btn-cancel:hover { color: #94a3b8; border-color: #64748b; }
  .btn-cancel:disabled { opacity: .5; cursor: default; }
</style>
