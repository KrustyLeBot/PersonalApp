<script>
  import { onMount } from 'svelte';
  import { Chart, ArcElement, PieController, Tooltip, Legend } from 'chart.js';
  import AssetModal from './AssetModal.svelte';
  import HoldingsModal from './HoldingsModal.svelte';

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

  let typeChartEl, tickerChartEl;
  let typeChart, tickerChart;

  const TYPE_LABELS = {
    immobilier: 'Immobilier',
    fond_euro:  'Fonds Euro',
    livret:     'Livret',
    crypto:     'Crypto',
    bourse:     'Bourse',
  };
  const COLORS = ['#60a5fa','#34d399','#f59e0b','#a78bfa','#f87171','#38bdf8','#fb923c','#4ade80'];
  const HIDDEN = '••••';

  onMount(loadSummary);

  async function loadSummary() {
    loading = true; error = '';
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
    return {
      maintainAspectRatio: false,
      layout: { padding: 16 },
      plugins: {
        legend: {
          position: 'right',
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
    return a.value ?? 0;
  }

  function groupByType(assets) {
    const groups = {};
    for (const a of assets || []) {
      (groups[a.type] ||= []).push(a);
    }
    return groups;
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
    await loadSummary();
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
      </div>
    </div>

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

    <!-- Asset list -->
    <div class="asset-list">
      {#each Object.entries(groups) as [type, assets]}
        <div class="group">
          <div class="group-header">
            <span class="group-title">{TYPE_LABELS[type] || type}</span>
            <span class="group-total">{fmt(summary.by_type[type] || 0, confidential)}</span>
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
                      <button class="btn-icon danger" on:click={() => deleteAsset(a.id)} title="Supprimer">🗑</button>
                    </div>
                  </div>

                  {#if (summary.holdings?.[a.id] || []).length > 0}
                    <table class="holdings-table">
                      <thead>
                        <tr><th>Ticker</th><th>Parts</th><th>Prix unitaire</th><th>Valeur</th></tr>
                      </thead>
                      <tbody>
                        {#each summary.holdings[a.id] as h}
                          {@const price = summary.ticker_prices?.[h.ticker] ?? 0}
                          {@const val = price * h.shares}
                          <tr>
                            <td><code>{h.ticker}</code></td>
                            <td>{fmtShares(h.shares, confidential)}</td>
                            <td>{price > 0 ? fmt(price, confidential) : '—'}</td>
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
                  <tr>
                    <td>{a.name}</td>
                    <td class="value-cell">{fmt(a.value || 0, confidential)}</td>
                    <td class="actions-cell">
                      {#if !confidential}
                        <button class="btn-icon" on:click={() => openEditAsset(a)} title="Modifier">✏️</button>
                      {/if}
                      <button class="btn-icon danger" on:click={() => deleteAsset(a.id)} title="Supprimer">🗑</button>
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
</div>

{#if showAssetModal}
  <AssetModal asset={editingAsset} on:save={onSaveAsset} on:close={() => showAssetModal = false} />
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

  .charts-row { display: flex; gap: 1.5rem; margin-bottom: 2rem; }
  .chart-card { background: #1e293b; border-radius: 10px; padding: 1.5rem; flex: 1; min-width: 0; }
  .chart-card h3 { margin: 0 0 .5rem; font-size: .9rem; color: #94a3b8; font-weight: 500; text-align: center; }
  .chart-wrap { position: relative; height: 260px; }
  .no-data { color: #475569; font-size: .85rem; }

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
  .holdings-table tr:hover td { background: #111f35; }

  .value-cell { font-weight: 600; color: #f1f5f9; }
  .actions-cell { text-align: right; white-space: nowrap; display: flex; align-items: center; gap: .25rem; }
  .btn-icon { background: none; border: none; cursor: pointer; font-size: 1rem; padding: .2rem .35rem; border-radius: 4px; opacity: .6; }
  .btn-icon:hover { opacity: 1; background: #334155; }
  .btn-icon.danger:hover { background: #450a0a; }

  .no-holdings { padding: .6rem 1.5rem; color: #475569; font-size: .85rem; margin: 0; }
  .link-btn { background: none; border: none; color: #60a5fa; cursor: pointer; font-size: .85rem; padding: 0; text-decoration: underline; }

  code { font-family: monospace; background: #1e293b; padding: .1rem .4rem; border-radius: 3px; font-size: .85rem; }
  .empty-state { text-align: center; padding: 3rem; color: #64748b; }
  .empty-state p { margin-bottom: 1rem; }
</style>
