<script>
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { platform } from './platform/index.js';
  import * as store from './local/store.js';
  import { checkForUpdate, downloadUpdate, installUpdate } from './local/update.js';
  import { updateInfo } from './lib/updateState.js';

  const dispatch = createEventDispatcher();

  let info = null;
  store.getInfo().then((i) => (info = i));

  // --- Data folder ---
  let folderError = '';

  async function chooseFolder() {
    folderError = '';
    try {
      const res = await platform.pickFolder();
      if (res.changed) dispatch('reload');
    } catch (e) {
      folderError = e.message;
    }
  }

  // --- Updates ---
  // idle | checking | downloading | downloaded | installing
  let phase = 'idle';
  let message = '';
  let isError = false;
  let progress = { done: 0, total: 0 };
  let mounted = true;
  onDestroy(() => (mounted = false));

  $: pct = progress.total ? Math.round((progress.done / progress.total) * 100) : 0;

  async function check() {
    phase = 'checking';
    message = '';
    isError = false;
    try {
      const res = await checkForUpdate();
      updateInfo.set(res);
      if (!res.available) message = `À jour (build ${res.current}).`;
    } catch (e) {
      message = `Vérification impossible : ${e.message}`;
      isError = true;
    }
    if (mounted) phase = 'idle';
  }

  async function download() {
    phase = 'downloading';
    message = '';
    isError = false;
    progress = { done: 0, total: $updateInfo.asset.size };
    try {
      await downloadUpdate($updateInfo.asset, (done, total) => {
        progress = { done, total: total || progress.total };
      });
      phase = 'downloaded';
      await install();
    } catch (e) {
      message = e.message;
      isError = true;
      if (phase === 'downloading') phase = 'idle';
    }
  }

  async function install() {
    phase = 'installing';
    message = '';
    isError = false;
    try {
      await installUpdate();
      // Android hands over to the system installer; the Windows app restarts itself.
      phase = 'downloaded';
    } catch (e) {
      message = e.message;
      isError = true;
      phase = 'downloaded';
    }
  }
</script>

<div class="settings">
  <section>
    <h2>Données</h2>
    <p class="hint">
      Portfolio, marche et télétravail sont enregistrés dans des fichiers JSON
      (<code>portfolio.json</code>, <code>walk.json</code>, <code>telework.json</code>).
    </p>
    <div class="row">
      <span class="label">Dossier</span>
      <code>{info?.dataDir ?? '…'}</code>
    </div>
    <div class="actions">
      {#if platform.canChooseFolder}
        <button class="btn" on:click={chooseFolder}>Changer de dossier…</button>
      {/if}
      <button class="btn" on:click={() => dispatch('reload')}>Recharger depuis les fichiers</button>
    </div>
    {#if folderError}<p class="msg err">{folderError}</p>{/if}
  </section>

  <section>
    <h2>Mise à jour</h2>
    <div class="row">
      <span class="label">Version</span>
      <span>{info ? `${info.version ?? ''} (build ${info.build})` : '…'}</span>
    </div>

    {#if $updateInfo?.available}
      <div class="update-card">
        <strong>Build {$updateInfo.latest} disponible</strong>
        {#if phase === 'downloading'}
          <div class="bar"><div class="bar-fill" style="width: {pct}%"></div></div>
          <span class="hint">Téléchargement… {pct} %</span>
        {:else if phase === 'downloaded' || phase === 'installing'}
          <button class="btn primary" on:click={install} disabled={phase === 'installing'}>
            {phase === 'installing' ? 'Installation…' : 'Installer'}
          </button>
        {:else}
          <button class="btn primary" on:click={download}>Télécharger et installer</button>
        {/if}
      </div>
    {/if}

    <div class="actions">
      <button class="btn" on:click={check} disabled={phase !== 'idle'}>
        {phase === 'checking' ? 'Vérification…' : 'Vérifier maintenant'}
      </button>
    </div>
    {#if message}<p class="msg" class:err={isError}>{message}</p>{/if}
  </section>
</div>

<style>
  .settings { max-width: 520px; margin: 0 auto; display: flex; flex-direction: column; gap: 1.5rem; }
  section {
    background: #1e293b; border: 1px solid #334155; border-radius: 12px; padding: 1.1rem 1.25rem;
    display: flex; flex-direction: column; gap: .8rem;
  }
  h2 { margin: 0; font-size: 1.1rem; color: #f1f5f9; }
  .hint { color: #94a3b8; margin: 0; font-size: .88rem; line-height: 1.45; }
  .row { display: flex; flex-wrap: wrap; gap: .5rem 1rem; align-items: baseline; font-size: .92rem; }
  .label { color: #94a3b8; min-width: 4.5rem; }
  .actions { display: flex; flex-wrap: wrap; gap: .6rem; }
  .btn {
    background: #0f172a; color: #f1f5f9; border: 1px solid #334155; border-radius: 8px;
    padding: .55rem 1rem; font-size: .92rem; cursor: pointer; transition: border-color .15s, background .15s;
  }
  .btn:hover:not(:disabled) { border-color: #60a5fa; }
  .btn.primary { background: #2563eb; border-color: #2563eb; color: #fff; }
  .btn.primary:hover:not(:disabled) { background: #1d4ed8; }
  .btn:disabled { opacity: .55; cursor: default; }
  .update-card {
    display: flex; flex-direction: column; gap: .6rem; align-items: flex-start;
    border: 1px solid #f59e0b55; background: #f59e0b12; border-radius: 10px; padding: .8rem 1rem;
  }
  .bar { width: 100%; height: 6px; background: #334155; border-radius: 3px; overflow: hidden; }
  .bar-fill { height: 100%; background: #60a5fa; transition: width .15s; }
  .msg { margin: 0; font-size: .88rem; color: #94a3b8; }
  .msg.err { color: #f87171; }
</style>
