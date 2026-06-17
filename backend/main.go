package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// stateStore keeps temporary OAuth "state" values (anti-CSRF) with a short
// expiration. A simple map is sufficient for this usage (single authorized
// user); no Redis or external store required.
var (
	stateStore   = map[string]time.Time{}
	stateStoreMu sync.Mutex
)

func generateState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(b)

	stateStoreMu.Lock()
	stateStore[state] = time.Now().Add(5 * time.Minute)
	stateStoreMu.Unlock()

	return state, nil
}

func consumeState(state string) bool {
	stateStoreMu.Lock()
	defer stateStoreMu.Unlock()

	expiry, ok := stateStore[state]
	if !ok {
		return false
	}
	delete(stateStore, state) // un state ne s'utilise qu'une fois
	return time.Now().Before(expiry)
}

func main() {
	if err := initSession(); err != nil {
		log.Fatalf("init session: %v", err)
	}
	if err := initAllowedEmails(); err != nil {
		log.Fatalf("init allowed emails: %v", err)
	}
	if err := initOAuth(); err != nil {
		log.Fatalf("init oauth: %v", err)
	}

	mux := http.NewServeMux()

	// --- Authentication routes ---

	// /auth/login : redirect the user to Google's consent screen.
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		state, err := generateState()
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		url := oauthConfig.AuthCodeURL(state)
		http.Redirect(w, r, url, http.StatusFound)
	})

	// /auth/callback : Google redirects here after consent with a "code".
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		if !consumeState(state) {
			http.Error(w, "invalid or expired OAuth state", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing OAuth code", http.StatusBadRequest)
			return
		}

		userInfo, err := fetchGoogleUserInfo(r.Context(), code)
		if err != nil {
			log.Printf("error fetchGoogleUserInfo: %v", err)
			http.Error(w, "Google authentication failed", http.StatusUnauthorized)
			return
		}

		if !userInfo.VerifiedEmail || !isAllowed(userInfo.Email) {
			// Do not reveal details to the unauthorized user; return a
			// generic forbidden response.
			http.Error(w, "access denied: this email is not authorized", http.StatusForbidden)
			return
		}

		http.SetCookie(w, createSessionCookie(userInfo.Email))
		http.Redirect(w, r, "/", http.StatusFound)
	})

	// /auth/logout : clear the session cookie.
	mux.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, clearSessionCookie())
		http.Redirect(w, r, "/", http.StatusFound)
	})

	// /auth/me : returns authentication state as JSON, used by the Svelte frontend.
	mux.HandleFunc("/auth/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		email, ok := readSessionCookie(r)
		if !ok {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"authenticated": false})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"authenticated": true, "email": email})
	})

	// --- Protected API ---

	// /api/hello : example protected route, responds only to allowed users.
	mux.HandleFunc("/api/hello", requireAuth(func(w http.ResponseWriter, r *http.Request, email string) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Hello World",
			"user":    email,
		})
	}))

	// --- Static files for the Svelte frontend ---
	// The frontend controls displaying the login button / protected content
	// by querying /auth/me. Serve the Svelte build as-is.
	staticDir := "./static"
	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("/", fs)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("server started on port %s", port)
	if err := http.ListenAndServe("0.0.0.0:"+port, mux); err != nil {
		log.Fatal(err)
	}
}

// requireAuth is middleware that verifies the session before calling the handler.
func requireAuth(next func(w http.ResponseWriter, r *http.Request, email string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email, ok := readSessionCookie(r)
		if !ok {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		next(w, r, email)
	}
}
