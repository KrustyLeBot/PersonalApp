<script>
  import { onMount, createEventDispatcher } from 'svelte';

  export let account;          // { id, name }
  export let tickerPrices;     // map ticker → price
  export let tickerCategories; // map ticker → category

  const dispatch = createEventDispatcher();

  let holdings = [];
  let loading = true;
  let saving = false;

  // Local copy of categories so edits are reflected immediately in the UI
  let localCategories = { ...tickerCategories };

  // Distinct category values available for autocomplete
  $: existingCategories = [...new Set(Object.values(localCategories).filter(Boolean))].sort();

  // Inline add/edit form
  let form = null; // null = hidden

  onMount(loadHoldings);

  async function loadHoldings() {
    loading = true;
    const res = await fetch(`/api/portfolio/assets/${account.id}/holdings`);
    holdings = res.ok ? (await res.json()) ?? [] : [];
    loading = false;
  }

  function openAdd()   { form = { ticker: '', shares: '' }; }
  function openEdit(h) { form = { id: h.id, ticker: h.ticker, shares: h.shares }; }
  function cancelForm(){ form = null; }

  async function saveForm() {
    if (!form.ticker.trim() || !form.shares) return;
    saving = true;
    const payload = { ticker: form.ticker.toUpperCase().trim(), shares: Number(form.shares) };
    if (form.id) {
      await fetch(`/api/portfolio/holdings/${form.id}`, {
        method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload),
      });
    } else {
      await fetch(`/api/portfolio/assets/${account.id}/holdings`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload),
      });
    }
    form = null;
    saving = false;
    await loadHoldings();
  }

  async function deleteHolding(id) {
    if (!confirm('Supprimer cette position ?')) return;
    await fetch(`/api/portfolio/holdings/${id}`, { method: 'DELETE' });
    await loadHoldings();
  }

  // Category editing — saved on blur or Enter
  async function saveCategory(ticker, value) {
    const trimmed = value.trim();
    if (trimmed) {
      await fetch(`/api/portfolio/tickers/${encodeURIComponent(ticker)}/category`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ category: trimmed }),
      });
      localCategories = { ...localCategories, [ticker]: trimmed };
    } else {
      await fetch(`/api/portfolio/tickers/${encodeURIComponent(ticker)}/category`, { method: 'DELETE' });
      const { [ticker]: _, ...rest } = localCategories;
      localCategories = rest;
    }
  }

  function onCategoryKeydown(e, ticker) {
    if (e.key === 'Enter') e.target.blur();
  }

  function fmt(v) {
    return new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR', maximumFractionDigits: 2 }).format(v);
  }

  $: accountTotal = holdings.reduce((sum, h) => sum + (tickerPrices[h.ticker] ?? 0) * h.shares, 0);
</script>

<div class="overlay" role="dialog" aria-modal="true">
  <div class="modal">
    <div class="modal-header">
      <div>
        <h2>{account.name}</h2>
        <div class="account-total">{fmt(accountTotal)}</div>
      </div>
      <button class="btn-close" on:click={() => dispatch('close')}>✕</button>
    </div>

    {#if loading}
      <p class="msg">Chargement...</p>
    {:else}
      {#if holdings.length > 0}
        <table class="holdings-table">
          <thead>
            <tr>
              <th>Ticker</th>
              <th>Catégorie <span class="th-hint">(groupement chart)</span></th>
              <th>Parts</th>
              <th>Prix</th>
              <th>Valeur</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {#each holdings as h}
              {@const price = tickerPrices[h.ticker] ?? 0}
              <tr>
                <td><code>{h.ticker}</code></td>
                <td>
                  <input
                    class="cat-input"
                    type="text"
                    list="categories-list"
                    value={localCategories[h.ticker] ?? ''}
                    placeholder="ex: MSCI World"
                    on:blur={(e) => saveCategory(h.ticker, e.target.value)}
                    on:keydown={(e) => onCategoryKeydown(e, h.ticker)}
                  />
                </td>
                <td>{h.shares}</td>
                <td>{price > 0 ? fmt(price) : '—'}</td>
                <td class="value-cell">{fmt(price * h.shares)}</td>
                <td class="actions-cell">
                  <button class="btn-icon" on:click={() => openEdit(h)} title="Modifier">✏️</button>
                  <button class="btn-icon danger" on:click={() => deleteHolding(h.id)} title="Supprimer">🗑</button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {:else}
        <p class="msg">Aucune position pour ce compte.</p>
      {/if}

      {#if form}
        <div class="form-row">
          <input
            type="text"
            bind:value={form.ticker}
            placeholder="Ticker (ex: CW8.PA)"
            class="input-ticker"
          />
          <input
            type="number"
            bind:value={form.shares}
            placeholder="Nombre de parts"
            min="0"
            step="0.000001"
            class="input-shares"
          />
          <button class="btn-save" on:click={saveForm} disabled={saving}>
            {saving ? '...' : form.id ? 'Mettre à jour' : 'Ajouter'}
          </button>
          <button class="btn-cancel" on:click={cancelForm}>Annuler</button>
        </div>
      {:else}
        <button class="btn-add" on:click={openAdd}>+ Ajouter une position</button>
      {/if}
    {/if}
  </div>
</div>

<!-- Shared datalist for category autocomplete across all rows -->
<datalist id="categories-list">
  {#each existingCategories as cat}
    <option value={cat} />
  {/each}
</datalist>

<style>
  .overlay { position: fixed; inset: 0; background: rgba(0,0,0,.65); display: flex; align-items: center; justify-content: center; z-index: 100; }
  .modal { background: #1e293b; border-radius: 12px; padding: 1.75rem; width: 100%; max-width: 680px; box-shadow: 0 20px 60px rgba(0,0,0,.5); }

  .modal-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1.25rem; }
  h2 { margin: 0 0 .2rem; font-size: 1.1rem; color: #f1f5f9; }
  .account-total { font-size: 1.4rem; font-weight: 700; color: #60a5fa; }
  .btn-close { background: none; border: none; color: #94a3b8; font-size: 1.1rem; cursor: pointer; padding: .25rem; }
  .btn-close:hover { color: #f1f5f9; }

  .msg { color: #64748b; font-size: .88rem; padding: .5rem 0; }

  .holdings-table { width: 100%; border-collapse: collapse; font-size: .88rem; margin-bottom: 1rem; }
  .holdings-table th { padding: .4rem .75rem; text-align: left; color: #64748b; font-size: .76rem; font-weight: 500; text-transform: uppercase; letter-spacing: .04em; border-bottom: 1px solid #334155; }
  .th-hint { font-size: .68rem; color: #475569; text-transform: none; letter-spacing: 0; }
  .holdings-table td { padding: .5rem .75rem; border-bottom: 1px solid #0f172a; color: #cbd5e1; }
  .holdings-table tr:last-child td { border-bottom: none; }
  .holdings-table tr:hover td { background: #263145; }

  /* Category inline input */
  .cat-input {
    background: transparent;
    border: 1px solid transparent;
    color: #94a3b8;
    font-size: .83rem;
    padding: .2rem .4rem;
    border-radius: 4px;
    width: 130px;
    outline: none;
    transition: border-color .15s, background .15s;
  }
  .cat-input:hover { border-color: #334155; }
  .cat-input:focus { border-color: #3b82f6; background: #0f172a; color: #f1f5f9; }
  .cat-input::placeholder { color: #334155; font-style: italic; }

  .value-cell { font-weight: 600; color: #f1f5f9; }
  .actions-cell { text-align: right; white-space: nowrap; }
  .btn-icon { background: none; border: none; cursor: pointer; font-size: .95rem; padding: .15rem .3rem; border-radius: 4px; opacity: .6; }
  .btn-icon:hover { opacity: 1; background: #334155; }
  .btn-icon.danger:hover { background: #450a0a; }

  .form-row { display: flex; gap: .6rem; align-items: center; margin-top: 1rem; flex-wrap: wrap; }
  .input-ticker, .input-shares { background: #0f172a; border: 1px solid #334155; color: #f1f5f9; padding: .5rem .7rem; border-radius: 6px; font-size: .9rem; outline: none; }
  .input-ticker { width: 130px; }
  .input-shares { width: 150px; }
  .input-ticker:focus, .input-shares:focus { border-color: #3b82f6; }

  .btn-save { background: #3b82f6; color: #fff; border: none; padding: .5rem 1rem; border-radius: 6px; font-size: .9rem; cursor: pointer; }
  .btn-save:hover:not(:disabled) { background: #2563eb; }
  .btn-save:disabled { opacity: .5; }
  .btn-cancel { background: transparent; color: #94a3b8; border: 1px solid #334155; padding: .5rem .9rem; border-radius: 6px; font-size: .9rem; cursor: pointer; }
  .btn-cancel:hover { color: #f1f5f9; border-color: #64748b; }

  .btn-add { background: #1e3a5f; color: #60a5fa; border: 1px solid #1d4ed8; padding: .5rem 1rem; border-radius: 6px; font-size: .88rem; cursor: pointer; margin-top: .75rem; }
  .btn-add:hover { background: #1d3a5a; }

  code { font-family: monospace; background: #0f172a; padding: .1rem .4rem; border-radius: 3px; }
</style>
