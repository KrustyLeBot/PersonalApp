package portfolio

// Asset type constants — must match values stored in the DB `type` column.
const (
	TypeImmobilier = "immobilier"
	TypeFondEuro   = "fond_euro"
	TypeLivret     = "livret"
	TypeCrypto     = "crypto"
	TypeBourse     = "bourse"
	TypeDette      = "dette"
	TypeStructure  = "structure"
)

// hasTickerHoldings reports whether the given asset type uses ticker-based positions
// rather than a manual value. Both bourse and crypto follow this model.
func hasTickerHoldings(assetType string) bool {
	return assetType == TypeBourse || assetType == TypeCrypto
}

// Asset represents a single wealth entry.
// For bourse and crypto assets, Value is computed from ticker holdings — not stored directly.
type Asset struct {
	ID        int     `json:"id"`
	Type      string  `json:"type"`
	Name      string  `json:"name"`
	Value     float64 `json:"value,omitempty"` // manual value for non-ticker asset types
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// CategoryRate is the projection rate for a bourse category, editable from the
// portfolio page. Rate is the auto-computed CAGR; Override is the user value (if any).
type CategoryRate struct {
	Category string   `json:"category"`
	Key      string   `json:"key"`               // projection_rates key (category_<slug> or ticker_<TICKER>)
	Rate     float64  `json:"rate"`              // computed CAGR (%/an)
	Override *float64 `json:"override,omitempty"` // user override (%/an), null = auto
}

// AssetRate is the per-asset projection rate for single-asset types (livret,
// fond euro, structure, immobilier). It's a single manual value: Rate is the
// effective rate (%/an, 0 when never set), IsSet distinguishes "set to 0" from
// "not set" for display.
type AssetRate struct {
	AssetID int     `json:"asset_id"`
	Key     string  `json:"key"`    // projection_rates key (<type>_<id>)
	Rate    float64 `json:"rate"`   // effective rate (%/an), 0 when unset
	IsSet   bool    `json:"is_set"` // whether the user has set a value
}

// Holding is a ticker position inside a bourse or crypto account.
type Holding struct {
	ID        int     `json:"id"`
	AssetID   int     `json:"asset_id"`
	Ticker    string  `json:"ticker"`
	Shares    float64 `json:"shares"` // units held — shares for stocks, coins for crypto
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// TickerPrice stores the last known market price for a ticker.
// DayOpen is the first quotation of the current trading day, used to compute
// the intraday gain/loss percentage.
type TickerPrice struct {
	Ticker   string  `json:"ticker"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	DayOpen  float64 `json:"day_open"`
}

// DayChangePct returns the intraday variation vs the day's first quotation, in percent.
// Returns 0 when the open price is unknown.
func (p TickerPrice) DayChangePct() float64 {
	if p.DayOpen == 0 {
		return 0
	}
	return (p.Price - p.DayOpen) / p.DayOpen * 100
}

// Summary is the aggregated portfolio view returned by the summary endpoint.
type Summary struct {
	Total             float64            `json:"total"`
	ByType            map[string]float64 `json:"by_type"`
	ByCategory        map[string]float64 `json:"by_category"`       // category (or ticker) → total value, used for the chart
	Assets            []Asset            `json:"assets"`
	Holdings          map[int][]Holding  `json:"holdings"`           // asset_id → holdings
	AccountValues     map[int]float64    `json:"account_values"`     // asset_id → computed value
	Dettes            map[int]DetteInfo  `json:"dettes"`             // asset_id → dette info
	TickerPrices      map[string]float64 `json:"ticker_prices"`
	TickerDayChanges  map[string]float64 `json:"ticker_day_changes"`  // ticker → intraday variation vs day open, in percent
	TickerCategories  map[string]string  `json:"ticker_categories"`  // ticker → category label
	AccountRates      []AssetRate        `json:"account_rates"`      // per-asset projection rates for editable single-asset types
	CategoryRates     []CategoryRate     `json:"category_rates"`     // per-bourse-category projection rates, editable from the page
	LastRefresh       *string            `json:"last_refresh"`
}

// DetteInfo holds the loan parameters for a dette asset.
type DetteInfo struct {
	AssetID         int     `json:"asset_id"`
	StartDate       string  `json:"start_date"`        // "YYYY-MM-DD"
	DurationMonths  int     `json:"duration_months"`
	TAEG            float64 `json:"taeg"`              // annual rate in percent, e.g. 3.5
	AmountBorrowed  float64 `json:"amount_borrowed"`
	MonthlyPayment  float64 `json:"monthly_payment"`   // computed, not stored
	RemainingCapital float64 `json:"remaining_capital"` // computed as of today, not stored
}

// yahooChartResponse is the subset of the Yahoo Finance v8/chart JSON we parse.
type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				Currency           string  `json:"currency"`
				Symbol             string  `json:"symbol"`
			} `json:"meta"`
			Indicators struct {
				Quote []struct {
					Open []float64 `json:"open"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}
