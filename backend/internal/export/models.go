package export

// FormatVersion is bumped whenever the on-disk layout below changes incompatibly.
const FormatVersion = 1

// File names inside the archive. The mobile app reads/writes the same names.
const (
	ManifestFile  = "manifest.json"
	PortfolioFile = "portfolio.json"
	TeleworkFile  = "telework.json"
	WalkFile      = "walk.json"
)

// Manifest describes the archive. Ticker prices and other external caches are
// deliberately not exported: the app rebuilds them on first refresh.
type Manifest struct {
	FormatVersion int      `json:"format_version"`
	ExportedAt    string   `json:"exported_at"` // RFC 3339, UTC
	Files         []string `json:"files"`
}

// Portfolio is the content of portfolio.json. Asset IDs are kept as-is because
// per-asset projection rate keys (<type>_<id>) reference them.
type Portfolio struct {
	Assets           []Asset           `json:"assets"`
	TickerCategories map[string]string `json:"ticker_categories"` // ticker → category label
	ProjectionRates  []ProjectionRate  `json:"projection_rates"`
}

type Asset struct {
	ID        int       `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Value     float64   `json:"value"` // manual value; 0 for ticker-based types
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
	Holdings  []Holding `json:"holdings"`        // bourse and crypto only
	Dette     *Dette    `json:"dette,omitempty"` // dette only
}

type Holding struct {
	Ticker string  `json:"ticker"`
	Shares float64 `json:"shares"`
}

// Dette stores loan inputs only; monthly payment and remaining capital are recomputed.
type Dette struct {
	StartDate      string  `json:"start_date"` // YYYY-MM-DD
	DurationMonths int     `json:"duration_months"`
	TAEG           float64 `json:"taeg"` // percent per year
	AmountBorrowed float64 `json:"amount_borrowed"`
}

type ProjectionRate struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Rate         float64  `json:"rate"`
	SourceURL    string   `json:"source_url"`
	RateOverride *float64 `json:"rate_override"`
}

// Telework is the content of telework.json.
type Telework struct {
	RemoteDays []int                     `json:"remote_days"` // 0=Sunday..6=Saturday
	Overrides  map[string]TeleworkChoice `json:"overrides"`   // YYYY-MM-DD → per-half override
}

// TeleworkChoice holds "leave", "remote" or "office" ("" = follow the preset).
type TeleworkChoice struct {
	AM string `json:"am"`
	PM string `json:"pm"`
}

// Walk is the content of walk.json.
type Walk struct {
	Entries map[string]float64 `json:"entries"` // YYYY-MM-DD → km
}
