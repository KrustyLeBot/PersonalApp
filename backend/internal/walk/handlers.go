package walk

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"helloauth/internal/auth"
)

// Handler exposes HTTP handlers for the /api/walk/* routes.
type Handler struct {
	repo *Repo
	svc  *Service
}

func NewHandler(repo *Repo, svc *Service) *Handler {
	return &Handler{repo: repo, svc: svc}
}

// RegisterRoutes attaches all walk routes to mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/walk/summary/{year}", auth.RequireAuth(h.summary))
	mux.HandleFunc("PUT /api/walk/day", auth.RequireAuth(h.setDay))
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request, email string) {
	year, err := pathYear(r)
	if err != nil {
		http.Error(w, "invalid year", http.StatusBadRequest)
		return
	}
	summary, err := h.svc.ComputeYear(year, email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, summary)
}

// setDay upserts one day's distance; km <= 0 clears the entry.
func (h *Handler) setDay(w http.ResponseWriter, r *http.Request, email string) {
	var e Entry
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	d, err := time.Parse("2006-01-02", e.Date)
	if err != nil {
		http.Error(w, "invalid date", http.StatusBadRequest)
		return
	}
	if d.Year() < time.Now().Year()-5 || d.After(time.Now()) {
		http.Error(w, "date out of range", http.StatusBadRequest)
		return
	}
	if e.Km < 0 || e.Km > 1000 {
		http.Error(w, "invalid distance", http.StatusBadRequest)
		return
	}
	if err := h.repo.SetDay(e, email); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pathYear(r *http.Request) (int, error) {
	y, err := strconv.Atoi(r.PathValue("year"))
	if err != nil {
		return 0, err
	}
	currentYear := time.Now().Year()
	if y < currentYear-5 || y > currentYear {
		return 0, strconv.ErrRange
	}
	return y, nil
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
