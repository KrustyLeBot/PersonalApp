package projection

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"helloauth/internal/portfolio"
)

var projectionYears = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 20}

// categoryReader is a minimal interface so the projection service can read ticker
// categories without depending on the full portfolio.Repo.
type categoryReader interface {
	GetTickerCategories(email string) (map[string]string, error)
	GetDistinctBourseTickers() ([]string, error)
}

// Service computes wealth projections.
type Service struct {
	repo          *Repo
	cagr          *CAGRClient
	portfolioRepo categoryReader
}

func NewService(repo *Repo, cagr *CAGRClient, portfolioRepo categoryReader) *Service {
	return &Service{repo: repo, cagr: cagr, portfolioRepo: portfolioRepo}
}

// BootstrapCAGRs runs RefreshTickerCAGRs for bourse tickers if no category rates exist yet.
// Call once at startup so projections are populated before the first daily refresh.
func (s *Service) BootstrapCAGRs() {
	if s.portfolioRepo == nil || s.repo.HasCategoryRates() {
		return
	}
	tickers, err := s.portfolioRepo.GetDistinctBourseTickers()
	if err != nil || len(tickers) == 0 {
		return
	}
	s.RefreshTickerCAGRs(tickers)
}

// categoryKey returns the projection_rates key for a category label.
func categoryKey(category string) string {
	slug := strings.ToLower(strings.ReplaceAll(category, " ", "_"))
	return "category_" + slug
}

// RefreshTickerCAGRs fetches CAGR for each bourse ticker and stores one rate per
// category (using the ticker with the longest history as representative).
// Tickers without a category fall back to their own key.
// Called after every ticker price refresh.
func (s *Service) RefreshTickerCAGRs(tickers []string) {
	categories := make(map[string]string)
	if s.portfolioRepo != nil {
		categories, _ = s.portfolioRepo.GetTickerCategories("")
	}

	type fetchResult struct {
		ticker string
		cagr   float64
		years  int
		err    error
	}

	ch := make(chan fetchResult, len(tickers))
	var wg sync.WaitGroup
	for _, t := range tickers {
		wg.Add(1)
		go func(ticker string) {
			defer wg.Done()
			cagr, years, err := s.cagr.FetchCAGR(ticker)
			ch <- fetchResult{ticker: ticker, cagr: cagr, years: years, err: err}
		}(t)
	}
	wg.Wait()
	close(ch)

	type cagrResult struct {
		cagr            float64
		years           int
		representTicker string
	}
	byTicker := make(map[string]cagrResult)
	for r := range ch {
		if r.err != nil {
			log.Printf("CAGR fetch for %s: %v", r.ticker, r.err)
			continue
		}
		byTicker[r.ticker] = cagrResult{cagr: r.cagr, years: r.years, representTicker: r.ticker}
	}

	// Per category: pick the ticker with the most years of history.
	type catBest struct {
		cagrResult
		label string
	}
	categoryBest := make(map[string]catBest)
	for ticker, res := range byTicker {
		cat, hasCat := categories[ticker]
		if !hasCat || cat == "" {
			continue
		}
		if prev, ok := categoryBest[cat]; !ok || res.years > prev.years {
			categoryBest[cat] = catBest{cagrResult: res, label: cat}
		}
	}

	// Save one rate per category.
	for cat, best := range categoryBest {
		rate := Rate{
			Key:       categoryKey(cat),
			Label:     fmt.Sprintf("%s (%d ans)", cat, best.years),
			Rate:      math.Round(best.cagr*100) / 100,
			SourceURL: fmt.Sprintf("https://finance.yahoo.com/quote/%s/history/", best.representTicker),
		}
		if err := s.repo.UpsertRate(rate); err != nil {
			log.Printf("save category CAGR for %s: %v", cat, err)
		}
	}

	// Tickers without a category get their own key.
	for _, ticker := range tickers {
		if cat, hasCat := categories[ticker]; hasCat && cat != "" {
			_ = cat
			continue
		}
		res, ok := byTicker[ticker]
		if !ok {
			continue
		}
		rate := Rate{
			Key:       tickerKey(ticker),
			Label:     fmt.Sprintf("%s (%d ans)", ticker, res.years),
			Rate:      math.Round(res.cagr*100) / 100,
			SourceURL: fmt.Sprintf("https://finance.yahoo.com/quote/%s/history/", ticker),
		}
		if err := s.repo.UpsertRate(rate); err != nil {
			log.Printf("save ticker CAGR for %s: %v", ticker, err)
		}
	}
}

// ComputeProjection builds the full projection summary from current portfolio state.
func (s *Service) ComputeProjection(
	assets []portfolio.Asset,
	holdings map[int][]portfolio.Holding,
	prices map[string]float64,
	dettes map[int]portfolio.DetteInfo,
	email string,
) (*ProjectionSummary, error) {
	categories, _ := func() (map[string]string, error) {
		if s.portfolioRepo != nil {
			return s.portfolioRepo.GetTickerCategories(email)
		}
		return make(map[string]string), nil
	}()

	rates, err := s.repo.GetAllRates()
	if err != nil {
		return nil, err
	}
	rateMap := make(map[string]Rate, len(rates))
	for _, r := range rates {
		rateMap[r.Key] = r
	}

	var projAssets []ProjectionAsset

	for _, a := range assets {
		var currentValue float64
		var appliedRate float64
		var rateKey string

		switch a.Type {
		case portfolio.TypeCrypto:
			for _, h := range holdings[a.ID] {
				currentValue += prices[h.Ticker] * h.Shares
			}
			projAssets = append(projAssets, ProjectionAsset{
				ID: a.ID, Name: a.Name, Type: a.Type,
				Current: currentValue, Values: flatValues(currentValue),
				RateKey: "", Rate: 0,
			})
			continue

		case portfolio.TypeImmobilier:
			currentValue = a.Value
			rateKey = immobilierKey(a.ID)
			if r, err := s.repo.EnsureRate(rateKey, a.Name, defaultImmobilierRate); err == nil && r != nil {
				rateMap[rateKey] = *r
				appliedRate = r.EffectiveRate()
			}

		case portfolio.TypeDette:
			if d, ok := dettes[a.ID]; ok {
				startDate, err := time.Parse("2006-01-02", d.StartDate)
				if err != nil {
					continue
				}
				now := time.Now().UTC()
				current := -portfolio.RemainingCapital(startDate, d.DurationMonths, d.TAEG, d.AmountBorrowed, now)
				vals := make(map[int]float64, len(projectionYears))
				for _, y := range projectionYears {
					future := now.AddDate(y, 0, 0)
					rem := portfolio.RemainingCapital(startDate, d.DurationMonths, d.TAEG, d.AmountBorrowed, future)
					vals[y] = -rem
				}
				projAssets = append(projAssets, ProjectionAsset{
					ID: a.ID, Name: a.Name, Type: a.Type,
					Current: current, Values: vals,
					RateKey: "", Rate: 0,
				})
			}
			continue

		case portfolio.TypeFondEuro:
			currentValue = a.Value
			rateKey = fondEuroKey(a.ID)
			if r, err := s.repo.EnsureRate(rateKey, a.Name, defaultFondEuroRate); err == nil && r != nil {
				rateMap[rateKey] = *r
				appliedRate = r.EffectiveRate()
			}

		case portfolio.TypeLivret:
			currentValue = a.Value
			rateKey = livretKey(a.ID)
			if r, err := s.repo.EnsureRate(rateKey, a.Name, defaultLivretRate); err == nil && r != nil {
				rateMap[rateKey] = *r
				appliedRate = r.EffectiveRate()
			}

		case portfolio.TypeStructure:
			currentValue = a.Value
			rateKey = structureKey(a.ID)
			if r, err := s.repo.EnsureRate(rateKey, a.Name, defaultStructureRate); err == nil && r != nil {
				rateMap[rateKey] = *r
				appliedRate = r.EffectiveRate()
			}

		case portfolio.TypeBourse:
			// Weighted average of effective rates across holdings.
			totalValue := 0.0
			weightedRate := 0.0
			dominantKey := ""
			dominantValue := 0.0

			for _, h := range holdings[a.ID] {
				pos := prices[h.Ticker] * h.Shares
				totalValue += pos
				currentValue += pos

				key := tickerKey(h.Ticker)
				if cat, hasCat := categories[h.Ticker]; hasCat && cat != "" {
					key = categoryKey(cat)
				}
				if r, ok := rateMap[key]; ok {
					weightedRate += r.EffectiveRate() * pos
					if pos > dominantValue {
						dominantValue = pos
						dominantKey = key
					}
				}
			}

			if totalValue > 0 {
				appliedRate = math.Round((weightedRate/totalValue)*100) / 100
			}
			rateKey = dominantKey
		}

		projAssets = append(projAssets, ProjectionAsset{
			ID:      a.ID,
			Name:    a.Name,
			Type:    a.Type,
			Current: currentValue,
			Values:  compoundValues(currentValue, appliedRate),
			RateKey: rateKey,
			Rate:    appliedRate,
		})
	}

	// Collect all rate keys used: asset-level + per-holding for bourse.
	usedKeys := make(map[string]struct{})
	for _, pa := range projAssets {
		if pa.RateKey != "" {
			usedKeys[pa.RateKey] = struct{}{}
		}
	}
	for _, a := range assets {
		if a.Type != portfolio.TypeBourse {
			continue
		}
		for _, h := range holdings[a.ID] {
			key := tickerKey(h.Ticker)
			if cat, hasCat := categories[h.Ticker]; hasCat && cat != "" {
				key = categoryKey(cat)
			}
			usedKeys[key] = struct{}{}
		}
	}

	// Read from rateMap, which includes lazily-ensured per-asset livret/fond euro
	// rates in addition to the initial snapshot.
	var usedRates []Rate
	for key := range usedKeys {
		if r, ok := rateMap[key]; ok {
			usedRates = append(usedRates, r)
		}
	}
	sort.Slice(usedRates, func(i, j int) bool { return usedRates[i].Key < usedRates[j].Key })

	return &ProjectionSummary{
		Years:  projectionYears,
		Assets: projAssets,
		Rates:  usedRates,
	}, nil
}

// tickerKey returns the projection_rates key for an uncategorised ticker.
func tickerKey(ticker string) string {
	return "ticker_" + ticker
}

// livretKey returns the per-asset projection_rates key for a livret.
func livretKey(assetID int) string {
	return fmt.Sprintf("livret_%d", assetID)
}

// fondEuroKey returns the per-asset projection_rates key for a fond euro.
func fondEuroKey(assetID int) string {
	return fmt.Sprintf("fond_euro_%d", assetID)
}

// structureKey returns the per-asset projection_rates key for a structured product.
func structureKey(assetID int) string {
	return fmt.Sprintf("structure_%d", assetID)
}

// immobilierKey returns the per-asset projection_rates key for a real estate property.
func immobilierKey(assetID int) string {
	return fmt.Sprintf("immobilier_%d", assetID)
}

func compoundValues(start, annualRatePct float64) map[int]float64 {
	result := make(map[int]float64, len(projectionYears))
	r := annualRatePct / 100.0
	for _, y := range projectionYears {
		result[y] = math.Round(start*math.Pow(1+r, float64(y))*100) / 100
	}
	return result
}

func flatValues(value float64) map[int]float64 {
	result := make(map[int]float64, len(projectionYears))
	for _, y := range projectionYears {
		result[y] = value
	}
	return result
}
