import { writable } from 'svelte/store';

// Result of the last update check, shared by the tab bar badge and the settings page.
// { available: bool, latest: number, current: number, asset?: {url,size,sha256} } | null
export const updateInfo = writable(null);
