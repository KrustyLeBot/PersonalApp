import App from './App.svelte';
import { installFetchRouter } from './local/router.js';

// The components talk to an HTTP-style /api; serve it from the local data files.
installFetchRouter();

const app = new App({
  target: document.getElementById('app'),
});

export default app;
