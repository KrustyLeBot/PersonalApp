package lolcalendar

import (
	"encoding/json"
	"log"
	"net/http"

	"helloauth/internal/auth"
)

type Handler struct {
	repo *Repo
	svc  *Service
}

func NewHandler(repo *Repo, svc *Service) *Handler {
	return &Handler{repo: repo, svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/lol-calendar/schedule",                 auth.RequireAuth(h.schedule))
	mux.HandleFunc("POST /api/lol-calendar/refresh",                 auth.RequireAuth(h.forceRefresh))
	mux.HandleFunc("GET /api/lol-calendar/leagues",                  auth.RequireAuth(h.listLeagues))
	mux.HandleFunc("GET /api/lol-calendar/leagues/available",        auth.RequireAuth(h.listAvailableLeagues))
	mux.HandleFunc("PUT /api/lol-calendar/leagues/{slug}",           auth.RequireAuth(h.updateLeague))
	mux.HandleFunc("POST /api/lol-calendar/matches/{id}/dismiss",    auth.RequireAuth(h.dismissSpoiler))
	mux.HandleFunc("GET /api/lol-calendar/matches/{id}/vods",        auth.RequireAuth(h.matchVODs))
	mux.HandleFunc("GET /api/lol-calendar/live",                     auth.RequireAuth(h.liveWindow))
	mux.HandleFunc("POST /api/lol-calendar/refresh-live",            auth.RequireAuth(h.refreshLive))
}

func (h *Handler) schedule(w http.ResponseWriter, r *http.Request, email string) {
	if _, err := h.svc.CheckAndRefreshDaily(email); err != nil {
		log.Printf("lol-calendar daily refresh: %v", err)
	}
	matches, err := h.repo.GetSchedule(pastDays, email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	if matches == nil {
		matches = []Match{}
	}
	jsonOK(w, map[string]any{
		"matches":     matches,
		"lastRefresh": h.repo.GetLastRefreshTime(email),
	})
}

func (h *Handler) forceRefresh(w http.ResponseWriter, r *http.Request, email string) {
	if err := h.svc.Refresh(email); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) listLeagues(w http.ResponseWriter, r *http.Request, email string) {
	leagues, err := h.repo.GetLeagues(email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, leagues)
}

// listAvailableLeagues fetches all leagues from the Riot API and merges
// the enabled state from the DB so the frontend can show the full picker.
func (h *Handler) listAvailableLeagues(w http.ResponseWriter, r *http.Request, email string) {
	all, err := h.svc.FetchAllLeagues()
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}

	// Build a set of enabled slugs from DB for this user.
	saved, err := h.repo.GetLeagues(email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	enabledSlugs := make(map[string]bool, len(saved))
	for _, l := range saved {
		enabledSlugs[l.Slug] = l.Enabled
	}

	for i := range all {
		all[i].Enabled = enabledSlugs[all[i].Slug]
	}

	jsonOK(w, all)
}

func (h *Handler) updateLeague(w http.ResponseWriter, r *http.Request, email string) {
	slug := r.PathValue("slug")
	var body struct {
		League
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	body.League.Slug = slug
	body.League.Enabled = body.Enabled
	if err := h.repo.UpsertLeague(body.League, email); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) liveWindow(w http.ResponseWriter, r *http.Request, email string) {
	matches, err := h.repo.GetLiveWindow(liveWindow, email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	if matches == nil {
		matches = []Match{}
	}
	jsonOK(w, matches)
}

func (h *Handler) refreshLive(w http.ResponseWriter, r *http.Request, email string) {
	if err := h.svc.RefreshLive(email); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) matchVODs(w http.ResponseWriter, r *http.Request, _ string) {
	id := r.PathValue("id")
	vods, err := h.svc.FetchVODs(id)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	if vods == nil {
		vods = []GameVOD{}
	}
	jsonOK(w, vods)
}

func (h *Handler) dismissSpoiler(w http.ResponseWriter, r *http.Request, email string) {
	id := r.PathValue("id")
	if err := h.repo.DismissSpoiler(id, email); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func apiError(w http.ResponseWriter, err error, code int) {
	log.Printf("HTTP %d: %v", code, err)
	http.Error(w, err.Error(), code)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("json encode: %v", err)
	}
}
