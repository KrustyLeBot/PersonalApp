# PersonalApp — Developer Reference

Multi-purpose personal web app. Authentication via Google OAuth (single-user whitelist). Stack: Go backend + Svelte 4 frontend, PostgreSQL.

---

## Fundamental rule: authentication

**Every route except `/auth/*` and static files requires a valid session.**
There are no "partially public" API endpoints. This is enforced at the route registration level in `main.go` via `auth.RequireAuth`. When adding any new route, always wrap it — never inline the auth check.

---

## Fundamental rule: per-user data isolation

**Every feature's data is scoped to the authenticated user's email.**
All data tables (except global infrastructure like `ticker_prices`, `projection_rates`, `daily_refresh` for portfolio tickers) must have a `user_email VARCHAR(255) NOT NULL` column that is part of the primary key.

**When adding a new feature:**
1. Every data table must include `user_email VARCHAR(255) NOT NULL` as part of the PK.
2. All repo methods that read or write user data must accept an `email string` parameter and include `WHERE user_email = $N` (or insert with `user_email = $N`).
3. Handlers receive `email string` (never `_ string`) and pass it to repo/service methods.
4. Service methods that call repo methods must accept and propagate `email string`.
5. Global/shared tables (e.g. `ticker_prices`, `projection_rates`) are the exception — they cache external data shared across users.

**Migration pattern for existing tables:**
```sql
DO $$ BEGIN
  IF NOT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'my_table' AND column_name = 'user_email') THEN
    ALTER TABLE my_table DROP CONSTRAINT IF EXISTS my_table_pkey;
    ALTER TABLE my_table ADD COLUMN user_email VARCHAR(255) NOT NULL DEFAULT '';
    UPDATE my_table SET user_email = '' WHERE user_email = '';
    ALTER TABLE my_table ADD PRIMARY KEY (original_pk_col, user_email);
  END IF;
END $$;
```

The only intentionally public routes are:
| Route | Reason |
|---|---|
| `/auth/login` | Entry point for unauthenticated users |
| `/auth/callback` | OAuth redirect target |
| `/auth/logout` | Safe to call unauthenticated; clears cookie |
| `/auth/me` | SPA polls this on load to determine auth state |
| `/` (static files) | Auth is enforced in JS by checking `/auth/me` |

---

## Backend structure (`backend/`) — Go 1.22

```
main.go                    ← entry point only: init + RegisterRoutes + ListenAndServe
internal/
  db/
    db.go                  ← Database struct, New(), Migrate(), HealthHandler()
  auth/
    models.go              ← googleUserInfo (package-private)
    config.go              ← InitAllowedEmails, InitOAuth, IsAllowed, fetchGoogleUserInfo
    session.go             ← InitSession, CreateSessionCookie, ReadSessionCookie, ClearSessionCookie
    middleware.go          ← HandlerFunc type, RequireAuth
    handlers.go            ← Handler struct, RegisterRoutes (/auth/*)
  portfolio/
    models.go              ← Asset, TickerPrice, Summary, constants (TypeBourse etc.)
    repo.go                ← Repo struct — all SQL queries
    ticker.go              ← TickerClient — Yahoo Finance v7 API
    service.go             ← Service struct — RefreshTickers, CheckAndRefreshDaily, ComputeSummary
    handlers.go            ← Handler struct, RegisterRoutes (/api/portfolio/*)
```

**Adding a new feature/tab:**
1. Create `internal/<feature>/` with `models.go`, `repo.go`, `service.go`, `handlers.go`
2. Add new tables to `internal/db/db.go → Migrate()`
3. Instantiate and register in `main.go`
4. Every route in `handlers.go` must use `auth.RequireAuth`

**Package dependency graph** (no cycles):
```
main → auth, db, portfolio
portfolio → auth, db
auth → (none internal)
db → (none internal)
```

---

## Frontend structure (`frontend/src/`) — Svelte 4, Vite, Chart.js

| File | Responsibility |
|---|---|
| `App.svelte` | Auth gate, status bar (email + DB), tab bar, tab routing |
| `Portfolio.svelte` | Dashboard: total, pie charts (by type + by ticker), asset table, CRUD |
| `AssetModal.svelte` | Create/edit modal — fields adapt to asset type |

**Adding a tab:** add a button in `App.svelte`'s tab bar and a `{#if activeTab === '...'}` block. Create a new `<Feature>.svelte` component.

`checkHealth()` is only called when authenticated — `/health` is a protected endpoint.

---

## Domain model (portfolio)

### Asset types
| Constant | DB value | Value input |
|---|---|---|
| `TypeImmobilier` | `immobilier` | Manual € value |
| `TypeFondEuro` | `fond_euro` | Manual € value |
| `TypeLivret` | `livret` | Manual € value |
| `TypeCrypto` | `crypto` | Manual € value |
| `TypeBourse` | `bourse` | `ticker` + `shares` → computed from live price |

### Ticker convention
Yahoo Finance symbols: `CW8.PA` (Amundi World on Euronext), `BTC-EUR` (Bitcoin in EUR), `AAPL` (Apple USD). Client hits `query1.finance.yahoo.com/v7/finance/quote` — no API key, requires real `User-Agent` header.

### Daily refresh
On first `GET /api/portfolio/summary` each calendar day, `Service.CheckAndRefreshDaily()` fetches fresh prices and writes to `daily_refresh`. Subsequent calls skip the fetch. `POST /api/portfolio/refresh` forces an immediate refresh.

---

## Database schema

```sql
assets         (id, type, name, value, ticker, shares, created_at, updated_at)
ticker_prices  (ticker PK, price, currency, updated_at)
daily_refresh  (refresh_date PK, refreshed_at)
```

All tables created via `db.Migrate()` using `CREATE TABLE IF NOT EXISTS`.

---

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | No | PostgreSQL DSN — app runs in degraded mode without it |
| `GOOGLE_CLIENT_ID` | Yes | OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | Yes | OAuth client secret |
| `GOOGLE_REDIRECT_URL` | Yes | Must match Google Console (e.g. `http://localhost:8080/auth/callback`) |
| `SESSION_SECRET` | Yes | HMAC key — generate: `openssl rand -base64 32` |
| `ALLOWED_EMAILS` | Yes | Comma-separated whitelist: `me@gmail.com,other@gmail.com` |
| `PORT` | No | HTTP listen port (default `8080`) |

---

## Development

```bash
# Backend
cd backend && go run .

# Frontend dev server (HMR, proxied to :8080)
cd frontend && npm run dev

# Frontend production build → backend/static/
cd frontend && npm run build
```

---

## Conventions

- **Language:** all code, comments, variable names, commit messages in English.
- **No monolithic files:** one concern per file. Handlers, service, repo always in separate files.
- **No comments for obvious code.** Only comment non-obvious constraints, workarounds, or invariants.
- **Handler pattern:** handlers are methods on a `*Handler` struct with `RegisterRoutes(mux *http.ServeMux)`. Constructor is `NewHandler(deps...)`.
- **Repository pattern:** all SQL in `*Repo`. Services call repo. Handlers call service.
- **Auth enforcement:** use `auth.RequireAuth` at route registration. Never inline session checks in handlers.
- **No error swallowing:** propagate errors; log at handler level only.
- **Never run `go build`, `go run`, `npm run build`, or any compile/start command.** The user tests and runs the project themselves.
