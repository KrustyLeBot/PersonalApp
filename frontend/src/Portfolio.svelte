<script>
  import { onMount, onDestroy } from 'svelte';
  import { Chart, ArcElement, PieController, Tooltip, Legend } from 'chart.js';
  import AssetModal from './AssetModal.svelte';
  import HoldingsModal from './HoldingsModal.svelte';
  import ProjectionPanel from './ProjectionPanel.svelte';
  import { autoRefreshIfStale } from './lib/dailyRefresh.js';

  Chart.register(ArcElement, PieController, Tooltip, Legend);

  let summary = null;
  let loading = true;
  let error = '';
  let refreshing = false;

  // Confidential mode is persisted per-browser via localStorage, so a work
  // machine can stay hidden while a home machine shows amounts.
  let confidential = localStorage.getItem('portfolio.confidential') === 'true';

  let showAssetModal = false;
  let editingAsset = null;

  let showHoldingsModal = false;
  let selectedAccount = null;

  let showProjection = false;

  let typeChartEl, tickerChartEl;
  let typeChart, tickerChart;

  const TYPE_LABELS = {
    immobilier: 'Immobilier',
    fond_euro:  'Fonds Euro',
    livret:     'Livret',
    crypto:     'Crypto',
    bourse:     'Bourse',
    structure:  'Produit structuré',
    dette:      'Dette',
  };
  const COLORS = ['#60a5fa','#34d399','#f59e0b','#a78bfa','#f87171','#38bdf8','#fb923c','#4ade80'];
  const HIDDEN = '••••';

  onMount(async () => {
    await loadSummary();
    // Page is now displayed with cached data; refresh in the background if stale.
    autoRefreshIfStale(summary?.last_refresh, forceRefresh);
    window.addEventListener('resize', onResize);
  });

  onDestroy(() => window.removeEventListener('resize', onResize));

  // Legend position (right vs bottom) depends on viewport width, so charts
  // need a redraw when crossing the mobile breakpoint (e.g. device rotation).
  let wasNarrow = window.innerWidth <= 480;
  function onResize() {
    const isNarrow = window.innerWidth <= 480;
    if (isNarrow !== wasNarrow) {
      wasNarrow = isNarrow;
      renderCharts();
    }
  }

  async function loadSummary() {
    // Only show the full-page loading state on the first load; a refresh-driven
    // reload keeps the current view up (the button spinner signals the refresh).
    if (!summary) loading = true;
    error = '';
    try {
      const res = await fetch('/api/portfolio/summary');
      if (!res.ok) throw new Error(await res.text());
      summary = await res.json();
      setTimeout(renderCharts, 50);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function forceRefresh() {
    refreshing = true;
    try {
      await fetch('/api/portfolio/refresh', { method: 'POST' });
      await loadSummary();
    } finally {
      refreshing = false;
    }
  }

  function toggleConfidential() {
    confidential = !confidential;
    localStorage.setItem('portfolio.confidential', String(confidential));
    setTimeout(renderCharts, 0);
  }

  // fmt takes `hidden` explicitly so Svelte tracks `confidential` as a markup
  // dependency and re-renders amounts the instant the mode is toggled.
  function fmt(v, hidden = confidential) {
    if (hidden) return HIDDEN;
    return new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 }).format(v);
  }

  function fmtShares(v, hidden = confidential) {
    return hidden ? '••' : v;
  }

  // Intraday variation is a percentage, not a monetary amount, so it stays
  // visible in confidential mode.
  function fmtChange(pct) {
    if (pct == null) return '—';
    const sign = pct > 0 ? '+' : '';
    return `${sign}${pct.toFixed(2)}%`;
  }

  function renderCharts() {
    if (!summary) return;

    const typeEntries = Object.entries(summary.by_type || {}).filter(([, v]) => v > 0);
    if (typeChartEl && typeEntries.length > 0) {
      typeChart?.destroy();
      typeChart = new Chart(typeChartEl, {
        type: 'pie',
        data: {
          labels: typeEntries.map(([k]) => TYPE_LABELS[k] || k),
          datasets: [{ data: typeEntries.map(([, v]) => v), backgroundColor: COLORS, hoverOffset: 28 }],
        },
        options: chartOptions(summary.total),
      });
    }

    const categoryEntries = Object.entries(summary.by_category || {}).filter(([, v]) => v > 0);
    if (tickerChartEl && categoryEntries.length > 0) {
      tickerChart?.destroy();
      const categoryTotal = categoryEntries.reduce((s, [, v]) => s + v, 0);
      tickerChart = new Chart(tickerChartEl, {
        type: 'pie',
        data: {
          labels: categoryEntries.map(([k]) => k),
          datasets: [{ data: categoryEntries.map(([, v]) => v), backgroundColor: COLORS.slice(2), hoverOffset: 28 }],
        },
        options: chartOptions(categoryTotal),
      });
    }
  }

  function chartOptions(denominator) {
    const isNarrow = window.innerWidth <= 480;
    return {
      maintainAspectRatio: false,
      layout: { padding: 16 },
      plugins: {
        legend: {
          position: isNarrow ? 'bottom' : 'right',
          labels: { color: '#cbd5e1', font: { size: 12 }, boxWidth: 14, padding: 12 },
          onHover(_, legendItem, legend) {
            const chart = legend.chart;
            chart.tooltip.setActiveElements(
              [{ datasetIndex: 0, index: legendItem.index }],
              { x: 0, y: 0 },
            );
            chart.update();
          },
          onLeave(_, _item, legend) {
            legend.chart.tooltip.setActiveElements([], {});
            legend.chart.update();
          },
        },
        tooltip: {
          callbacks: {
            label: (ctx) => {
              const pct = denominator > 0 ? ((ctx.parsed / denominator) * 100).toFixed(1) : 0;
              if (confidential) return ` ${ctx.label}: ${pct}%`;
              return ` ${ctx.label}: ${new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 }).format(ctx.parsed)} (${pct}%)`;
            },
          },
        },
      },
    };
  }

  const TICKER_BASED = new Set(['bourse', 'crypto']);

  function assetDisplayValue(a) {
    if (TICKER_BASED.has(a.type)) return summary?.account_values?.[a.id] ?? 0;
    if (a.type === 'dette') return summary?.account_values?.[a.id] ?? 0;
    return a.value ?? 0;
  }

  const fmtEur = new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 });

  function detteOf(a) { return summary?.dettes?.[a.id] ?? null; }

  function groupByType(assets) {
    const groups = {};
    for (const a of assets || []) {
      (groups[a.type] ||= []).push(a);
    }
    return groups;
  }

  function toggleProjection() {
    showProjection = !showProjection;
    // Coming back to the charts view remounts the canvases; redraw once the
    // {:else} block is in the DOM (Chart.js needs a live canvas element).
    if (!showProjection) setTimeout(renderCharts, 50);
  }

  function openCreateAsset() { editingAsset = null; showAssetModal = true; }
  function openEditAsset(a)  { editingAsset = { ...a }; showAssetModal = true; }
  function openHoldings(a)   { selectedAccount = a; showHoldingsModal = true; }

  async function onSaveAsset(event) {
    const a = event.detail;
    const method = a.id ? 'PUT' : 'POST';
    const url    = a.id ? `/api/portfolio/assets/${a.id}` : '/api/portfolio/assets';
    await fetch(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(a) });
    showAssetModal = false;
    await loadSummary();
  }

  async function deleteAsset(id) {
    if (!confirm('Supprimer cet actif et toutes ses positions ?')) return;
    await fetch(`/api/portfolio/assets/${id}`, { method: 'DELETE' });
    showAssetModal = false;
    await loadSummary();
  }

  // --- Projection rate editing (shared PUT to the rate-override endpoint) ---
  let savingRate = {}; // key → bool

  async function saveRateOverride(key, raw) {
    savingRate[key] = true; savingRate = savingRate;
    try {
      const rate = raw === '' || raw == null ? null : parseFloat(raw);
      await fetch(`/api/portfolio/projection/rates/${encodeURIComponent(key)}/rate-override`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ rate }),
      });
      await loadSummary();
    } finally {
      savingRate[key] = false; savingRate = savingRate;
    }
  }

  // Per-category CAGR (bourse) and per-asset rates share the same shape:
  // { key, rate, override }. The input map is keyed by projection key.
  let categoryOverride = {}; // key → input string ('' = auto)
  $: if (summary?.category_rates) {
    categoryOverride = Object.fromEntries(
      summary.category_rates.map((c) => [c.key, c.override != null ? String(c.override) : ''])
    );
  }

  const assetNameById = (id) => (summary?.assets || []).find((a) => a.id === id)?.name ?? '';
  const assetTypeById = (id) => (summary?.assets || []).find((a) => a.id === id)?.type ?? '';

  // Effective per-asset rate to preload the modal (null when unset).
  function assetRateForModal(id) {
    const r = (summary?.account_rates || []).find((x) => x.asset_id === id);
    return r?.is_set ? r.rate : null;
  }

  let assetOverride = {}; // key → input string ('' = unset)
  $: if (summary?.account_rates) {
    assetOverride = Object.fromEntries(
      summary.account_rates.map((a) => [a.key, a.is_set ? String(a.rate) : ''])
    );
  }

  $: groups = groupByType(summary?.assets);
  $: hasStocks = Object.keys(summary?.by_category || {}).length > 0;
</script>

<div class="portfolio">
  {#if loading}
    <div class="center-msg">Chargement du portfolio...</div>
  {:else if error}
    <div class="error-msg">{error}</div>
  {:else}
    <!-- Header -->
    <div class="top-bar">
      <div class="total-block">
        <div class="total-label">Patrimoine total</div>
        <div class="total-value {confidential ? 'blurred' : ''}">{fmt(summary.total || 0, confidential)}</div>
        {#if summary.last_refresh}
          <div class="refresh-info">Cotations : {new Date(summary.last_refresh).toLocaleString('fr-FR')}</div>
        {/if}
      </div>
      <div class="top-actions">
        <button
          class="btn-confidential {confidential ? 'active' : ''}"
          on:click={toggleConfidential}
          title={confidential ? 'Afficher les montants' : 'Masquer les montants'}
        >
          {confidential ? '🔒 Confidentiel' : '👁 Visible'}
        </button>
        <button class="btn-secondary" on:click={forceRefresh} disabled={refreshing}>
          {refreshing ? 'Rafraîchissement...' : '↻ Rafraîchir cotations'}
        </button>
        <button class="btn-primary" on:click={openCreateAsset}>+ Ajouter un actif</button>
        <button class="btn-projection {showProjection ? 'active' : ''}" on:click={toggleProjection}>
          📈 Projection
        </button>
      </div>
    </div>

    {#if showProjection}
      <div class="projection-section">
        <ProjectionPanel {confidential} />
      </div>
    {:else}
    <!-- Charts -->
    <div class="charts-row">
      <div class="chart-card">
        <h3>Répartition par type</h3>
        {#if Object.keys(summary.by_type || {}).length > 0}
          <div class="chart-wrap">
            <canvas bind:this={typeChartEl}></canvas>
          </div>
        {:else}
          <p class="no-data">Aucune donnée</p>
        {/if}
      </div>
      {#if hasStocks}
        <div class="chart-card">
          <h3>Répartition par catégorie</h3>
          <div class="chart-wrap">
            <canvas bind:this={tickerChartEl}></canvas>
          </div>
        </div>
      {/if}
    </div>

    <!-- Projection rates: per-asset (left) + per-category CAGR (right).
         Both cards share the same height (stretch) so they line up. -->
    {#if (summary.account_rates || []).length > 0 || (summary.category_rates || []).length > 0}
      <div class="rates-row">
        <div class="rate-block">
          <h3>Taux par actif</h3>
          {#if (summary.account_rates || []).length > 0}
            {#each summary.account_rates as a (a.key)}
              <div class="rate-line">
                <span class="rate-name">{assetNameById(a.asset_id)}<span class="rate-sub">{TYPE_LABELS[assetTypeById(a.asset_id)] || assetTypeById(a.asset_id)}</span></span>
                <span class="rate-badge" class:override={a.is_set}>
                  {a.is_set ? `${a.rate.toFixed(2)} %/an` : 'non défini'}
                </span>
                <input
                  class="rate-input"
                  type="number" step="0.1" min="0" max="100" placeholder="0"
                  value={assetOverride[a.key]}
                  on:input={(e) => { assetOverride[a.key] = e.currentTarget.value; }}
                  data-form-type="other" data-lpignore="true" autocomplete="off"
                />
                <button class="rate-btn" disabled={savingRate[a.key]}
                  on:click={() => saveRateOverride(a.key, assetOverride[a.key])}
                >{savingRate[a.key] ? '…' : 'OK'}</button>
              </div>
            {/each}
          {:else}
            <p class="rate-empty">Aucun actif à taux fixe.</p>
          {/if}
        </div>

        <div class="rate-block">
          <h3>CAGR par catégorie</h3>
          {#if (summary.category_rates || []).length > 0}
            {#each summary.category_rates as c (c.key)}
              <div class="rate-line">
                <span class="rate-name">{c.category}</span>
                <span class="rate-badge" class:override={c.override != null}>
                  {(c.override ?? c.rate).toFixed(2)} %/an
                  {#if c.override != null}<span class="rate-auto">(auto {c.rate.toFixed(2)})</span>{/if}
                </span>
                <input
                  class="rate-input"
                  type="number" step="0.1" min="0" max="100" placeholder="auto"
                  value={categoryOverride[c.key]}
                  on:input={(e) => { categoryOverride[c.key] = e.currentTarget.value; }}
                  data-form-type="other" data-lpignore="true" autocomplete="off"
                />
                <button class="rate-btn" disabled={savingRate[c.key]}
                  on:click={() => saveRateOverride(c.key, categoryOverride[c.key])}
                >{savingRate[c.key] ? '…' : 'OK'}</button>
              </div>
            {/each}
          {:else}
            <p class="rate-empty">Aucune catégorie bourse.</p>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Asset list -->
    <div class="asset-list">
      {#each Object.entries(groups) as [type, assets]}
        <div class="group">
          <div class="group-header">
            <span class="group-title">{TYPE_LABELS[type] || type}</span>
            <span class="group-total {type === 'dette' ? 'negative' : ''}">{fmt(summary.by_type[type] || 0, confidential)}</span>
          </div>

          {#if TICKER_BASED.has(type)}
            <div class="bourse-accounts">
              {#each assets as a}
                <div class="bourse-account">
                  <div class="bourse-account-header">
                    <div class="bourse-account-info">
                      <span class="account-name">{a.name}</span>
                      <span class="account-value">{fmt(assetDisplayValue(a), confidential)}</span>
                    </div>
                    <div class="actions-cell">
                      {#if !confidential}
                        <button class="btn-holdings" on:click={() => openHoldings(a)}>
                          Gérer les positions ({(summary.holdings?.[a.id] || []).length})
                        </button>
                        <button class="btn-icon" on:click={() => openEditAsset(a)} title="Renommer">✏️</button>
                      {/if}
                    </div>
                  </div>

                  {#if (summary.holdings?.[a.id] || []).length > 0}
                    <table class="holdings-table">
                      <thead>
                        <tr><th>Ticker</th><th>Parts</th><th>Prix unitaire</th><th>Var. jour</th><th>Valeur</th></tr>
                      </thead>
                      <tbody>
                        {#each summary.holdings[a.id] as h}
                          {@const price = summary.ticker_prices?.[h.ticker] ?? 0}
                          {@const val = price * h.shares}
                          {@const change = summary.ticker_day_changes?.[h.ticker]}
                          <tr>
                            <td><code>{h.ticker}</code></td>
                            <td>{fmtShares(h.shares, confidential)}</td>
                            <td>{price > 0 ? fmt(price, confidential) : '—'}</td>
                            <td class="change-cell {change > 0 ? 'up' : change < 0 ? 'down' : ''}">{fmtChange(change)}</td>
                            <td class="value-cell">{fmt(val, confidential)}</td>
                          </tr>
                        {/each}
                      </tbody>
                    </table>
                  {:else}
                    <p class="no-holdings">Aucune position — <button class="link-btn" on:click={() => openHoldings(a)}>ajouter un ticker</button></p>
                  {/if}
                </div>
              {/each}
            </div>

          {:else}
            <table class="asset-table">
              <thead>
                <tr><th>Nom</th><th>Valeur</th><th></th></tr>
              </thead>
              <tbody>
                {#each assets as a}
                  {@const d = detteOf(a)}
                  <tr class={a.type === 'dette' ? 'dette-row' : ''}>
                    <td>
                      {a.name}
                      {#if a.type === 'dette' && d}
                        <span class="dette-info">{fmtEur.format(d.monthly_payment)}/mois · {d.duration_months} mois · {d.taeg}% TAEG</span>
                      {/if}
                    </td>
                    <td class="value-cell {a.type === 'dette' ? 'negative' : ''}">
                      {fmt(assetDisplayValue(a), confidential)}
                    </td>
                    <td class="actions-cell">
                      {#if !confidential}
                        <button class="btn-icon" on:click={() => openEditAsset(a)} title="Modifier">✏️</button>
                      {/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>
      {/each}

      {#if (summary.assets || []).length === 0}
        <div class="empty-state">
          <p>Aucun actif enregistré. Commencez par en ajouter un !</p>
          <button class="btn-primary" on:click={openCreateAsset}>+ Ajouter un actif</button>
        </div>
      {/if}
    </div>
    {/if}
  {/if}
</div>

{#if showAssetModal}
  <AssetModal
    asset={editingAsset}
    dette={editingAsset ? (summary?.dettes?.[editingAsset.id] ?? null) : null}
    rate={editingAsset ? assetRateForModal(editingAsset.id) : null}
    on:save={onSaveAsset}
    on:delete={() => deleteAsset(editingAsset.id)}
    on:close={() => showAssetModal = false}
  />
{/if}

{#if showHoldingsModal && selectedAccount}
  <HoldingsModal
    account={selectedAccount}
    tickerPrices={summary?.ticker_prices || {}}
    tickerCategories={summary?.ticker_categories || {}}
    on:close={() => { showHoldingsModal = false; loadSummary(); }}
  />
{/if}

<style>
  .portfolio { max-width: 1100px; margin: 0 auto; }

  @media (max-width: 480px) {
    .top-actions {
      width: 100%; flex-wrap: nowrap; overflow-x: auto;
      -webkit-overflow-scrolling: touch; padding-bottom: .25rem;
    }
    .top-actions button { flex: 0 0 auto; white-space: nowrap; }
  }

  .center-msg, .error-msg { text-align: center; padding: 3rem; color: #94a3b8; }
  .error-msg { color: #f87171; }

  .top-bar {
    display: flex; justify-content: space-between; align-items: flex-end;
    margin-bottom: 2rem; flex-wrap: wrap; gap: 1rem;
  }
  .total-label { font-size: .85rem; color: #94a3b8; margin-bottom: .25rem; }
  .total-value { font-size: 2.5rem; font-weight: 700; color: #f1f5f9; transition: filter .2s; }
  .total-value.blurred { filter: blur(8px); user-select: none; }
  .refresh-info { font-size: .75rem; color: #64748b; margin-top: .25rem; }
  .top-actions { display: flex; gap: .75rem; align-items: center; }

  .btn-primary    { background: #3b82f6; color: #fff; border: none; padding: .6rem 1.1rem; border-radius: 6px; font-size: .9rem; cursor: pointer; }
  .btn-primary:hover { background: #2563eb; }
  .btn-secondary  { background: transparent; color: #94a3b8; border: 1px solid #334155; padding: .6rem 1.1rem; border-radius: 6px; font-size: .9rem; cursor: pointer; }
  .btn-secondary:hover:not(:disabled) { border-color: #64748b; color: #f1f5f9; }
  .btn-secondary:disabled { opacity: .5; cursor: default; }

  .btn-confidential {
    background: transparent; border: 1px solid #334155; color: #94a3b8;
    padding: .6rem 1.1rem; border-radius: 6px; font-size: .9rem; cursor: pointer;
    transition: border-color .15s, color .15s, background .15s;
  }
  .btn-confidential:hover { border-color: #64748b; color: #f1f5f9; }
  .btn-confidential.active { border-color: #f59e0b; color: #f59e0b; background: rgba(245,158,11,.08); }

  .btn-projection {
    background: transparent; border: 1px solid #334155; color: #94a3b8;
    padding: .6rem 1.1rem; border-radius: 6px; font-size: .9rem; cursor: pointer;
    transition: border-color .15s, color .15s, background .15s;
  }
  .btn-projection:hover { border-color: #64748b; color: #f1f5f9; }
  .btn-projection.active { border-color: #34d399; color: #34d399; background: rgba(52,211,153,.08); }

  .projection-section { margin-bottom: 2rem; border-top: 1px solid #334155; padding-top: 1.5rem; }

  .charts-row { display: flex; flex-wrap: wrap; gap: 1.5rem; margin-bottom: 2rem; }
  .chart-card { background: #1e293b; border-radius: 10px; padding: 1.5rem; flex: 1; min-width: 260px; }
  .chart-card h3 { margin: 0 0 .5rem; font-size: .9rem; color: #94a3b8; font-weight: 500; text-align: center; }
  .chart-wrap { position: relative; height: 260px; }
  .no-data { color: #475569; font-size: .85rem; }

  /* Projection rate blocks (per-asset | per-category), equal height */
  .rates-row { display: flex; flex-wrap: wrap; gap: 1.5rem; margin-bottom: 2rem; align-items: stretch; }
  .rate-block { flex: 1; min-width: 260px; background: #1e293b; border-radius: 10px; padding: 1.5rem; display: flex; flex-direction: column; gap: .5rem; }
  .rate-block h3 { margin: 0 0 .5rem; font-size: .9rem; color: #94a3b8; font-weight: 500; text-align: center; }
  .rate-empty { color: #475569; font-size: .85rem; margin: 0; }
  .rate-line { display: flex; flex-wrap: wrap; align-items: center; gap: .5rem; }
  .rate-name { flex: 1; font-size: .85rem; color: #cbd5e1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .rate-sub { display: block; font-size: .72rem; color: #64748b; }
  .rate-badge { background: #1e3a5f; color: #60a5fa; border-radius: 4px; padding: .15rem .45rem; font-size: .78rem; white-space: nowrap; }
  .rate-badge.override { background: #1a3a2a; color: #34d399; }
  .rate-auto { color: #475569; font-size: .7rem; margin-left: .25rem; }
  .rate-input { width: 4.5rem; background: #0f172a; border: 1px solid #334155; color: #f1f5f9; border-radius: 4px; padding: .25rem .4rem; font-size: .8rem; }
  .rate-input:focus { outline: none; border-color: #3b82f6; }
  .rate-btn { background: #1e3a5f; color: #60a5fa; border: none; border-radius: 4px; padding: .25rem .55rem; font-size: .78rem; cursor: pointer; }
  .rate-btn:hover:not(:disabled) { background: #1d4ed8; color: #fff; }
  .rate-btn:disabled { opacity: .5; cursor: default; }

  @media (max-width: 480px) {
    .rate-block, .chart-card { flex: 1 1 100%; min-width: 0; padding: 1rem; }
    .rate-line { gap: .4rem; }
    .rate-name { flex: 1 1 100%; white-space: normal; }
    .rate-badge { white-space: normal; }
    .rate-auto { display: block; margin-left: 0; }
    .rate-input { width: 3.5rem; }
    .chart-wrap { height: 320px; }
  }

  .asset-list { display: flex; flex-direction: column; gap: 1.25rem; }
  .group { background: #1e293b; border-radius: 10px; overflow: hidden; }
  .group-header { display: flex; justify-content: space-between; align-items: center; padding: .8rem 1.25rem; border-bottom: 1px solid #334155; }
  .group-title { font-weight: 600; color: #e2e8f0; }
  .group-total { font-weight: 600; color: #60a5fa; }

  .asset-table { width: 100%; border-collapse: collapse; font-size: .88rem; }
  .asset-table th { padding: .5rem 1.25rem; text-align: left; color: #64748b; font-size: .78rem; font-weight: 500; text-transform: uppercase; letter-spacing: .04em; border-bottom: 1px solid #334155; }
  .asset-table td { padding: .65rem 1.25rem; border-bottom: 1px solid #1e293b; color: #cbd5e1; }
  .asset-table tr:last-child td { border-bottom: none; }
  .asset-table tr:hover td { background: #263145; }

  .bourse-accounts { display: flex; flex-direction: column; }
  .bourse-account { border-bottom: 1px solid #334155; }
  .bourse-account:last-child { border-bottom: none; }
  .bourse-account-header { display: flex; justify-content: space-between; align-items: center; padding: .85rem 1.25rem; gap: 1rem; }
  .bourse-account-info { display: flex; align-items: center; gap: 1rem; }
  .account-name  { font-weight: 600; color: #e2e8f0; font-size: .95rem; }
  .account-value { font-weight: 600; color: #60a5fa; }

  .btn-holdings { background: #1e3a5f; color: #60a5fa; border: 1px solid #1d4ed8; padding: .3rem .75rem; border-radius: 5px; font-size: .82rem; cursor: pointer; white-space: nowrap; }
  .btn-holdings:hover { background: #1d3a5a; }

  .holdings-table { width: 100%; border-collapse: collapse; font-size: .83rem; background: #0f172a; }
  .holdings-table th { padding: .4rem 1.5rem; text-align: left; color: #475569; font-size: .75rem; font-weight: 500; text-transform: uppercase; letter-spacing: .04em; }
  .holdings-table td { padding: .5rem 1.5rem; color: #94a3b8; border-top: 1px solid #1e293b; }

  .dette-row { background: rgba(239,68,68,.06); }
  .dette-row td { background: transparent; }
  .dette-row:hover { background: rgba(239,68,68,.12); }
  .dette-row:hover td { background: transparent !important; }
  .dette-info { display: block; font-size: .75rem; color: #64748b; margin-top: .15rem; font-weight: 400; }
  .value-cell.negative, .negative { color: #f87171; }
  .holdings-table tr:hover td { background: #111f35; }

  .value-cell { font-weight: 600; color: #f1f5f9; }
  .change-cell { font-weight: 600; color: #64748b; font-variant-numeric: tabular-nums; }
  .change-cell.up   { color: #34d399; }
  .change-cell.down { color: #f87171; }
  .actions-cell { text-align: right; white-space: nowrap; display: flex; align-items: center; gap: .25rem; }
  .btn-icon { background: none; border: none; cursor: pointer; font-size: 1rem; padding: .2rem .35rem; border-radius: 4px; opacity: .6; }
  .btn-icon:hover { opacity: 1; background: #334155; }

  .no-holdings { padding: .6rem 1.5rem; color: #475569; font-size: .85rem; margin: 0; }
  .link-btn { background: none; border: none; color: #60a5fa; cursor: pointer; font-size: .85rem; padding: 0; text-decoration: underline; }

  code { font-family: monospace; background: #1e293b; padding: .1rem .4rem; border-radius: 3px; font-size: .85rem; }
  .empty-state { text-align: center; padding: 3rem; color: #64748b; }
  .empty-state p { margin-bottom: 1rem; }
</style>
