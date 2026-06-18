<script>
  import { onMount, onDestroy } from 'svelte';
  import Portfolio from './Portfolio.svelte';

  let loading = true;
  let authenticated = false;
  let email = '';
  let dbStatus = 'checking';
  let activeTab = 'portfolio';
  let healthInterval;

  onMount(async () => {
    try {
      const res = await fetch('/auth/me');
      const data = await res.json();
      authenticated = data.authenticated;
      email = data.email || '';
    } catch {
      // server unreachable
    } finally {
      loading = false;
    }
    if (authenticated) {
      await checkHealth();
      healthInterval = setInterval(checkHealth, 30000);
    }
  });

  onDestroy(() => clearInterval(healthInterval));

  async function checkHealth() {
    try {
      const res = await fetch('/health');
      const data = await res.json();
      dbStatus = data.db || 'unknown';
    } catch {
      dbStatus = 'unreachable';
    }
  }
</script>

{#if loading}
  <div class="splash">Chargement...</div>
{:else if !authenticated}
  <div class="login-wrap">
    <div class="login-card">
      <h1>Accès Restreint</h1>
      <p>Connectez-vous avec un compte Google autorisé.</p>
      <a href="/auth/login" class="btn-login">Connexion avec Google</a>
    </div>
  </div>
{:else}
  <div class="app">
    <!-- Status bar -->
    <header class="statusbar">
      <div class="status-left">
        <span class="status-item">
          <span class="dot dot-green"></span>
          {email}
        </span>
        <span class="status-item">
          <span class="dot dot-{dbStatus === 'connected' ? 'green' : dbStatus === 'error' ? 'red' : 'yellow'}"></span>
          DB: {dbStatus}
        </span>
      </div>
      <div class="status-right">
        <a href="/auth/logout" class="btn-logout">Déconnexion</a>
      </div>
    </header>

    <!-- Tab bar -->
    <nav class="tabbar">
      <button
        class="tab {activeTab === 'portfolio' ? 'active' : ''}"
        on:click={() => activeTab = 'portfolio'}
      >
        Portfolio
      </button>
    </nav>

    <!-- Tab content -->
    <main class="content">
      {#if activeTab === 'portfolio'}
        <Portfolio />
      {/if}
    </main>
  </div>
{/if}

<style>
  :global(*, *::before, *::after) { box-sizing: border-box; }
  :global(body) {
    margin: 0;
    font-family: system-ui, -apple-system, sans-serif;
    background: #0f172a;
    color: #f1f5f9;
    min-height: 100vh;
  }

  .splash {
    display: flex; align-items: center; justify-content: center;
    min-height: 100vh; color: #94a3b8; font-size: 1.2rem;
  }

  .login-wrap {
    display: flex; align-items: center; justify-content: center; min-height: 100vh;
  }
  .login-card {
    background: #1e293b; padding: 2.5rem 3rem; border-radius: 12px;
    text-align: center; box-shadow: 0 10px 30px rgba(0,0,0,.3);
  }
  .login-card h1 { margin: 0 0 .5rem; }
  .login-card p { color: #94a3b8; margin-bottom: 1.5rem; }
  .btn-login {
    background: #4285f4; color: #fff; border: none; padding: .7rem 1.4rem;
    border-radius: 6px; font-size: 1rem; cursor: pointer; text-decoration: none;
    display: inline-block;
  }

  .app { display: flex; flex-direction: column; min-height: 100vh; }

  .statusbar {
    background: #0f172a; border-bottom: 1px solid #1e293b;
    display: flex; align-items: center; justify-content: space-between;
    padding: .5rem 1.5rem; font-size: .82rem; color: #94a3b8;
  }
  .status-left { display: flex; gap: 1.2rem; align-items: center; }
  .status-item { display: flex; align-items: center; gap: .4rem; }
  .dot {
    width: 8px; height: 8px; border-radius: 50%; display: inline-block;
  }
  .dot-green { background: #10b981; }
  .dot-red   { background: #ef4444; }
  .dot-yellow{ background: #f59e0b; }
  .btn-logout {
    background: transparent; border: 1px solid #334155; color: #94a3b8;
    padding: .25rem .75rem; border-radius: 5px; font-size: .82rem;
    cursor: pointer; text-decoration: none;
  }
  .btn-logout:hover { border-color: #64748b; color: #f1f5f9; }

  .tabbar {
    background: #1e293b; border-bottom: 1px solid #334155;
    display: flex; padding: 0 1.5rem; gap: .25rem;
  }
  .tab {
    background: none; border: none; color: #94a3b8;
    padding: .75rem 1.2rem; font-size: .95rem; cursor: pointer;
    border-bottom: 2px solid transparent; transition: color .15s, border-color .15s;
  }
  .tab:hover { color: #f1f5f9; }
  .tab.active { color: #60a5fa; border-bottom-color: #60a5fa; }

  .content { flex: 1; padding: 1.5rem; }
</style>
