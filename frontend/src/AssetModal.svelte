<script>
  import { createEventDispatcher } from 'svelte';

  export let asset = null;
  export let dette = null; // existing DetteInfo if editing a dette asset

  const dispatch = createEventDispatcher();

  const TYPES = [
    { value: 'immobilier', label: 'Immobilier' },
    { value: 'fond_euro',  label: 'Fonds Euro' },
    { value: 'livret',     label: 'Livret' },
    { value: 'crypto',     label: 'Crypto (compte/wallet)' },
    { value: 'bourse',     label: 'Bourse (compte)' },
    { value: 'dette',      label: 'Dette / Emprunt' },
  ];

  let form = asset
    ? { id: asset.id, type: asset.type, name: asset.name, value: asset.value || 0 }
    : { type: 'immobilier', name: '', value: 0 };

  let detteForm = dette
    ? { start_date: dette.start_date, duration_months: dette.duration_months, taeg: dette.taeg, amount_borrowed: dette.amount_borrowed }
    : { start_date: new Date().toISOString().slice(0, 7) + '-01', duration_months: 240, taeg: 3.5, amount_borrowed: 0 };

  $: isTickerBased = form.type === 'bourse' || form.type === 'crypto';
  $: isDette = form.type === 'dette';

  // Compute monthly payment reactively for preview
  $: monthlyPayment = (() => {
    const p = Number(detteForm.amount_borrowed);
    const n = Number(detteForm.duration_months);
    const r = Number(detteForm.taeg) / 12 / 100;
    if (!p || !n) return 0;
    if (r === 0) return p / n;
    return p * r / (1 - Math.pow(1 + r, -n));
  })();

  const fmtEur = new Intl.NumberFormat('fr-FR', { style: 'currency', currency: 'EUR', maximumFractionDigits: 0 });

  function save() {
    if (!form.name.trim()) return;
    const payload = {
      id:    form.id,
      type:  form.type,
      name:  form.name.trim(),
      value: isTickerBased || isDette ? 0 : Number(form.value) || 0,
    };
    if (isDette) {
      payload.dette = {
        start_date:      detteForm.start_date,
        duration_months: Number(detteForm.duration_months),
        taeg:            Number(detteForm.taeg),
        amount_borrowed: Number(detteForm.amount_borrowed),
      };
    }
    dispatch('save', payload);
  }
</script>

<div class="overlay" role="dialog" aria-modal="true">
  <div class="modal">
    <h2>{asset ? 'Modifier l\'actif' : 'Ajouter un actif'}</h2>

    <label>
      Type
      <select bind:value={form.type} disabled={!!asset}>
        {#each TYPES as t}
          <option value={t.value}>{t.label}</option>
        {/each}
      </select>
    </label>

    <label>
      Nom
      <input type="text" bind:value={form.name}
        placeholder={
          isDette       ? 'Ex: Prêt immobilier, Crédit auto…' :
          isTickerBased ? (form.type === 'crypto' ? 'Ex: Binance, Ledger…' : 'Ex: PEA Fortuneo, CTO IBKR…') :
                          'Ex: Appartement Paris, Livret A…'
        } />
    </label>

    {#if isTickerBased}
      <p class="hint">
        Les positions (tickers + quantités) se gèrent après création via "Gérer les positions".
        {#if form.type === 'crypto'}
          Utilise les symboles Yahoo Finance : <code>BTC-EUR</code>, <code>ETH-EUR</code>…
        {:else}
          Ajoute <code>.PA</code> pour Euronext Paris : <code>CW8.PA</code>, <code>WPEA.PA</code>…
        {/if}
      </p>
    {:else if isDette}
      <label>
        Date de début
        <input type="date" bind:value={detteForm.start_date} />
      </label>
      <label>
        Durée (mois)
        <input type="number" bind:value={detteForm.duration_months} min="1" step="1"
          placeholder="Ex: 240 pour 20 ans" />
      </label>
      <label>
        TAEG (%)
        <input type="number" bind:value={detteForm.taeg} min="0" step="0.01"
          placeholder="Ex: 3.5" />
      </label>
      <label>
        Montant emprunté (€)
        <input type="number" bind:value={detteForm.amount_borrowed} min="0" step="100" />
      </label>
      {#if monthlyPayment > 0}
        <div class="dette-preview">
          Mensualité estimée : <strong>{fmtEur.format(monthlyPayment)}/mois</strong>
          — Durée : {Math.floor(detteForm.duration_months / 12)} ans {detteForm.duration_months % 12 > 0 ? `${detteForm.duration_months % 12} mois` : ''}
        </div>
      {/if}
    {:else}
      <label>
        Valeur (€)
        <input type="number" bind:value={form.value} min="0" step="0.01" />
      </label>
    {/if}

    <div class="modal-actions">
      <button class="btn-cancel" on:click={() => dispatch('close')}>Annuler</button>
      <button class="btn-save" on:click={save}>Enregistrer</button>
    </div>
  </div>
</div>

<style>
  .overlay { position: fixed; inset: 0; background: rgba(0,0,0,.6); display: flex; align-items: center; justify-content: center; z-index: 100; }
  .modal { background: #1e293b; border-radius: 12px; padding: 2rem; width: 100%; max-width: 420px; box-shadow: 0 20px 60px rgba(0,0,0,.5); max-height: 90vh; overflow-y: auto; }
  h2 { margin: 0 0 1.5rem; font-size: 1.1rem; color: #f1f5f9; }

  label { display: flex; flex-direction: column; gap: .35rem; font-size: .82rem; color: #94a3b8; margin-bottom: 1rem; }
  input, select { background: #0f172a; border: 1px solid #334155; color: #f1f5f9; padding: .55rem .75rem; border-radius: 6px; font-size: .95rem; outline: none; }
  input:focus, select:focus { border-color: #3b82f6; }
  select:disabled { opacity: .6; cursor: default; }

  .hint { background: #0f2744; border: 1px solid #1d4ed8; border-radius: 6px; padding: .75rem 1rem; font-size: .83rem; color: #93c5fd; margin: 0 0 1rem; }

  .dette-preview { background: #0f2218; border: 1px solid #166534; border-radius: 6px; padding: .65rem 1rem; font-size: .83rem; color: #86efac; margin-bottom: 1rem; }
  .dette-preview strong { color: #34d399; }

  .modal-actions { display: flex; justify-content: flex-end; gap: .75rem; margin-top: 1.5rem; }
  .btn-cancel { background: transparent; color: #94a3b8; border: 1px solid #334155; padding: .55rem 1.1rem; border-radius: 6px; font-size: .9rem; cursor: pointer; }
  .btn-cancel:hover { border-color: #64748b; color: #f1f5f9; }
  .btn-save { background: #3b82f6; color: #fff; border: none; padding: .55rem 1.2rem; border-radius: 6px; font-size: .9rem; cursor: pointer; }
  .btn-save:hover { background: #2563eb; }
</style>
