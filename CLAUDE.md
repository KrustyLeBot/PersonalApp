# PersonalDashboard — Developer Reference

Personal dashboard (Portfolio, Marche, Télétravail) as two apps sharing one interface: an Android app (Capacitor) and a Windows app (Python + pywebview, using the system's Edge WebView2). Data is stored in JSON files; there is no server, database or account. The former Go + PostgreSQL web app was removed (it remains in the git history).

---

## Architecture

```
ui/                      Svelte 4 + Vite + Chart.js — the interface AND the business logic
  src/App.svelte         startup (load files, storage permission screen), tab bar, update badge
  src/Portfolio.svelte, AssetModal.svelte, HoldingsModal.svelte, ProjectionPanel.svelte
  src/Walk.svelte, Telework.svelte, Settings.svelte
  src/local/             logic ported from the old Go backend
    store.js             in-memory state + load/save of the JSON files (atomic, per-file write queue)
    portfolio.js, projection.js, rates.js, finance.js, yahoo.js, telework.js, walk.js
    router.js            answers the components' fetch('/api/...') calls from the local state
    update.js            checks the GitHub release « latest »
  src/platform/          android.js (Capacitor plugin AppBridge) / windows.js (HTTP bridge)
android/                 Capacitor project; AppBridgePlugin.java = data files, HTTP, self-update
windows/                 pywebview shell hosting the built UI (WebView2; no bundled browser engine)
  personaldashboard/bridge.py    local server: static UI + /__bridge/<name> (token-protected)
  personaldashboard/services.py  what the UI may ask the native side
  personaldashboard/updater.py   exe self-update (GitEnough approach)
branding/                logo SVG; icons are rendered from it
docs/RELEASE.md          CI, signing key, release, auto-update
```

### Platform interface (`ui/src/platform`)
Both platforms implement the same object: `getInfo()` (`build`, `version`, `dataDir`, `hasAccess`), `requestAccess()`, `readFile(name)`, `writeFile(name, content)`, `httpGet(url, headers)`, `downloadUpdate(asset, onProgress)`, `installUpdate()`, and `pickFolder()` on Windows only. All file access and all outgoing HTTP go through it (no CORS issue, one code path). **A new native capability must be added to both platforms.**

### Proton Drive sync (Android only)
The Android app asks Synchrotron (separate app, `KrustyLeBot/Synchrotron`) to sync `Documents/PersonalDashboard` with its backend (Proton Drive or the self-hosted Synchrotron server) when it opens: `SynchrotronClient.java` (copy of Synchrotron's client library) → `AppBridgePlugin.syncOnOpen` → `syncState` events → `local/syncOnOpen.js`. While a sync runs the app is read-only (`local/syncGuard.js`; `router.js` answers 423 to anything but GET and the prices refresh) and the data is re-read when the sync brought changes. The folder is added in Synchrotron (no setup button in the app); `.*.synctmp-*` and `*.tmp` are always excluded by Synchrotron and `cache.json` by the `.syncignore` file that `store.load` creates when missing. The Windows app has nothing to do (`platform.sync` is null): the folder is synced by Proton's own client, or is the Synchrotron server's SMB share; the app re-reads the files when they changed (window shown again, every 30 s).

### The `/api` shim
The components were written against an HTTP API and still call `fetch('/api/...')`. `router.js` replaces `window.fetch` for `/api/*` and dispatches to the services in `src/local/`, with the same request/response shapes as the old backend. New features can keep this pattern or call services directly.

---

## Data files (the file format)

Folder: `Documents/PersonalDashboard` on Android (fixed), user-chosen on Windows. Files: `portfolio.json`, `telework.json`, `walk.json`, `cache.json` (prices cache, safe to delete), optional `manifest.json` (`format_version`, currently 1). The format is documented in README.md.

Rules:
- A missing file means empty data. An unparsable file must raise an error and must **never be overwritten**.
- Every write is atomic (`.<name>.synctmp-<uuid>`, fsync, rename) and goes through `store.save(key)`, which serialises writes per file and refuses to replace a version it has not read (`ExternalChangeError`, the data is reloaded).
- A parse failure is retried once before the file is declared unreadable (a sync may be replacing it).
- Holding IDs exist only in memory (assigned at load); asset IDs are persisted and stable (rate keys like `livret_12` refer to them).
- Bumping `FORMAT_VERSION` in `store.js` is required for any incompatible change, with a migration on load.

---

## Daily refresh (prices)

Portfolio prices refresh once per calendar day, driven by the frontend: `GET /api/portfolio/summary` returns cached data instantly (with `last_refresh`), then `autoRefreshIfStale` from `src/lib/dailyRefresh.js` calls `POST /api/portfolio/refresh` (`portfolio.refreshTickers`). The manual « Actualiser » button calls the same function. The reload after a refresh must not flip the full-page loading state. If every price request fails (offline), no refresh is recorded so the next launch retries.

---

## Releases

Every push to `main` builds and publishes both apps (`.github/workflows/build.yml`, rolling release `latest`, title `PersonalDashboard build N`). Details, signing key and secrets: `docs/RELEASE.md`. Never commit a keystore; never print secrets in the workflow.

---

## Conventions

- **Language:** code, comments, variable names, commit messages in English; UI text in French.
- **One concern per file.** No comments for obvious code; only for non-obvious constraints or invariants.
- **No error swallowing:** propagate errors; the router turns them into HTTP-style responses and the UI displays them.
- **Same behaviour on both platforms:** logic lives in `ui/src/local/` once; the Windows and Android sides only provide platform capabilities.
- **Never run `go build`-style compile/start commands:** do not run `npm run build`, `gradlew`, `pyinstaller` or start the apps; the user tests and runs the project themselves (the CI builds the apps). Pure-logic unit tests (`npm test` in `ui/`, `python -m unittest discover -s tests -t .` in `windows/`) may be run.
