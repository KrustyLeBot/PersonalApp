# Personal Dashboard

A personal web app built to centralize tools I use daily. Access is restricted to authorized Google accounts.

**Stack:** Go · Svelte · PostgreSQL · Docker · Render

---

## Features

### 💰 Portfolio Tracker
Track your net worth across all asset classes in one place.

- **Asset types:** Real estate, Euro funds, Savings accounts, Crypto, Stock accounts
- **Stock accounts** (PEA, CTO, AV Bourse…) hold individual positions — each with a ticker and a number of shares
- **Live prices** fetched daily from Yahoo Finance on first visit, or on demand via a manual refresh button
- **Dashboard** shows total net worth, a breakdown pie chart by asset type, and a second pie chart showing stock allocation by ticker across all accounts
- Tickers use Yahoo Finance symbols — append `.PA` for Euronext Paris (e.g. `CW8.PA`), no suffix for US markets (`AAPL`)

---

## Project structure

```
.
├── backend/
│   ├── main.go                      # Entry point — init + route registration
│   └── internal/
│       ├── db/         db.go        # Database connection & migrations
│       ├── auth/                    # Google OAuth, session cookies, auth middleware
│       └── portfolio/               # Portfolio feature — models, repo, service, handlers
└── frontend/
    └── src/
        ├── App.svelte               # Shell — status bar, tab routing
        ├── Portfolio.svelte         # Portfolio dashboard
        ├── AssetModal.svelte        # Create / edit asset
        └── HoldingsModal.svelte     # Manage stock positions within an account
```

---

## Local development

**Prerequisites:** Go 1.22+, Node 18+, PostgreSQL

```bash
# 1. Copy and fill environment variables
cp .env.example .env

# 2. Backend
cd backend && go run .

# 3. Frontend (separate terminal)
cd frontend && npm install && npm run dev
```

The Vite dev server runs on port 5173 and proxies API calls to the Go backend on port 8080.

---

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | No | PostgreSQL DSN — app runs without it (no persistence) |
| `GOOGLE_CLIENT_ID` | Yes | OAuth client ID from Google Cloud Console |
| `GOOGLE_CLIENT_SECRET` | Yes | OAuth client secret |
| `GOOGLE_REDIRECT_URL` | Yes | Exact callback URL registered in Google Console |
| `SESSION_SECRET` | Yes | HMAC signing key — generate with `openssl rand -base64 32` |
| `ALLOWED_EMAILS` | Yes | Comma-separated whitelist of authorized emails |
| `PORT` | No | HTTP listen port (default `8080`, auto-set by Render) |

---

## Google OAuth setup (one-time)

1. Go to [Google Cloud Console](https://console.cloud.google.com) and create a project.
2. **APIs & Services → OAuth consent screen** — set user type to External, add your email as a test user.
3. **Credentials → Create → OAuth client ID** — type: Web application.
4. Add your redirect URI: `http://localhost:8080/auth/callback` for local dev, `https://your-app.onrender.com/auth/callback` for production.
5. Copy the Client ID and Secret into `.env`.

---

## Deploy on Render

1. Push to GitHub.
2. Render → New Web Service → connect repo → it detects the `Dockerfile` automatically.
3. Add environment variables in the Render dashboard.
4. Add the Render callback URL to your Google OAuth client's authorized redirect URIs.
