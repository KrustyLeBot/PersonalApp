package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// Handler holds CSRF state and exposes HTTP handlers for the /auth/* routes.
// /auth/me and /auth/logout are intentionally public — they are safe to call
// unauthenticated and required for the login flow. All other routes (/api/*)
// must go through RequireAuth.
type Handler struct {
	stateMu sync.Mutex
	states  map[string]time.Time
}

func NewHandler() *Handler {
	return &Handler{states: make(map[string]time.Time)}
}

// RegisterRoutes attaches all /auth/* routes to mux.
// These routes are deliberately NOT wrapped in RequireAuth.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/auth/login", h.login)
	mux.HandleFunc("/auth/callback", h.callback)
	mux.HandleFunc("/auth/logout", h.logout)
	mux.HandleFunc("/auth/me", h.me) // returns {authenticated: bool, email?}
}

func (h *Handler) generateState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(b)
	h.stateMu.Lock()
	h.states[state] = time.Now().Add(5 * time.Minute)
	h.stateMu.Unlock()
	return state, nil
}

func (h *Handler) consumeState(state string) bool {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	expiry, ok := h.states[state]
	if !ok {
		return false
	}
	delete(h.states, state)
	return time.Now().Before(expiry)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	state, err := h.generateState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, oauthConfig.AuthCodeURL(state), http.StatusFound)
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	if !h.consumeState(r.URL.Query().Get("state")) {
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
		log.Printf("fetchGoogleUserInfo: %v", err)
		http.Error(w, "Google authentication failed", http.StatusUnauthorized)
		return
	}
	if !userInfo.VerifiedEmail || !IsAllowed(userInfo.Email) {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}
	http.SetCookie(w, CreateSessionCookie(userInfo.Email))
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, ClearSessionCookie())
	http.Redirect(w, r, "/", http.StatusFound)
}

// me returns auth state as JSON. The frontend polls this on load.
// Must stay public — the SPA needs it before showing the login button.
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	email, ok := ReadSessionCookie(r)
	if !ok {
		json.NewEncoder(w).Encode(map[string]any{"authenticated": false})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"authenticated": true, "email": email})
}
