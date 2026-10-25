// While Synchrotron is syncing the data folder, changes are refused so they cannot collide with files
// being downloaded. Reads, and the prices refresh (cache.json, never synced), stay allowed.
let readOnly = false;

export function setReadOnly(value) {
  readOnly = value;
}

export function isReadOnly() {
  return readOnly;
}
