<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import {
    Chart, LineElement, PointElement, LineController,
    CategoryScale, LinearScale, Tooltip, Legend, Filler,
  } from 'chart.js';

  Chart.register(LineElement, PointElement, LineController, CategoryScale, LinearScale, Tooltip, Legend, Filler);

  export let confidential = false;

  let data = null;
  let loading = true;
  let error = '';
  let saving = {};
  let saveError = {};

  let chartEl;
  let chart;

  const HIDDEN = '••••';
  const TYPE_LABELS = {
    immobilier: 'Immobilier',
    fond_euro:  'Fonds Euro',
    livret:     'Livret',
    crypto:     'Crypto',
    bourse:     'Bourse',
  };

  const FLAT_TYPES = new Set(['immobilier', 'crypto']);
  const EDITABLE_KEYS = new Set(['livret_a', 'ldd', 'fond_euro']);

  const LINE_COLORS = [
    '#60a5fa','#34d399','#f59e0b','#a78bfa','#f87171',
    '#38bdf8','#fb923c','#4ade80','#e879f9','#fbbf24',
  ];

  onMount(load);
  onDestroy(() => chart?.destroy());

  async function load() {
    loading = true; error = '';
    try {
      const res = await fetch('/api/projection/summary');
      if (!res.ok) throw new Error(await res.text());
      data = await res.json();
      editableRates = Object.fromEntries(
        (data.rates || []).map(r => [r.key, { rate: r.rate }])
      );
      overrideRates = Object.fromEntries(
        (data.rates || []).map(r => [r.key, r.rate_override != null ? String(r.rate_override) : ''])
      );
      // Wait for Svelte to render the canvas (loading becomes false), then draw.
      await tick();
      renderChart();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
      // tick again after loading=false so canvas is in DOM
      await tick();
      renderChart();
    }
  }

  let editableRates = {};
  let overrideRates = {};    // key → string input value (empty = clear)
  let savingOverride = {};
  let overrideError = {};

  async function saveRateOverride(key) {
    savingOverride[key] = true; savingOverride = savingOverride;
    overrideError[key] = ''; overrideError = overrideError;
    try {
      const raw = overrideRates[key];
      const rate = raw === '' ? null : parseFloat(raw);
      const res = await fetch(`/api/projection/rates/${encodeURIComponent(key)}/rate-override`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ rate }),
      });
      if (!res.ok) throw new Error(await res.text());
      await load();
    } catch (e) {
      overrideError[key] = e.message;
      overrideError = overrideError;
    } finally {
      savingOverride[key] = false; savingOverride = savingOverride;
    }
  }

  async function saveRate(key) {
    saving[key] = true; saving = saving;
    saveError[key] = ''; saveError = saveError;
    try {
      const res = await fetch(`/api/projection/rates/${encodeURIComponent(key)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(editableRates[key]),
      });
      if (!res.ok) throw new Error(await res.text());
      await load();
    } catch (e) {
      saveError[key] = e.message;
      saveError = saveError;
    } finally {
      saving[key] = false; saving = saving;
    }
  }

  const _fmt = new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 });

  // $: ensures fmtMoney is reassigned whenever confidential changes,
  // which makes Svelte invalidate every template expression that calls it.
  $: fmtMoney = (v) => confidential ? HIDDEN : _fmt.format(v);

  function renderChart() {
    if (!data || !chartEl) return;
    chart?.destroy();

    const isConfidential = confidential;
    const years = data.years || [];

    // Use {x, y} points with x = year (0 = today) so Chart.js LinearScale
    // places points proportionally — +20 ans is visually 2× farther than +10 ans.
    const toPoints = (current, values) => [
      { x: 0, y: current },
      ...years.map(y => ({ x: y, y: values?.[String(y)] ?? current })),
    ];

    const datasets = [];
    let colorIdx = 0;

    for (const asset of data.assets || []) {
      const color = LINE_COLORS[colorIdx++ % LINE_COLORS.length];
      datasets.push({
        label: asset.name,
        data: toPoints(asset.current, asset.values),
        borderColor: color,
        backgroundColor: color + '18',
        borderWidth: 1.5,
        pointRadius: 2,
        tension: 0.3,
        fill: false,
      });
    }

    const totalByYear = y => (data.assets || []).reduce((s, a) => s + (a.values?.[String(y)] ?? a.current), 0);
    const totalCurrent = (data.assets || []).reduce((s, a) => s + a.current, 0);
    datasets.push({
      label: 'Total',
      data: [{ x: 0, y: totalCurrent }, ...years.map(y => ({ x: y, y: totalByYear(y) }))],
      borderColor: '#ffffff',
      backgroundColor: 'rgba(255,255,255,0.05)',
      borderWidth: 2.5,
      pointRadius: 3,
      tension: 0.3,
      fill: true,
      order: -1,
    });

    const fmtCompact = new Intl.NumberFormat('fr-FR', { notation: 'compact', compactDisplay: 'short' });
    const fmtFull   = new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 });

    chart = new Chart(chartEl, {
      type: 'line',
      data: { datasets },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: { mode: 'index', intersect: false },
        plugins: {
          legend: {
            position: 'bottom',
            labels: { color: '#94a3b8', font: { size: 11 }, boxWidth: 12, padding: 10 },
          },
          tooltip: {
            callbacks: {
              title: items => `+${items[0].parsed.x} ans`,
              label: ctx => {
                if (isConfidential) return ` ${ctx.dataset.label}: ${HIDDEN}`;
                return ` ${ctx.dataset.label}: ${fmtFull.format(ctx.parsed.y)}`;
              },
            },
          },
        },
        scales: {
          x: {
            type: 'linear',
            min: 0,
            max: Math.max(...years),
            ticks: {
              color: '#64748b',
              font: { size: 11 },
              stepSize: 1,
              callback: v => v === 0 ? "Auj." : `+${v}`,
            },
            grid: { color: '#1e293b' },
          },
          y: {
            ticks: {
              color: '#64748b', font: { size: 11 },
              callback: v => isConfidential ? HIDDEN : fmtCompact.format(v) + ' €',
            },
            grid: { color: '#1e293b' },
          },
        },
      },
    });
  }

  $: editableRateList = (data?.rates || []).filter(r => EDITABLE_KEYS.has(r.key));
  $: readonlyRateList  = (data?.rates || []).filter(r => !EDITABLE_KEYS.has(r.key));

  $: total0  = (data?.assets || []).reduce((s, a) => s + a.current, 0);
  $: total5  = (data?.assets || []).reduce((s, a) => s + (a.values?.['5']  ?? a.current), 0);
  $: total10 = (data?.assets || []).reduce((s, a) => s + (a.values?.['10'] ?? a.current), 0);
  $: total20 = (data?.assets || []).reduce((s, a) => s + (a.values?.['20'] ?? a.current), 0);

  // Re-render chart when confidential toggles (chart captures isConfidential at creation).
  $: if (confidential !== undefined && data) {
    renderChart();
  }
</script>

<div class="panel">
  {#if loading}
    <div class="center-msg">Chargement des projections...</div>
  {:else if error}
    <div class="error-msg">{error}</div>
  {:else}
    <!-- Chart -->
    <div class="chart-wrap">
      <canvas bind:this={chartEl}></canvas>
    </div>

    <!-- Summary table -->
    <div class="summary-table-wrap">
      <table class="summary-table">
        <thead>
          <tr>
            <th>Actif</th>
            <th>Aujourd'hui</th>
            <th>+5 ans</th>
            <th>+10 ans</th>
            <th>+20 ans</th>
            <th>Taux appliqué</th>
          </tr>
        </thead>
        <tbody>
          {#each data.assets || [] as asset}
            <tr class:flat={FLAT_TYPES.has(asset.type)}>
              <td>
                <span class="asset-name">{asset.name}</span>
                <span class="asset-type">{TYPE_LABELS[asset.type] || asset.type}</span>
              </td>
              <td class="num">{fmtMoney(asset.current)}</td>
              <td class="num">{fmtMoney(asset.values?.['5']  ?? asset.current)}</td>
              <td class="num">{fmtMoney(asset.values?.['10'] ?? asset.current)}</td>
              <td class="num projected">{fmtMoney(asset.values?.['20'] ?? asset.current)}</td>
              <td class="rate-cell">
                {#if FLAT_TYPES.has(asset.type)}
                  <span class="no-rate">—</span>
                {:else}
                  <span class="rate-badge">{asset.rate.toFixed(2)} %/an</span>
                {/if}
              </td>
            </tr>
          {/each}
          <tr class="total-row">
            <td>Total</td>
            <td class="num">{fmtMoney(total0)}</td>
            <td class="num">{fmtMoney(total5)}</td>
            <td class="num">{fmtMoney(total10)}</td>
            <td class="num projected">{fmtMoney(total20)}</td>
            <td></td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Editable rates -->
    <div class="rates-section">
      <h3>Taux de rendement</h3>
      <p class="rates-hint">Modifiez les taux utilisés pour les projections.</p>
      <div class="rates-grid">
        {#each editableRateList as rate (rate.key)}
          <div class="rate-card">
            <div class="rate-label">{rate.label}</div>
            <div class="rate-fields">
              <label class="field-group">
                <span>Taux (%/an)</span>
                <input
                  type="number"
                  step="0.1"
                  min="0"
                  max="100"
                  bind:value={editableRates[rate.key].rate}
                />
              </label>
            </div>
            {#if saveError[rate.key]}
              <div class="save-error">{saveError[rate.key]}</div>
            {/if}
            <button
              class="btn-save"
              disabled={saving[rate.key]}
              on:click={() => saveRate(rate.key)}
            >
              {saving[rate.key] ? 'Sauvegarde...' : 'Sauvegarder'}
            </button>
          </div>
        {/each}
      </div>
    </div>

    <!-- Read-only CAGR rates (tickers + synthetics) -->
    {#if readonlyRateList.length > 0}
      <div class="rates-section readonly-section">
        <h3>CAGR calculés</h3>
        <p class="rates-hint">Rendements annualisés calculés depuis l'historique Yahoo Finance. Mis à jour à chaque refresh.</p>
        <div class="readonly-grid">
          {#each readonlyRateList as rate (rate.key)}
            <div class="readonly-card">
              <div class="readonly-top">
                <span class="readonly-label">{rate.label}</span>
                <div class="rate-badges">
                  {#if rate.rate_override != null}
                    <span class="rate-badge override-active" title="Override actif">{rate.rate_override.toFixed(2)} %/an</span>
                    <span class="rate-badge computed" title="CAGR calculé">{rate.rate.toFixed(2)} %/an calculé</span>
                  {:else}
                    <span class="rate-badge">{rate.rate.toFixed(2)} %/an</span>
                  {/if}
                </div>
              </div>
              <div class="override-row">
                <input
                  class="override-input"
                  type="number"
                  step="0.1"
                  min="0"
                  max="100"
                  placeholder="Override % (vide = auto)"
                  bind:value={overrideRates[rate.key]}
                />
                <button
                  class="btn-override"
                  disabled={savingOverride[rate.key]}
                  on:click={() => saveRateOverride(rate.key)}
                >
                  {savingOverride[rate.key] ? '...' : 'OK'}
                </button>
              </div>
              {#if overrideError[rate.key]}
                <div class="save-error">{overrideError[rate.key]}</div>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .panel { padding: 1.5rem 0; }

  .center-msg, .error-msg { text-align: center; padding: 2rem; color: #94a3b8; }
  .error-msg { color: #f87171; }

  .chart-wrap { position: relative; height: 320px; margin-bottom: 2rem; background: #1e293b; border-radius: 10px; padding: 1rem; }

  /* Summary table */
  .summary-table-wrap { margin-bottom: 2rem; overflow-x: auto; }
  .summary-table { width: 100%; border-collapse: collapse; font-size: .87rem; background: #1e293b; border-radius: 10px; overflow: hidden; }
  .summary-table th { padding: .55rem 1.1rem; text-align: left; color: #64748b; font-size: .75rem; font-weight: 500; text-transform: uppercase; letter-spacing: .04em; border-bottom: 1px solid #334155; }
  .summary-table td { padding: .6rem 1.1rem; border-bottom: 1px solid #0f172a; color: #cbd5e1; }
  .summary-table tr:last-child td { border-bottom: none; }
  .summary-table tr:hover td { background: #263145; }
  .summary-table tr.flat td { opacity: .6; }

  .asset-name { display: block; font-weight: 500; color: #e2e8f0; }
  .asset-type  { font-size: .75rem; color: #64748b; }
  .num { text-align: right; font-variant-numeric: tabular-nums; }
  .projected { color: #34d399; font-weight: 600; }
  .rate-cell { text-align: center; }
  .rate-badge { background: #1e3a5f; color: #60a5fa; border-radius: 4px; padding: .15rem .45rem; font-size: .78rem; }
  .no-rate { color: #475569; }

  .total-row td { font-weight: 700; color: #f1f5f9; border-top: 2px solid #334155; }
  .total-row .projected { color: #34d399; }

  /* Editable rates */
  .rates-section { margin-bottom: 2rem; }
  .rates-section h3 { margin: 0 0 .35rem; font-size: .95rem; color: #e2e8f0; }
  .rates-hint { margin: 0 0 1rem; font-size: .82rem; color: #64748b; }

  .rates-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1rem; }
  .rate-card { background: #1e293b; border-radius: 8px; padding: 1rem; display: flex; flex-direction: column; gap: .65rem; }
  .rate-label { font-weight: 600; color: #e2e8f0; font-size: .9rem; }

  .rate-fields { display: flex; gap: .5rem; align-items: flex-end; flex-wrap: wrap; }
  .field-group { display: flex; flex-direction: column; gap: .25rem; font-size: .78rem; color: #64748b; }
  .field-group input { background: #0f172a; border: 1px solid #334155; color: #f1f5f9; border-radius: 5px; padding: .35rem .6rem; font-size: .88rem; width: 100%; box-sizing: border-box; }
  .field-group input:focus { outline: none; border-color: #3b82f6; }
  .source-field { flex: 1; min-width: 160px; }

  .source-link { color: #60a5fa; font-size: 1rem; text-decoration: none; padding: .35rem .3rem; align-self: flex-end; }
  .source-link:hover { color: #93c5fd; }

  .save-error { color: #f87171; font-size: .78rem; }

  .btn-save { align-self: flex-start; background: #1d4ed8; color: #fff; border: none; padding: .4rem .85rem; border-radius: 5px; font-size: .83rem; cursor: pointer; }
  .btn-save:hover:not(:disabled) { background: #2563eb; }
  .btn-save:disabled { opacity: .5; cursor: default; }

  /* Read-only CAGR section */
  .readonly-section { border-top: 1px solid #1e293b; padding-top: 1.5rem; }
  .readonly-grid { display: flex; flex-wrap: wrap; gap: .6rem; }
  .readonly-card { background: #0f172a; border: 1px solid #1e293b; border-radius: 6px; padding: .5rem .75rem; display: flex; flex-direction: column; gap: .4rem; min-width: 220px; }
  .readonly-top { display: flex; align-items: center; gap: .5rem; flex-wrap: wrap; }
  .readonly-label { font-size: .82rem; color: #94a3b8; flex: 1; }
  .rate-badges { display: flex; gap: .3rem; flex-wrap: wrap; }
  .rate-badge.override-active { background: #1a3a2a; color: #34d399; }
  .rate-badge.computed { background: #1e293b; color: #475569; font-size: .72rem; }
  .override-row { display: flex; gap: .35rem; align-items: center; }
  .override-input { flex: 1; background: #1e293b; border: 1px solid #334155; color: #f1f5f9; border-radius: 4px; padding: .25rem .5rem; font-size: .78rem; min-width: 0; }
  .override-input:focus { outline: none; border-color: #3b82f6; }
  .override-input::placeholder { color: #475569; }
  .btn-override { background: #1e3a5f; color: #60a5fa; border: none; border-radius: 4px; padding: .25rem .55rem; font-size: .78rem; cursor: pointer; white-space: nowrap; }
  .btn-override:hover:not(:disabled) { background: #1d4ed8; color: #fff; }
  .btn-override:disabled { opacity: .5; cursor: default; }
</style>
