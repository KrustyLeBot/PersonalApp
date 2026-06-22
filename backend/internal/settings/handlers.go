package settings

import (
	"encoding/json"
	"net/http"

	"helloauth/internal/auth"
)

type Handler struct {
	repo *Repo
}

func NewHandler(repo *Repo) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings/features", auth.RequireAuth(h.getFeatures))
	mux.HandleFunc("PUT /api/settings/features", auth.RequireAuth(h.setFeatures))
}

func (h *Handler) getFeatures(w http.ResponseWriter, r *http.Request, email string) {
	features, err := h.repo.GetFeatures(email)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserFeatures{Enabled: features})
}

func (h *Handler) setFeatures(w http.ResponseWriter, r *http.Request, email string) {
	var body UserFeatures
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Filter to only known features.
	known := make(map[string]bool, len(KnownFeatures))
	for _, f := range KnownFeatures {
		known[f] = true
	}
	filtered := body.Enabled[:0]
	for _, f := range body.Enabled {
		if known[f] {
			filtered = append(filtered, f)
		}
	}

	if err := h.repo.SetFeatures(email, filtered); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserFeatures{Enabled: filtered})
}
