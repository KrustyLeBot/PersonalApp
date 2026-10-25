// Shared frontend-driven daily-refresh helpers.
// See "Fundamental rule: frontend-driven daily refresh" in CLAUDE.md.
//
// Pages that refresh external data once per calendar day render cached data
// first, then trigger the refresh only if it hasn't run today — so the page is
// never blank while the external fetch runs.

// True if the last server-side refresh wasn't today (local time). A missing or
// unparsable timestamp counts as stale so the refresh runs.
export function isStale(lastRefresh) {
  if (!lastRefresh) return true;
  const d = new Date(lastRefresh);
  if (Number.isNaN(d.getTime())) return true;
  return d.toDateString() !== new Date().toDateString();
}

// Run after the initial load has rendered cached data: if `lastRefresh` is stale,
// invoke `forceRefresh` (the same function the manual "Actualiser" button uses,
// which shows the button spinner and reloads on completion).
export async function autoRefreshIfStale(lastRefresh, forceRefresh) {
  if (isStale(lastRefresh)) await forceRefresh();
}
