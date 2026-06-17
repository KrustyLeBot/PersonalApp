# Hello World with restricted Google login

Production URL: https://hello-ov0m.onrender.com

Small app: Svelte frontend + Go backend, packaged in a single Docker container.
Access to the page is only possible after signing in with Google, and only
emails configured in the `ALLOWED_EMAILS` environment variable are allowed.

## How authentication works

1. A visitor loads `/`; the Svelte frontend calls `GET /auth/me`.
2. If there is no valid session, a "Sign in with Google" button is shown.
   The button points to `GET /auth/login`, which redirects to Google's
   consent screen.
3. Google then redirects back to `GET /auth/callback?code=...&state=...`.
4. The backend exchanges that `code` for an access token (using the official
   `golang.org/x/oauth2` library), then calls Google's `userinfo` API with
   that token to obtain the user's verified email.
   -> This avoids manually decoding/validating a JWT and handling Google's
   JWKS certificates: the library + HTTPS call to Google's API handle that,
   reducing surface for errors.
5. If the email is verified AND present in the allowed email list, a signed session
   cookie (HMAC-SHA256) is set, valid for 7 days. Otherwise, access is denied
   (403).
6. All `/api/*` routes pass through the `requireAuth` middleware, which
   re-reads and re-validates the cookie on each request (signature,
   expiration, and that the email is still whitelisted).

The cookie is `HttpOnly`, `Secure` (requires HTTPS, so OK on Render), and
`SameSite=Lax`. It does not contain any Google tokens — only the email,
expiration, and signature.

## Create Google credentials (one-time setup)

1. Go to https://console.cloud.google.com and create or select a project.
2. **APIs & Services → OAuth consent screen**:
   - User type: External
   - Add your email to "Test users" (while the app is unpublished, ONLY test
     users can sign in — extra safety).
3. **APIs & Services → Credentials → Create Credentials → OAuth client ID**:
   - Type: Web application
   - Authorized redirect URIs: add the exact callback URL
     (`http://localhost:8080/auth/callback` for local development,
     `https://your-app.onrender.com/auth/callback` in production).
4. Retrieve the generated **Client ID** and **Client Secret**.

## Configuration

Copy `.env.example` to `.env` and fill the values:

```bash
cp .env.example .env
```

## Run locally (without Docker, for development)

Backend:
```bash
cd backend
export $(cat ../.env | xargs)   # load environment variables
go mod tidy
go run .
```

Frontend (in another terminal, for hot-reload during development):
```bash
cd frontend
npm install
npm run dev
```
Vite dev server runs on a separate port (5173 by default); for development,
configure a Vite proxy to `localhost:8080` if you want to test the full
auth flow without rebuilding. For a production-faithful test, prefer the
Docker method below.

## Build and run with Docker (recommended for prod-like testing)

```bash
docker build -t hello-auth .
docker run --rm -p 8080:8080 --env-file .env hello-auth
```

Visit http://localhost:8080 — you should see the Google sign-in button.

## Deploy on Render

1. Push this project to a GitHub repository.
2. On Render: New → Web Service → connect your repo.
3. Render will detect the `Dockerfile` automatically.
4. In the service's **Environment Variables** on Render, add:
   - `GOOGLE_CLIENT_ID`
   - `GOOGLE_CLIENT_SECRET`
   - `GOOGLE_REDIRECT_URL` → `https://<your-service-name>.onrender.com/auth/callback`
   - `SESSION_SECRET` → a long random value (e.g. `openssl rand -base64 32`)
   - `ALLOWED_EMAILS` → comma-separated authorized emails
5. Go back to Google Cloud Console → Credentials → your OAuth client → add
   the same `https://<your-service-name>.onrender.com/auth/callback` URL in
   "Authorized redirect URIs" (Render provides the final URL after first
   deployment — update it later if needed).
6. Deploy. Render injects a `PORT` environment variable automatically; the
   Go server already uses `os.Getenv("PORT")`, so no code changes are needed.

## Configure allowed emails

Set the `ALLOWED_EMAILS` environment variable with a comma-separated list
of authorized email addresses:

```bash
ALLOWED_EMAILS=other.email@example.com,other2.email@example.com
```

Or add it to `.env` / Render environment variables.

Rebuild the image after any modification.

## Project structure

```
.
├── Dockerfile
├── .env.example
├── backend/
│   ├── go.mod
│   ├── main.go      # HTTP routes, auth middleware, static file server
│   ├── auth.go      # Google OAuth config, code exchange, email whitelist
│   └── session.go   # signed session cookie creation/validation
└── frontend/
    ├── package.json
    ├── vite.config.js
    ├── index.html
    └── src/
        ├── main.js
        └── App.svelte # queries /auth/me, shows login or protected content
```
