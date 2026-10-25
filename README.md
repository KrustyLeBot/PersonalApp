# PersonalDashboard

Personal dashboard for Android and Windows. All data lives in plain JSON files — no server, no database, no account.

| Tab | What it does |
|---|---|
| **Portfolio** | Net worth by asset class (real estate, euro funds, savings, crypto, stock accounts, structured products, loans). Live prices from Yahoo Finance, pie charts, wealth projection over 1–20 years |
| **Marche** | Daily walking distance, year grid, week/month/year totals |
| **Télétravail** | Remote-work calendar for Geneva (public holidays included), remote percentage against the 40 % threshold |

## Apps

- **Android** (`android/`, Capacitor): the data folder is fixed, `Documents/PersonalDashboard`. On first launch the app asks for « All files access » so it can read and write it.
- **Windows** (`windows/`, Python + pywebview + PyInstaller): a single, small `PersonalDashboard.exe` that shows the UI in the Microsoft Edge WebView2 already installed with Windows 10/11 (no browser engine is bundled). The data folder defaults to `Documents\PersonalDashboard` and can be changed in *Paramètres → Changer de dossier…* (e.g. a synced folder).

Both run the same interface (`ui/`, Svelte) and the same logic (`ui/src/local/`), and both update themselves from the GitHub release (see [docs/RELEASE.md](docs/RELEASE.md)).

## Data files

Put these files in the data folder (they are the content of the zip exported by the former web version):

| File | Content |
|---|---|
| `portfolio.json` | `assets[]` (each with `holdings[]` and, for loans, `dette`), `ticker_categories`, `projection_rates[]` |
| `telework.json` | `remote_days` (0 = Sunday … 6 = Saturday), `overrides` (`YYYY-MM-DD` → `{am, pm}` with `leave`, `remote` or `office`) |
| `walk.json` | `entries` (`YYYY-MM-DD` → km) |
| `cache.json` | Last Yahoo prices and refresh time. Written by the app; safe to delete |
| `manifest.json` | Optional: `format_version`. The app refuses a newer format than it knows (currently 1) |

Missing files are treated as empty. A file that cannot be parsed is reported and never overwritten. Writes are atomic (temporary file, then rename). Asset IDs are stable because per-asset projection rate keys (`livret_12`) refer to them.

To migrate from the old web app: unzip the export into the data folder.

## Project structure

```
ui/                    Svelte UI + local logic, shared by both apps
  src/local/           what the Go backend used to do: portfolio, projection, telework, walk, store, router
  src/platform/        android.js (Capacitor plugin) and windows.js (local bridge)
android/               Capacitor project; AppBridgePlugin.java = files, HTTP, self-update
windows/               pywebview shell: window + local bridge server + updater
branding/              logo (SVG source of every icon)
docs/RELEASE.md        build, signing, release, auto-update
.github/workflows/     build.yml
```

The components call `fetch('/api/…')` as before; `ui/src/local/router.js` answers those calls from the data files instead of a server.

## Tests

```bash
cd ui && npm test                                  # logic: loans, holidays, summaries, projections, router
cd windows && python -m unittest discover -s tests -t .   # bridge server, storage, config
```
