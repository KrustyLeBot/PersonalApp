<script>
  import { onMount } from 'svelte';

  let loading = true;
  let authenticated = false;
  let email = '';
  let helloMessage = '';
  let errorMsg = '';

  // On mount, ask the backend whether a valid session already exists.
  onMount(async () => {
    try {
      const res = await fetch('/auth/me');
      const data = await res.json();
      authenticated = data.authenticated;
      email = data.email || '';

      if (authenticated) {
        await loadHello();
      }
    } catch (e) {
      errorMsg = 'Unable to reach the server.';
    } finally {
      loading = false;
    }
  });

  async function loadHello() {
    const res = await fetch('/api/hello');
    if (res.ok) {
      const data = await res.json();
      helloMessage = data.message;
    }
  }

  function login() {
    window.location.href = '/auth/login';
  }

  function logout() {
    window.location.href = '/auth/logout';
  }
</script>

<main>
  {#if loading}
    <p>Loading...</p>
  {:else if errorMsg}
    <p class="error">{errorMsg}</p>
  {:else if !authenticated}
    <div class="card">
      <h1>Restricted Access</h1>
      <p>Sign in with an authorized Google account to access this page.</p>
      <button on:click={login}>Sign in with Google</button>
    </div>
  {:else}
    <div class="card">
      <h1>{helloMessage || 'Hello World'}</h1>
      <p class="subtitle">Signed in as {email}</p>
      <button on:click={logout}>Sign out</button>
    </div>
  {/if}
</main>

<style>
  :global(body) {
    margin: 0;
    font-family: system-ui, -apple-system, sans-serif;
    background: #0f172a;
    color: #f1f5f9;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
  }

  main {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
  }

  .card {
    background: #1e293b;
    padding: 2.5rem 3rem;
    border-radius: 12px;
    text-align: center;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3);
  }

  h1 {
    margin: 0 0 0.5rem;
    font-size: 1.8rem;
  }

  .subtitle {
    color: #94a3b8;
    margin-bottom: 1.5rem;
  }

  button {
    background: #4285f4;
    color: white;
    border: none;
    padding: 0.7rem 1.4rem;
    border-radius: 6px;
    font-size: 1rem;
    cursor: pointer;
  }

  button:hover {
    background: #3367d6;
  }

  .error {
    color: #f87171;
  }
</style>
