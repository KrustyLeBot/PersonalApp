package projection

// Rate keys for non-ticker asset types.
const (
	KeyLivretA  = "livret_a"
	KeyLDD      = "ldd"
	KeyFondEuro = "fond_euro"
)

// defaultRates holds the initial rate and source URL for each fixed key.
// Ticker CAGR entries are seeded separately during daily refresh.
var defaultRates = map[string]Rate{
	KeyLivretA: {
		Key:       KeyLivretA,
		Label:     "Livret A",
		Rate:      2.4,
		SourceURL: "",
	},
	KeyLDD: {
		Key:       KeyLDD,
		Label:     "LDD",
		Rate:      2.4,
		SourceURL: "",
	},
	KeyFondEuro: {
		Key:       KeyFondEuro,
		Label:     "Fonds Euro (moyenne)",
		Rate:      2.5,
		SourceURL: "https://www.linxea.com/assurance-vie/linxea-spirit-2/supports-disponibles-sur-linxea-spirit-2/fonds-euro-linxea-spirit-2-euro-objectif-climat/",
	},
}

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
