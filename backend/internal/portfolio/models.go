package portfolio

// Asset type constants — must match values stored in the DB `type` column.
const (
	TypeImmobilier = "immobilier"
	TypeFondEuro   = "fond_euro"
	TypeLivret     = "livret"
	TypeCrypto     = "crypto"
	TypeBourse     = "bourse"
	TypeDette      = "dette"
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
type TickerPrice struct {
	Ticker   string  `json:"ticker"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
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
	TickerCategories  map[string]string  `json:"ticker_categories"`  // ticker → category label
	LastRefresh       *string            `json:"last_refresh"`
	RefreshedToday    bool               `json:"refreshed_today"`
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
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}
