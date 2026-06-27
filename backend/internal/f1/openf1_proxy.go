package f1

import (
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenF1's public API dropped browser CORS support, so the frontend can no longer
// call api.openf1.org directly. We proxy it server-side: a server request isn't
// subject to CORS and carries a real User-Agent. Only GET, only the openf1 host,
// and the query string is forwarded verbatim — this is a read-only relay.

const openF1BaseURL = "https://api.openf1.org/v1"

// openF1Allowed is the set of endpoints the replay UI needs. Restricting the
// proxy to these avoids turning it into an open relay to arbitrary openf1 paths.
var openF1Allowed = map[string]bool{
	"sessions":     true,
	"drivers":      true,
	"position":     true,
	"intervals":    true,
	"laps":         true,
	"location":     true,
	"race_control": true,
}

type openF1Proxy struct {
	http *http.Client
}

func newOpenF1Proxy() *openF1Proxy {
	return &openF1Proxy{http: &http.Client{Timeout: 20 * time.Second}}
}

// handle relays GET /api/f1/openf1/{endpoint}?<query> to the OpenF1 API.
func (p *openF1Proxy) handle(w http.ResponseWriter, r *http.Request, _ string) {
	endpoint := r.PathValue("endpoint")
	if !openF1Allowed[endpoint] {
		http.Error(w, "unknown openf1 endpoint", http.StatusNotFound)
		return
	}

	url := openF1BaseURL + "/" + endpoint
	if q := r.URL.RawQuery; q != "" {
		url += "?" + q
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		http.Error(w, "openf1 upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct == "" || strings.HasPrefix(ct, "application/json") {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
