package portfolio

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// TickerClient fetches live market prices from Yahoo Finance.
// Uses the v8/finance/chart endpoint (one request per ticker, parallelised)
// which does not require authentication unlike the deprecated v7/quote batch endpoint.
type TickerClient struct {
	http *http.Client
}

func NewTickerClient() *TickerClient {
	return &TickerClient{
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

// FetchPrices retrieves current prices for all given tickers in parallel.
// Tickers not found or returning an error are silently omitted.
// Use Yahoo Finance symbols: append ".PA" for Euronext Paris (e.g. CW8.PA),
// ".L" for London, no suffix for US markets.
func (c *TickerClient) FetchPrices(tickers []string) (map[string]TickerPrice, error) {
	if len(tickers) == 0 {
		return nil, nil
	}

	type result struct {
		price TickerPrice
		err   error
	}

	ch := make(chan result, len(tickers))
	var wg sync.WaitGroup

	for _, ticker := range tickers {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			p, err := c.fetchOne(t)
			ch <- result{price: p, err: err}
		}(ticker)
	}

	wg.Wait()
	close(ch)

	prices := make(map[string]TickerPrice, len(tickers))
	for r := range ch {
		if r.err == nil && r.price.Ticker != "" {
			prices[r.price.Ticker] = r.price
		}
	}
	return prices, nil
}

func (c *TickerClient) fetchOne(ticker string) (TickerPrice, error) {
	// range=1d&interval=1d yields the day's open in indicators.quote[0].open[0],
	// which is the first quotation of the current trading day.
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=1d&interval=1d", ticker)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return TickerPrice{}, err
	}
	// Yahoo requires a browser-like User-Agent to avoid 429/Unauthorized responses.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.http.Do(req)
	if err != nil {
		return TickerPrice{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TickerPrice{}, err
	}

	var parsed yahooChartResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return TickerPrice{}, fmt.Errorf("parse response for %s: %w", ticker, err)
	}
	if parsed.Chart.Error != nil {
		return TickerPrice{}, fmt.Errorf("yahoo error for %s: %s", ticker, parsed.Chart.Error.Description)
	}
	if len(parsed.Chart.Result) == 0 {
		return TickerPrice{}, fmt.Errorf("no data for ticker %s", ticker)
	}

	result := parsed.Chart.Result[0]
	meta := result.Meta
	var dayOpen float64
	if len(result.Indicators.Quote) > 0 && len(result.Indicators.Quote[0].Open) > 0 {
		dayOpen = result.Indicators.Quote[0].Open[0]
	}
	return TickerPrice{
		Ticker:   ticker,
		Price:    meta.RegularMarketPrice,
		Currency: meta.Currency,
		DayOpen:  dayOpen,
	}, nil
}
