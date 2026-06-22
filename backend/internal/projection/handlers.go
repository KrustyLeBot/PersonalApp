package projection

import (
	"encoding/json"
	"log"
	"net/http"

	"helloauth/internal/auth"
	"helloauth/internal/portfolio"
)

// Handler exposes HTTP handlers for the /api/projection/* routes.
type Handler struct {
	repo          *Repo
	svc           *Service
	portfolioRepo *portfolio.Repo
}

func NewHandler(repo *Repo, svc *Service, portfolioRepo *portfolio.Repo) *Handler {
	return &Handler{repo: repo, svc: svc, portfolioRepo: portfolioRepo}
}

// RegisterRoutes attaches all projection routes to mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/projection/rates",                       auth.RequireAuth(h.listRates))
	mux.HandleFunc("PUT /api/projection/rates/{key}",                 auth.RequireAuth(h.updateRate))
	mux.HandleFunc("PUT /api/projection/rates/{key}/rate-override",   auth.RequireAuth(h.updateRateOverride))
	mux.HandleFunc("GET /api/projection/summary",                     auth.RequireAuth(h.summary))
}

func (h *Handler) listRates(w http.ResponseWriter, r *http.Request, _ string) {
	rates, err := h.repo.GetAllRates()
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, rates)
}

func (h *Handler) updateRate(w http.ResponseWriter, r *http.Request, _ string) {
	key := r.PathValue("key")
	var body struct {
		Rate      float64 `json:"rate"`
		SourceURL string  `json:"source_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	existing, err := h.repo.GetRate(key)
	if err != nil || existing == nil {
		http.Error(w, "rate not found", http.StatusNotFound)
		return
	}

	existing.Rate = body.Rate
	existing.SourceURL = body.SourceURL
	if err := h.repo.UpsertRate(*existing); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// updateRateOverride sets or clears a manual rate override (in %) for a rate key.
// Body: {"rate": 14.5} to set, {"rate": null} to clear.
func (h *Handler) updateRateOverride(w http.ResponseWriter, r *http.Request, _ string) {
	key := r.PathValue("key")
	var body struct {
		Rate *float64 `json:"rate"` // null clears the override
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.repo.SetRateOverride(key, body.Rate); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request, email string) {
	assets, err := h.portfolioRepo.GetAllAssets(email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	holdings, err := h.portfolioRepo.GetAllHoldings(email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	prices, err := h.portfolioRepo.GetTickerPrices()
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	dettes, err := h.portfolioRepo.GetAllDettes(email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}

	summary, err := h.svc.ComputeProjection(assets, holdings, prices, dettes, email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, summary)
}

func apiError(w http.ResponseWriter, err error, code int) {
	log.Printf("HTTP %d: %v", code, err)
	http.Error(w, err.Error(), code)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
