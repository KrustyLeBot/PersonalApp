package projection

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// CAGRClient fetches historical price data from Yahoo Finance to compute CAGR.
type CAGRClient struct {
	http *http.Client
}

func NewCAGRClient() *CAGRClient {
	return &CAGRClient{
		http: &http.Client{Timeout: 20 * time.Second},
	}
}

// FetchCAGR fetches the maximum available historical data for a ticker and
// returns the annualised compound growth rate as a percentage (e.g. 7.5 for 7.5%).
// Returns an error if fewer than 1 year of data is available.
func (c *CAGRClient) FetchCAGR(ticker string) (float64, int, error) {
	// Use "max" range with monthly granularity to minimise payload size.
	url := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v8/finance/chart/%s?range=max&interval=1mo",
		ticker,
	)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, err
	}

	var parsed yahooHistoricalResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, 0, fmt.Errorf("parse historical for %s: %w", ticker, err)
	}
	if parsed.Chart.Error != nil {
		return 0, 0, fmt.Errorf("yahoo error for %s: %s", ticker, parsed.Chart.Error.Description)
	}
	if len(parsed.Chart.Result) == 0 {
		return 0, 0, fmt.Errorf("no historical data for %s", ticker)
	}

	result := parsed.Chart.Result[0]
	closes := result.Indicators.Quote[0].Close
	timestamps := result.Timestamps

	if len(timestamps) < 2 || len(closes) < 2 {
		return 0, 0, fmt.Errorf("insufficient data for %s", ticker)
	}

	// Find first and last non-zero closes.
	firstIdx, lastIdx := -1, -1
	for i, v := range closes {
		if v > 0 {
			if firstIdx == -1 {
				firstIdx = i
			}
			lastIdx = i
		}
	}
	if firstIdx == -1 || firstIdx == lastIdx {
		return 0, 0, fmt.Errorf("no valid prices for %s", ticker)
	}

	startPrice := closes[firstIdx]
	endPrice := closes[lastIdx]
	startTime := time.Unix(int64(timestamps[firstIdx]), 0)
	endTime := time.Unix(int64(timestamps[lastIdx]), 0)

	years := endTime.Sub(startTime).Hours() / 8760.0
	if years < 1 {
		return 0, 0, fmt.Errorf("less than 1 year of data for %s", ticker)
	}

	cagr := (math.Pow(endPrice/startPrice, 1/years) - 1) * 100
	yearsInt := int(math.Round(years))
	return cagr, yearsInt, nil
}

type yahooHistoricalResponse struct {
	Chart struct {
		Result []struct {
			Timestamps []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}
