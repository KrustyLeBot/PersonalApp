import { writable } from 'svelte/store';

// syncing: a sync with Proton Drive runs (the app is read-only). offer: Synchrotron is installed but
// does not sync the data folder yet. notice: short message shown after a sync.
export const syncState = writable({ syncing: false, offer: false, notice: '' });
