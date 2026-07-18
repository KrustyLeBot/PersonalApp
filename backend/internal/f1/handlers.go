package f1

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

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
	mux.HandleFunc("GET /api/f1/standings",                         auth.RequireAuth(h.standings))
	mux.HandleFunc("GET /api/f1/races",                             auth.RequireAuth(h.races))
	mux.HandleFunc("GET /api/f1/races/{season}/{round}/results",    auth.RequireAuth(h.raceResults))
	mux.HandleFunc("GET /api/f1/races/{season}/{round}/qualifying", auth.RequireAuth(h.qualifying))
	mux.HandleFunc("POST /api/f1/refresh",                          auth.RequireAuth(h.forceRefresh))
}

func (h *Handler) standings(w http.ResponseWriter, r *http.Request, _ string) {
	// Daily refresh is driven by the frontend (POST /refresh) so this GET returns
	// cached data instantly — see "Daily-refresh pages" in CLAUDE.md.
	year := currentSeason()
	drivers, err := h.repo.GetDriverStandings(year)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	constructors, err := h.repo.GetConstructorStandings(year)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	if drivers == nil {
		drivers = []DriverStanding{}
	}
	if constructors == nil {
		constructors = []ConstructorStanding{}
	}
	jsonOK(w, map[string]any{
		"drivers":      drivers,
		"constructors": constructors,
		"season":       year,
		"lastRefresh":  h.repo.GetLastRefreshTime(),
	})
}

func (h *Handler) races(w http.ResponseWriter, r *http.Request, _ string) {
	year := currentSeason()
	races, err := h.repo.GetRaces(year)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	if races == nil {
		races = []Race{}
	}
	jsonOK(w, map[string]any{
		"races":       races,
		"season":      year,
		"lastRefresh": h.repo.GetLastRefreshTime(),
	})
}

func (h *Handler) raceResults(w http.ResponseWriter, r *http.Request, _ string) {
	season, err := strconv.Atoi(r.PathValue("season"))
	if err != nil {
		http.Error(w, "invalid season", http.StatusBadRequest)
		return
	}
	round, err := strconv.Atoi(r.PathValue("round"))
	if err != nil {
		http.Error(w, "invalid round", http.StatusBadRequest)
		return
	}

	results, err := h.repo.GetRaceResults(season, round)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	if results == nil {
		results = []RaceResult{}
	}
	jsonOK(w, results)
}

func (h *Handler) qualifying(w http.ResponseWriter, r *http.Request, _ string) {
	season, err := strconv.Atoi(r.PathValue("season"))
	if err != nil {
		http.Error(w, "invalid season", http.StatusBadRequest)
		return
	}
	round, err := strconv.Atoi(r.PathValue("round"))
	if err != nil {
		http.Error(w, "invalid round", http.StatusBadRequest)
		return
	}

	results, err := h.repo.GetQualifyingResults(season, round)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	if results == nil {
		results = []QualifyingResult{}
	}
	jsonOK(w, results)
}

func (h *Handler) forceRefresh(w http.ResponseWriter, r *http.Request, _ string) {
	if err := h.svc.Refresh(); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
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
