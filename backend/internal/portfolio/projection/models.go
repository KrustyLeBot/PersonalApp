package projection

// Default rates applied when a per-asset livret or fond euro rate has not been
// set yet. Livrets and fonds euro each get their own editable rate keyed by
// asset ID (livret_<id> / fond_euro_<id>).
// Single-asset rates (livret, fond euro, structure, immobilier) are a single
// manual value the user sets; they default to 0 (flat) until then.
const (
	defaultLivretRate     = 0
	defaultFondEuroRate   = 0
	defaultStructureRate  = 0
	defaultImmobilierRate = 0
)

// Rate stores an annual return rate (in %) and its source for a given key.
type Rate struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Rate         float64  `json:"rate"`                    // computed annual return in percent
	SourceURL    string   `json:"source_url"`
	RateOverride *float64 `json:"rate_override,omitempty"` // user-set override; takes priority over computed rate
	UpdatedAt    string   `json:"updated_at,omitempty"`
}

// EffectiveRate returns the rate override if set, otherwise the computed rate.
func (r Rate) EffectiveRate() float64 {
	if r.RateOverride != nil {
		return *r.RateOverride
	}
	return r.Rate
}

// ProjectionAsset is a single line in the projection response.
type ProjectionAsset struct {
	ID      int               `json:"id"`
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Current float64           `json:"current"`
	Values  map[int]float64   `json:"values"` // year → projected value
	RateKey string            `json:"rate_key"`
	Rate    float64           `json:"rate"` // applied annual rate (%)
}

// ProjectionSummary is the full response for GET /api/projection/summary.
type ProjectionSummary struct {
	Years  []int             `json:"years"`
	Assets []ProjectionAsset `json:"assets"`
	Rates  []Rate            `json:"rates"`
}
