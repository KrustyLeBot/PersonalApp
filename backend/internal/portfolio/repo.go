package portfolio

import (
	"fmt"
	"time"

	"helloauth/internal/db"
)

// Repo handles all database operations for the portfolio feature.
type Repo struct {
	db *db.Database
}

func NewRepo(database *db.Database) *Repo {
	return &Repo{db: database}
}

func (r *Repo) requireDB() error {
	if !r.db.IsConnected() {
		return fmt.Errorf("database not connected")
	}
	return nil
}

// --- Assets ---

func (r *Repo) GetAllAssets() ([]Asset, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT id, type, name, COALESCE(value, 0), created_at, updated_at
		FROM assets
		ORDER BY type, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets []Asset
	for rows.Next() {
		var a Asset
		if err := rows.Scan(&a.ID, &a.Type, &a.Name, &a.Value, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		assets = append(assets, a)
	}
	return assets, nil
}

func (r *Repo) CreateAsset(a Asset) (int, error) {
	if err := r.requireDB(); err != nil {
		return 0, err
	}
	var id int
	err := r.db.QueryRow(`
		INSERT INTO assets (type, name, value) VALUES ($1, $2, $3) RETURNING id
	`, a.Type, a.Name, a.Value).Scan(&id)
	return id, err
}

func (r *Repo) UpdateAsset(id int, a Asset) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		UPDATE assets SET type = $1, name = $2, value = $3, updated_at = NOW() WHERE id = $4
	`, a.Type, a.Name, a.Value, id)
	return err
}

func (r *Repo) DeleteAsset(id int) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	// ticker_holdings rows are removed automatically via ON DELETE CASCADE
	_, err := r.db.Exec(`DELETE FROM assets WHERE id = $1`, id)
	return err
}

// --- Ticker holdings (shared by bourse and crypto) ---

func (r *Repo) GetHoldingsByAsset(assetID int) ([]Holding, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT id, asset_id, ticker, shares, created_at, updated_at
		FROM ticker_holdings
		WHERE asset_id = $1
		ORDER BY ticker
	`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var holdings []Holding
	for rows.Next() {
		var h Holding
		if err := rows.Scan(&h.ID, &h.AssetID, &h.Ticker, &h.Shares, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		holdings = append(holdings, h)
	}
	return holdings, nil
}

func (r *Repo) GetAllHoldings() (map[int][]Holding, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT id, asset_id, ticker, shares, created_at, updated_at
		FROM ticker_holdings
		ORDER BY asset_id, ticker
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int][]Holding)
	for rows.Next() {
		var h Holding
		if err := rows.Scan(&h.ID, &h.AssetID, &h.Ticker, &h.Shares, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		result[h.AssetID] = append(result[h.AssetID], h)
	}
	return result, nil
}

func (r *Repo) CreateHolding(h Holding) (int, error) {
	if err := r.requireDB(); err != nil {
		return 0, err
	}
	var id int
	err := r.db.QueryRow(`
		INSERT INTO ticker_holdings (asset_id, ticker, shares) VALUES ($1, $2, $3) RETURNING id
	`, h.AssetID, h.Ticker, h.Shares).Scan(&id)
	return id, err
}

func (r *Repo) UpdateHolding(id int, h Holding) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		UPDATE ticker_holdings SET ticker = $1, shares = $2, updated_at = NOW() WHERE id = $3
	`, h.Ticker, h.Shares, id)
	return err
}

func (r *Repo) DeleteHolding(id int) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`DELETE FROM ticker_holdings WHERE id = $1`, id)
	return err
}

// --- Tickers ---

// GetDistinctTickers returns all unique tickers across bourse and crypto holdings.
func (r *Repo) GetDistinctTickers() ([]string, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`SELECT DISTINCT ticker FROM ticker_holdings WHERE ticker != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickers []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil && t != "" {
			tickers = append(tickers, t)
		}
	}
	return tickers, nil
}

func (r *Repo) SaveTickerPrice(p TickerPrice) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		INSERT INTO ticker_prices (ticker, price, currency, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (ticker) DO UPDATE SET price = $2, currency = $3, updated_at = NOW()
	`, p.Ticker, p.Price, p.Currency)
	return err
}

func (r *Repo) GetTickerPrices() (map[string]float64, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`SELECT ticker, price FROM ticker_prices`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prices := make(map[string]float64)
	for rows.Next() {
		var ticker string
		var price float64
		if err := rows.Scan(&ticker, &price); err == nil {
			prices[ticker] = price
		}
	}
	return prices, nil
}

// --- Ticker categories ---

// GetTickerCategories returns a map of ticker → category for all stored entries.
func (r *Repo) GetTickerCategories() (map[string]string, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`SELECT ticker, category FROM ticker_categories`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cats := make(map[string]string)
	for rows.Next() {
		var ticker, category string
		if err := rows.Scan(&ticker, &category); err == nil {
			cats[ticker] = category
		}
	}
	return cats, nil
}

// UpsertTickerCategory sets or updates the category for a ticker.
func (r *Repo) UpsertTickerCategory(ticker, category string) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		INSERT INTO ticker_categories (ticker, category) VALUES ($1, $2)
		ON CONFLICT (ticker) DO UPDATE SET category = $2
	`, ticker, category)
	return err
}

// DeleteTickerCategory removes the category for a ticker.
func (r *Repo) DeleteTickerCategory(ticker string) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`DELETE FROM ticker_categories WHERE ticker = $1`, ticker)
	return err
}

// --- Daily refresh ---

func (r *Repo) WasRefreshedToday() (bool, error) {
	if err := r.requireDB(); err != nil {
		return false, err
	}
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM daily_refresh WHERE refresh_date = $1`,
		time.Now().Format("2006-01-02"),
	).Scan(&count)
	return count > 0, err
}

func (r *Repo) RecordDailyRefresh() error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		INSERT INTO daily_refresh (refresh_date, refreshed_at) VALUES ($1, NOW())
		ON CONFLICT (refresh_date) DO UPDATE SET refreshed_at = NOW()
	`, time.Now().Format("2006-01-02"))
	return err
}

func (r *Repo) GetLastRefreshTime() (*string, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	var ts string
	err := r.db.QueryRow(
		`SELECT refreshed_at FROM daily_refresh ORDER BY refresh_date DESC LIMIT 1`,
	).Scan(&ts)
	if err != nil {
		return nil, nil
	}
	return &ts, nil
}
