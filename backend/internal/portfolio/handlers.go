package portfolio

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"helloauth/internal/auth"
)

// Handler exposes HTTP handlers for the /api/portfolio/* routes.
type Handler struct {
	repo *Repo
	svc  *Service
}

func NewHandler(repo *Repo, svc *Service) *Handler {
	return &Handler{repo: repo, svc: svc}
}

// RegisterRoutes attaches all portfolio routes to mux.
// Every route is wrapped in auth.RequireAuth — no portfolio endpoint is public.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Assets
	mux.HandleFunc("GET /api/portfolio/assets",         auth.RequireAuth(h.listAssets))
	mux.HandleFunc("POST /api/portfolio/assets",        auth.RequireAuth(h.createAsset))
	mux.HandleFunc("PUT /api/portfolio/assets/{id}",    auth.RequireAuth(h.updateAsset))
	mux.HandleFunc("DELETE /api/portfolio/assets/{id}", auth.RequireAuth(h.deleteAsset))

	// Bourse holdings (nested under an asset)
	mux.HandleFunc("GET /api/portfolio/assets/{id}/holdings",  auth.RequireAuth(h.listHoldings))
	mux.HandleFunc("POST /api/portfolio/assets/{id}/holdings", auth.RequireAuth(h.createHolding))
	mux.HandleFunc("PUT /api/portfolio/holdings/{id}",         auth.RequireAuth(h.updateHolding))
	mux.HandleFunc("DELETE /api/portfolio/holdings/{id}",      auth.RequireAuth(h.deleteHolding))

	// Dette info (one row per dette asset)
	mux.HandleFunc("PUT /api/portfolio/assets/{id}/dette", auth.RequireAuth(h.upsertDette))

	// Ticker categories
	mux.HandleFunc("PUT /api/portfolio/tickers/{ticker}/category",    auth.RequireAuth(h.upsertCategory))
	mux.HandleFunc("DELETE /api/portfolio/tickers/{ticker}/category", auth.RequireAuth(h.deleteCategory))

	// Summary + refresh
	mux.HandleFunc("GET /api/portfolio/summary",  auth.RequireAuth(h.summary))
	mux.HandleFunc("POST /api/portfolio/refresh", auth.RequireAuth(h.forceRefresh))
}

// --- Asset handlers ---

func (h *Handler) listAssets(w http.ResponseWriter, r *http.Request, email string) {
	assets, err := h.repo.GetAllAssets(email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, assets)
}

func (h *Handler) createAsset(w http.ResponseWriter, r *http.Request, email string) {
	var body struct {
		Asset
		Dette *DetteInfo `json:"dette"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.Type == "" || body.Name == "" {
		http.Error(w, "type and name are required", http.StatusBadRequest)
		return
	}
	id, err := h.repo.CreateAsset(body.Asset, email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	if body.Type == TypeDette && body.Dette != nil {
		body.Dette.AssetID = id
		if err := h.repo.UpsertDette(*body.Dette); err != nil {
			apiError(w, err, http.StatusInternalServerError)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int{"id": id})
}

func (h *Handler) updateAsset(w http.ResponseWriter, r *http.Request, email string) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var body struct {
		Asset
		Dette *DetteInfo `json:"dette"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.repo.UpdateAsset(id, body.Asset, email); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	if body.Type == TypeDette && body.Dette != nil {
		body.Dette.AssetID = id
		if err := h.repo.UpsertDette(*body.Dette); err != nil {
			apiError(w, err, http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteAsset(w http.ResponseWriter, r *http.Request, email string) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.repo.DeleteAsset(id, email); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Holdings handlers ---

func (h *Handler) listHoldings(w http.ResponseWriter, r *http.Request, email string) {
	assetID, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "invalid asset id", http.StatusBadRequest)
		return
	}
	holdings, err := h.repo.GetHoldingsByAsset(assetID, email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, holdings)
}

func (h *Handler) createHolding(w http.ResponseWriter, r *http.Request, _ string) {
	assetID, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "invalid asset id", http.StatusBadRequest)
		return
	}
	var hold Holding
	if err := json.NewDecoder(r.Body).Decode(&hold); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	hold.AssetID = assetID
	hold.Ticker = strings.ToUpper(strings.TrimSpace(hold.Ticker))
	if hold.Ticker == "" {
		http.Error(w, "ticker is required", http.StatusBadRequest)
		return
	}
	id, err := h.repo.CreateHolding(hold)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int{"id": id})
}

func (h *Handler) updateHolding(w http.ResponseWriter, r *http.Request, email string) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var hold Holding
	if err := json.NewDecoder(r.Body).Decode(&hold); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	hold.Ticker = strings.ToUpper(strings.TrimSpace(hold.Ticker))
	if err := h.repo.UpdateHolding(id, hold, email); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteHolding(w http.ResponseWriter, r *http.Request, email string) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.repo.DeleteHolding(id, email); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Dette handler ---

func (h *Handler) upsertDette(w http.ResponseWriter, r *http.Request, email string) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, "invalid asset id", http.StatusBadRequest)
		return
	}
	var d DetteInfo
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	d.AssetID = id
	if err := h.repo.UpsertDette(d); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Summary + refresh ---

func (h *Handler) upsertCategory(w http.ResponseWriter, r *http.Request, email string) {
	ticker := r.PathValue("ticker")
	var body struct {
		Category string `json:"category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Category) == "" {
		http.Error(w, "category is required", http.StatusBadRequest)
		return
	}
	if err := h.repo.UpsertTickerCategory(ticker, strings.TrimSpace(body.Category), email); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteCategory(w http.ResponseWriter, r *http.Request, email string) {
	ticker := r.PathValue("ticker")
	if err := h.repo.DeleteTickerCategory(ticker, email); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request, email string) {
	// Daily refresh is driven by the frontend (POST /refresh) so this GET returns
	// cached data instantly — see "frontend-driven daily refresh" in CLAUDE.md.
	assets, err := h.repo.GetAllAssets(email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	holdings, err := h.repo.GetAllHoldings(email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	prices, err := h.repo.GetTickerPrices()
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	categories, err := h.repo.GetTickerCategories(email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	dettes, err := h.repo.GetAllDettes(email)
	if err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	lastRefresh, _ := h.repo.GetLastRefreshTime()
	jsonOK(w, h.svc.ComputeSummary(assets, holdings, prices, categories, dettes, lastRefresh))
}

func (h *Handler) forceRefresh(w http.ResponseWriter, r *http.Request, _ string) {
	if err := h.svc.RefreshTickers(); err != nil {
		apiError(w, err, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

// --- helpers ---

func pathID(r *http.Request, key string) (int, error) {
	return strconv.Atoi(r.PathValue(key))
}

func apiError(w http.ResponseWriter, err error, code int) {
	log.Printf("HTTP %d: %v", code, err)
	http.Error(w, err.Error(), code)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
