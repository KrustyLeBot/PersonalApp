import { writable } from 'svelte/store';

// syncing: a sync with Proton Drive runs (the app is read-only). notice: short message shown after a sync.
export const syncState = writable({ syncing: false, notice: '' });
