package export

import (
	"bytes"
	"log"
	"net/http"
	"time"

	"helloauth/internal/auth"
)

// Handler exposes the /api/export route.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/export", auth.RequireAuth(h.download))
}

// download buffers the archive so a failure yields a clean 500 instead of a truncated zip.
func (h *Handler) download(w http.ResponseWriter, r *http.Request, email string) {
	var buf bytes.Buffer
	if err := h.svc.WriteZip(&buf, email); err != nil {
		log.Printf("export: %v", err)
		http.Error(w, "export failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="personalapp-export-`+time.Now().Format("2006-01-02")+`.zip"`)
	w.Write(buf.Bytes())
}
