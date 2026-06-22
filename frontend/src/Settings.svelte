<script>
  import { createEventDispatcher } from 'svelte';

  export let enabledFeatures = [];

  const dispatch = createEventDispatcher();

  const ALL_FEATURES = [
    { id: 'portfolio',    label: 'Portfolio' },
    { id: 'telework',     label: 'Télétravail' },
    { id: 'lol-calendar', label: 'Calendrier LoL' },
    { id: 'f1',           label: 'Formule 1' },
  ];

  let saving = false;
  let saved = false;

  function isEnabled(id) {
    return enabledFeatures.includes(id);
  }

  function toggle(id) {
    if (isEnabled(id)) {
      enabledFeatures = enabledFeatures.filter(f => f !== id);
    } else {
      enabledFeatures = [...enabledFeatures, id];
    }
    saved = false;
  }

  async function save() {
    saving = true;
    try {
      const res = await fetch('/api/settings/features', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: enabledFeatures }),
      });
      if (!res.ok) throw new Error('save failed');
      const data = await res.json();
      enabledFeatures = data.enabled;
      saved = true;
      dispatch('change', { enabled: enabledFeatures });
    } finally {
      saving = false;
    }
  }
</script>

<div class="settings">
  <h2>Fonctionnalités</h2>
  <p class="hint">Activez les fonctionnalités que vous souhaitez utiliser. Par défaut, aucune n'est activée.</p>

  <div class="feature-list">
    {#each ALL_FEATURES as feature}
      <label class="feature-row">
        <input
          type="checkbox"
          checked={isEnabled(feature.id)}
          on:change={() => toggle(feature.id)}
        />
        <span class="feature-label">{feature.label}</span>
      </label>
    {/each}
  </div>

  <div class="actions">
    <button class="btn-save" on:click={save} disabled={saving}>
      {saving ? 'Enregistrement…' : 'Enregistrer'}
    </button>
    {#if saved}
      <span class="saved-msg">Sauvegardé</span>
    {/if}
  </div>
</div>

<style>
  .settings {
    max-width: 480px;
    margin: 2rem auto;
  }

  h2 {
    margin: 0 0 .5rem;
    font-size: 1.25rem;
    color: #f1f5f9;
  }

  .hint {
    color: #94a3b8;
    margin: 0 0 1.5rem;
    font-size: .9rem;
  }

  .feature-list {
    display: flex;
    flex-direction: column;
    gap: .75rem;
    margin-bottom: 1.5rem;
  }

  .feature-row {
    display: flex;
    align-items: center;
    gap: .75rem;
    background: #1e293b;
    border: 1px solid #334155;
    border-radius: 8px;
    padding: .75rem 1rem;
    cursor: pointer;
    transition: border-color .15s;
  }

  .feature-row:hover {
    border-color: #60a5fa;
  }

  .feature-row input[type="checkbox"] {
    width: 18px;
    height: 18px;
    accent-color: #60a5fa;
    cursor: pointer;
  }

  .feature-label {
    font-size: 1rem;
    color: #f1f5f9;
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .btn-save {
    background: #2563eb;
    color: #fff;
    border: none;
    padding: .55rem 1.25rem;
    border-radius: 6px;
    font-size: .95rem;
    cursor: pointer;
    transition: background .15s;
  }

  .btn-save:hover:not(:disabled) { background: #1d4ed8; }
  .btn-save:disabled { opacity: .5; cursor: default; }

  .saved-msg {
    color: #10b981;
    font-size: .9rem;
  }
</style>
