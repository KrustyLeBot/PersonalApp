package telework

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"helloauth/internal/auth"
)

// Handler exposes HTTP handlers for the /api/telework/* routes.
type Handler struct {
	repo *Repo
	svc  *Service
}

func NewHandler(repo *Repo, svc *Service) *Handler {
	return &Handler{repo: repo, svc: svc}
}

// RegisterRoutes attaches all telework routes to mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/telework/summary/{year}", auth.RequireAuth(h.summary))
	mux.HandleFunc("GET /api/telework/preset",         auth.RequireAuth(h.getPreset))
	mux.HandleFunc("PUT /api/telework/preset",         auth.RequireAuth(h.savePreset))
	mux.HandleFunc("GET /api/telework/overrides/{year}", auth.RequireAuth(h.getOverrides))
	mux.HandleFunc("PUT /api/telework/overrides/{year}", auth.RequireAuth(h.bulkSetOverrides))
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request, _ string) {
	year, err := pathYear(r)
	if err != nil {
		http.Error(w, "invalid year", http.StatusBadRequest)
		return
	}
	summary, err := h.svc.ComputeYear(year)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, summary)
}

func (h *Handler) getPreset(w http.ResponseWriter, r *http.Request, _ string) {
	p, err := h.repo.GetPreset()
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, p)
}

func (h *Handler) savePreset(w http.ResponseWriter, r *http.Request, _ string) {
	var p Preset
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.repo.SavePreset(p); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getOverrides(w http.ResponseWriter, r *http.Request, _ string) {
	year, err := pathYear(r)
	if err != nil {
		http.Error(w, "invalid year", http.StatusBadRequest)
		return
	}
	overrides, err := h.repo.GetOverrides(year)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, overrides)
}

// bulkSetOverrides accepts a JSON object mapping YYYY-MM-DD → type ("leave"|"remote"|"office").
func (h *Handler) bulkSetOverrides(w http.ResponseWriter, r *http.Request, _ string) {
	year, err := pathYear(r)
	if err != nil {
		http.Error(w, "invalid year", http.StatusBadRequest)
		return
	}
	var overrides map[string]string
	if err := json.NewDecoder(r.Body).Decode(&overrides); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	// Validate types
	for _, t := range overrides {
		if t != "leave" && t != "remote" && t != "office" {
			http.Error(w, "invalid override type: "+t, http.StatusBadRequest)
			return
		}
	}
	if err := h.repo.BulkSetOverrides(year, overrides); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

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

func apiError(w http.ResponseWriter, err error, code int) {
	http.Error(w, err.Error(), code)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
