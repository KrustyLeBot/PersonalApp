<script>
  import { onMount } from 'svelte';
  import logo from './assets/logo.svg';
  import Portfolio from './Portfolio.svelte';
  import Walk from './Walk.svelte';
  import Telework from './Telework.svelte';
  import Settings from './Settings.svelte';
  import * as store from './local/store.js';
  import { StorageAccessError } from './local/errors.js';
  import { bootstrap } from './local/portfolio.js';
  import { checkForUpdate } from './local/update.js';
  import { updateInfo } from './lib/updateState.js';
  import { platform } from './platform/index.js';
  import { syncState } from './lib/syncState.js';
  import { startSyncOnOpen } from './local/syncOnOpen.js';

  const TABS = [
    { id: 'portfolio', label: 'Portfolio' },
    { id: 'walk',      label: 'Marche' },
    { id: 'telework',  label: 'Télétravail' },
  ];

  let phase = 'loading'; // loading | needsAccess | error | ready
  let error = '';
  let info = null;
  let activeTab = 'portfolio';
  // Bumped after the data is re-read from disk so every tab remounts on fresh data.
  let dataVersion = 0;

  async function start() {
    phase = 'loading';
    try {
      await store.load();
      info = await store.getInfo();
      phase = 'ready';
      dataVersion++;
      bootstrap().catch((e) => console.warn('CAGR bootstrap:', e));
      syncOnOpen().catch((e) => console.warn('sync on open:', e));
      checkForUpdate().then((r) => updateInfo.set(r)).catch(() => {});
    } catch (e) {
      if (e instanceof StorageAccessError) {
        info = await store.getInfo().catch(() => null);
        phase = 'needsAccess';
      } else {
        error = e.message;
        phase = 'error';
      }
    }
  }

  // Re-read the files after Synchrotron brought changes, without the loading screen.
  async function reloadData() {
    await store.load();
    dataVersion++;
  }

  let stopSync = () => {};
  async function syncOnOpen() {
    stopSync();
    stopSync = await startSyncOnOpen({ onState: (s) => syncState.set(s), reload: reloadData });
  }

  async function grantAccess() {
    await platform.requestAccess();
  }

  // Without Synchrotron (Windows), other devices change the files directly (server share, Proton's
  // client): re-read them when they changed, rather than keep showing and saving stale data.
  async function reloadIfChanged() {
    if (phase === 'ready' && !document.hidden && (await store.changedOnDisk())) await reloadData();
  }

  onMount(() => {
    store.setExternalChangeHandler(reloadData);
    start();
    // Coming back from the system settings screen after granting file access.
    const onVisible = () => {
      if (document.hidden) return;
      if (phase === 'needsAccess') start();
      else if (phase === 'ready') {
        syncOnOpen().catch((e) => console.warn('sync on open:', e));
        if (!platform.sync) reloadIfChanged().catch((e) => console.warn('reload:', e));
      }
    };
    document.addEventListener('visibilitychange', onVisible);
    const timer = platform.sync ? null : setInterval(() => reloadIfChanged().catch((e) => console.warn('reload:', e)), 30000);
    return () => {
      document.removeEventListener('visibilitychange', onVisible);
      if (timer) clearInterval(timer);
    };
  });

  function onSettingsReload() {
    start();
    activeTab = 'portfolio';
  }
</script>

{#if phase === 'loading'}
  <div class="splash"><img src={logo} alt="" class="splash-logo" />Chargement…</div>
{:else if phase === 'needsAccess'}
  <div class="center-wrap">
    <div class="card">
      <img src={logo} alt="" class="card-logo" />
      <h1>Accès aux fichiers</h1>
      <p>
        PersonalDashboard garde vos données dans des fichiers JSON, dans
        <code>{info?.dataDir ?? 'Documents/PersonalDashboard'}</code>.
        Autorisez l'accès à tous les fichiers pour que l'app puisse les lire et les écrire.
      </p>
      <button class="btn-primary" on:click={grantAccess}>Autoriser l'accès</button>
    </div>
  </div>
{:else if phase === 'error'}
  <div class="center-wrap">
    <div class="card">
      <h1>Lecture impossible</h1>
      <p class="err">{error}</p>
      <p>Aucun fichier n'a été modifié. Corrigez le fichier concerné puis réessayez.</p>
      <button class="btn-primary" on:click={start}>Réessayer</button>
    </div>
  </div>
{:else}
  <div class="app">
    <header class="statusbar">
      <div class="brand">
        <img src={logo} alt="" class="brand-logo" />
        <span class="brand-name">PersonalDashboard</span>
      </div>
      <span class="status-item" title={info?.dataDir}>
        <span class="dot dot-green"></span>
        <span class="path">{info?.dataDir}</span>
      </span>
    </header>

    {#if $syncState.syncing}
      <div class="sync-banner">Synchronisation avec Proton Drive… (lecture seule)</div>
    {:else if $syncState.notice}
      <div class="sync-banner">{$syncState.notice}</div>
    {/if}

    <nav class="tabbar">
      {#each TABS as tab}
        <button class="tab {activeTab === tab.id ? 'active' : ''}" on:click={() => (activeTab = tab.id)}>
          {tab.label}
        </button>
      {/each}
      <button class="tab {activeTab === 'settings' ? 'active' : ''}" on:click={() => (activeTab = 'settings')}>
        ⚙ Paramètres
        {#if $updateInfo?.available}<span class="badge" title="Mise à jour disponible"></span>{/if}
      </button>
    </nav>

    <main class="content">
      {#key dataVersion}
        {#if activeTab === 'portfolio'}
          <Portfolio />
        {:else if activeTab === 'walk'}
          <Walk />
        {:else if activeTab === 'telework'}
          <Telework />
        {:else}
          <Settings on:reload={onSettingsReload} />
        {/if}
      {/key}
    </main>
  </div>
{/if}

<style>
  :global(*, *::before, *::after) { box-sizing: border-box; }
  /* Capacitor injects --safe-area-inset-* where env() is not supported by the WebView. */
  :global(:root) {
    --sat: max(env(safe-area-inset-top, 0px), var(--safe-area-inset-top, 0px));
    --sab: max(env(safe-area-inset-bottom, 0px), var(--safe-area-inset-bottom, 0px));
  }
  :global(html, body) {
    margin: 0;
    font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
    background: #0f172a;
    color: #f1f5f9;
    min-height: 100vh;
    max-width: 100%;
    overflow-x: hidden;
    -webkit-tap-highlight-color: transparent;
  }
  :global(code) {
    background: #0f172a;
    border: 1px solid #334155;
    border-radius: 4px;
    padding: .1rem .35rem;
    font-size: .85em;
    word-break: break-all;
  }

  .splash {
    display: flex; flex-direction: column; gap: 1rem; align-items: center; justify-content: center;
    min-height: 100vh; color: #94a3b8; font-size: 1.1rem;
  }
  .splash-logo { width: 88px; height: 88px; border-radius: 22px; }

  .center-wrap {
    display: flex; align-items: center; justify-content: center; min-height: 100vh;
    padding: 1.25rem;
    padding-top: max(1.25rem, var(--sat));
  }
  .card {
    background: #1e293b; padding: 2rem 1.75rem; border-radius: 14px; max-width: 440px; width: 100%;
    text-align: center; box-shadow: 0 10px 30px rgba(0,0,0,.3);
  }
  .card-logo { width: 72px; height: 72px; border-radius: 18px; margin-bottom: .75rem; }
  .card h1 { margin: 0 0 .75rem; font-size: 1.35rem; }
  .card p { color: #94a3b8; line-height: 1.5; margin: 0 0 1.25rem; }
  .err { color: #f87171 !important; }
  .btn-primary {
    background: #2563eb; color: #fff; border: none; padding: .7rem 1.4rem;
    border-radius: 8px; font-size: 1rem; cursor: pointer;
  }
  .btn-primary:hover { background: #1d4ed8; }

  .app { display: flex; flex-direction: column; min-height: 100vh; }

  .sync-banner {
    background: #1e3a5f; color: #dbeafe; font-size: .85rem; padding: .5rem 1.5rem;
    display: flex; align-items: center; gap: .75rem; flex-wrap: wrap;
  }

  .statusbar {
    background: #0f172a; border-bottom: 1px solid #1e293b;
    display: flex; align-items: center; justify-content: space-between; gap: 1rem;
    padding: .55rem 1.5rem; padding-top: calc(.55rem + var(--sat));
    font-size: .82rem; color: #94a3b8;
  }
  .brand { display: flex; align-items: center; gap: .6rem; min-width: 0; }
  .brand-logo { width: 28px; height: 28px; border-radius: 7px; flex-shrink: 0; }
  .brand-name { font-weight: 600; color: #f1f5f9; font-size: .95rem; white-space: nowrap; }
  .status-item { display: flex; align-items: center; gap: .4rem; min-width: 0; }
  .path { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; direction: rtl; text-align: left; }
  .dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; flex-shrink: 0; }
  .dot-green { background: #10b981; }

  .tabbar {
    background: #1e293b; border-bottom: 1px solid #334155;
    display: flex; padding: 0 1.5rem; gap: .25rem;
    overflow-x: auto; -webkit-overflow-scrolling: touch; scrollbar-width: none;
  }
  .tabbar::-webkit-scrollbar { display: none; }
  .tab {
    position: relative;
    background: none; border: none; color: #94a3b8;
    padding: .75rem 1.2rem; font-size: .95rem; cursor: pointer;
    border-bottom: 2px solid transparent; transition: color .15s, border-color .15s;
    white-space: nowrap; flex-shrink: 0;
  }
  .tab:hover { color: #f1f5f9; }
  .tab.active { color: #60a5fa; border-bottom-color: #60a5fa; }
  .badge {
    position: absolute; top: .55rem; right: .45rem; width: 8px; height: 8px;
    border-radius: 50%; background: #f59e0b;
  }

  .content {
    flex: 1; padding: 1.5rem; min-width: 0;
    padding-bottom: calc(1.5rem + var(--sab));
  }

  @media (max-width: 480px) {
    .statusbar { padding-left: .75rem; padding-right: .75rem; }
    .path { max-width: 42vw; }
    .tabbar { padding: 0 .5rem; }
    .tab { padding: .65rem .8rem; font-size: .88rem; }
    .content { padding: 1rem; padding-bottom: calc(1rem + var(--sab)); }
  }
</style>
