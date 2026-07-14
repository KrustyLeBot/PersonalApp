package portfolio

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

// RateProvider exposes projection rates to the portfolio summary without the
// portfolio package depending on the projection package (dependency inversion —
// the projection package implements this). Keys follow the projection_rates
// convention: "<type>_<assetID>" for single assets, "category_<slug>" /
// "ticker_<TICKER>" for bourse categories.
type RateProvider interface {
	// RateOverride returns the user override (%/an) for a key, or nil if none.
	RateOverride(key string) (*float64, error)
	// ComputedRate returns the auto-computed rate (%/an) for a key and whether it exists.
	ComputedRate(key string) (float64, bool, error)
}

// Service contains the business logic for the portfolio feature.
type Service struct {
	repo            *Repo
	ticker          *TickerClient
	rates           RateProvider            // optional; enriches summary with projection rates
	onTickerRefresh func(tickers []string) // optional hook called after price refresh
}

func NewService(repo *Repo, ticker *TickerClient) *Service {
	return &Service{repo: repo, ticker: ticker}
}

// SetRateProvider wires the projection rate provider used to enrich the summary.
func (s *Service) SetRateProvider(rp RateProvider) {
	s.rates = rp
}

// categoryRateKey mirrors projection.categoryKey / tickerKey: a ticker's rate is
// keyed by its category slug when categorised, else by the ticker itself.
func categoryRateKey(category string) string {
	slug := strings.ToLower(strings.ReplaceAll(category, " ", "_"))
	return "category_" + slug
}

func tickerRateKey(ticker string) string {
	return "ticker_" + ticker
}

// accountRateKey mirrors projection.<type>Key for single-asset editable rates.
func accountRateKey(assetType string, assetID int) string {
	return fmt.Sprintf("%s_%d", assetType, assetID)
}

// editableSingleRate reports whether an asset type carries a per-asset editable rate.
func editableSingleRate(assetType string) bool {
	_, ok := defaultSingleRate(assetType)
	return ok
}

// defaultSingleRate reports whether an asset type carries a per-asset editable
// rate. The default rate is always 0 — a single manual value the user sets.
func defaultSingleRate(assetType string) (float64, bool) {
	switch assetType {
	case TypeLivret, TypeFondEuro, TypeStructure, TypeImmobilier:
		return 0, true
	}
	return 0, false
}

// OnTickerRefresh registers a callback invoked with the refreshed ticker list
// after each successful price update. Used to trigger CAGR recomputation.
func (s *Service) OnTickerRefresh(fn func(tickers []string)) {
	s.onTickerRefresh = fn
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
	if s.onTickerRefresh != nil {
		bourseTickers, err := s.repo.GetDistinctBourseTickers()
		if err == nil {
			s.onTickerRefresh(bourseTickers)
		}
	}
	return s.repo.RecordDailyRefresh()
}

// ComputeSummary aggregates assets, holdings, prices, categories, and dettes into a Summary.
// ByCategory groups ticker positions by their assigned category label; tickers
// without a category fall back to the ticker symbol itself.
func (s *Service) ComputeSummary(
	assets []Asset,
	holdings map[int][]Holding,
	prices map[string]float64,
	dayChanges map[string]float64,
	categories map[string]string,
	dettes map[int]DetteInfo,
	lastRefresh *string,
) Summary {
	byType := make(map[string]float64)
	byCategory := make(map[string]float64)
	accountValues := make(map[int]float64)
	total := 0.0

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
				if a.Type == TypeBourse {
					byCategory[categoryLabel(h.Ticker)] += pos
				}
			}
			accountValues[a.ID] = val
		} else if a.Type == TypeDette {
			if d, ok := dettes[a.ID]; ok {
				val = -d.RemainingCapital
			}
			accountValues[a.ID] = val
		} else {
			val = a.Value
		}
		byType[a.Type] += val
		total += val
	}

	accountRates, categoryRates := s.buildRates(assets, holdings, categories, byCategory)

	return Summary{
		Total:            total,
		ByType:           byType,
		ByCategory:       byCategory,
		Assets:           assets,
		Holdings:         holdings,
		AccountValues:    accountValues,
		Dettes:           dettes,
		TickerPrices:     prices,
		TickerDayChanges: dayChanges,
		TickerCategories: categories,
		AccountRates:     accountRates,
		CategoryRates:    categoryRates,
		LastRefresh:      lastRefresh,
	}
}

// buildRates reads projection rates (if a provider is wired) and shapes them for
// the summary payload: per-asset overrides for single-asset types, and one entry
// per bourse category (or ticker fallback) with computed CAGR + override.
func (s *Service) buildRates(
	assets []Asset,
	holdings map[int][]Holding,
	categories map[string]string,
	byCategory map[string]float64,
) ([]AssetRate, []CategoryRate) {
	if s.rates == nil {
		return nil, nil
	}

	var accountRates []AssetRate
	for _, a := range assets {
		if _, ok := defaultSingleRate(a.Type); !ok {
			continue
		}
		key := accountRateKey(a.Type, a.ID)
		// Single manual value: an explicit override wins; otherwise a non-zero
		// stored rate (set via the legacy projection flow) also counts as "set".
		var rate float64
		isSet := false
		if ov, _ := s.rates.RateOverride(key); ov != nil {
			rate, isSet = *ov, true
		} else if r, exists, err := s.rates.ComputedRate(key); err == nil && exists && r != 0 {
			rate, isSet = r, true
		}
		accountRates = append(accountRates, AssetRate{
			AssetID: a.ID,
			Key:     key,
			Rate:    rate,
			IsSet:   isSet,
		})
	}

	// One rate entry per bourse category label (key = category slug, or ticker
	// fallback for uncategorised tickers). Resolve the label back to a key the
	// same way ComputeSummary grouped positions.
	keyForLabel := make(map[string]string) // category label → projection key
	for _, a := range assets {
		if a.Type != TypeBourse {
			continue
		}
		for _, h := range holdings[a.ID] {
			cat, ok := categories[h.Ticker]
			if ok && cat != "" {
				keyForLabel[cat] = categoryRateKey(cat)
			} else {
				keyForLabel[h.Ticker] = tickerRateKey(h.Ticker)
			}
		}
	}

	var categoryRates []CategoryRate
	for label := range byCategory {
		key, ok := keyForLabel[label]
		if !ok {
			continue
		}
		rate, exists, err := s.rates.ComputedRate(key)
		if err != nil || !exists {
			continue
		}
		override, _ := s.rates.RateOverride(key)
		categoryRates = append(categoryRates, CategoryRate{
			Category: label,
			Key:      key,
			Rate:     rate,
			Override: override,
		})
	}
	sort.Slice(categoryRates, func(i, j int) bool { return categoryRates[i].Category < categoryRates[j].Category })

	return accountRates, categoryRates
}
