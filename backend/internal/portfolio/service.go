package portfolio

import "log"

// Service contains the business logic for the portfolio feature.
type Service struct {
	repo   *Repo
	ticker *TickerClient
}

func NewService(repo *Repo, ticker *TickerClient) *Service {
	return &Service{repo: repo, ticker: ticker}
}

// RefreshTickers fetches current market prices for all holdings and persists them.
func (s *Service) RefreshTickers() error {
	tickers, err := s.repo.GetDistinctTickers()
	if err != nil {
		return err
	}
	if len(tickers) == 0 {
		return s.repo.RecordDailyRefresh()
	}
	prices, err := s.ticker.FetchPrices(tickers)
	if err != nil {
		return err
	}
	for _, p := range prices {
		if err := s.repo.SaveTickerPrice(p); err != nil {
			log.Printf("save ticker %s: %v", p.Ticker, err)
		}
	}
	return s.repo.RecordDailyRefresh()
}

// CheckAndRefreshDaily triggers RefreshTickers only if it has not run today.
// Returns true if a refresh was actually performed.
func (s *Service) CheckAndRefreshDaily() (bool, error) {
	done, err := s.repo.WasRefreshedToday()
	if err != nil || done {
		return false, err
	}
	return true, s.RefreshTickers()
}

// ComputeSummary aggregates assets, holdings, prices, and categories into a Summary.
// ByCategory groups ticker positions by their assigned category label; tickers
// without a category fall back to the ticker symbol itself.
func (s *Service) ComputeSummary(
	assets []Asset,
	holdings map[int][]Holding,
	prices map[string]float64,
	categories map[string]string,
	lastRefresh *string,
	refreshedToday bool,
) Summary {
	byType := make(map[string]float64)
	byCategory := make(map[string]float64)
	accountValues := make(map[int]float64)
	total := 0.0

	// categoryLabel returns the display label for a ticker — its category if
	// assigned, otherwise the ticker symbol itself.
	categoryLabel := func(ticker string) string {
		if cat, ok := categories[ticker]; ok && cat != "" {
			return cat
		}
		return ticker
	}

	for _, a := range assets {
		var val float64
		if hasTickerHoldings(a.Type) {
			for _, h := range holdings[a.ID] {
				pos := prices[h.Ticker] * h.Shares
				val += pos
				// Category chart is bourse-only — crypto is visible via the by_type chart.
				if a.Type == TypeBourse {
					byCategory[categoryLabel(h.Ticker)] += pos
				}
			}
			accountValues[a.ID] = val
		} else {
			val = a.Value
		}
		byType[a.Type] += val
		total += val
	}

	return Summary{
		Total:            total,
		ByType:           byType,
		ByCategory:       byCategory,
		Assets:           assets,
		Holdings:         holdings,
		AccountValues:    accountValues,
		TickerPrices:     prices,
		TickerCategories: categories,
		LastRefresh:      lastRefresh,
		RefreshedToday:   refreshedToday,
	}
}
