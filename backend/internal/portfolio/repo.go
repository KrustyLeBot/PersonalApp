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

func (r *Repo) GetAllAssets(email string) ([]Asset, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT id, type, name, COALESCE(value, 0), created_at, updated_at
		FROM assets
		WHERE user_email = $1
		ORDER BY type, name
	`, email)
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

func (r *Repo) CreateAsset(a Asset, email string) (int, error) {
	if err := r.requireDB(); err != nil {
		return 0, err
	}
	var id int
	err := r.db.QueryRow(`
		INSERT INTO assets (type, name, value, user_email) VALUES ($1, $2, $3, $4) RETURNING id
	`, a.Type, a.Name, a.Value, email).Scan(&id)
	return id, err
}

func (r *Repo) UpdateAsset(id int, a Asset, email string) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		UPDATE assets SET type = $1, name = $2, value = $3, updated_at = NOW()
		WHERE id = $4 AND user_email = $5
	`, a.Type, a.Name, a.Value, id, email)
	return err
}

func (r *Repo) DeleteAsset(id int, email string) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	// ticker_holdings rows are removed automatically via ON DELETE CASCADE
	_, err := r.db.Exec(`DELETE FROM assets WHERE id = $1 AND user_email = $2`, id, email)
	return err
}

// --- Ticker holdings (shared by bourse and crypto) ---

func (r *Repo) GetHoldingsByAsset(assetID int, email string) ([]Holding, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT th.id, th.asset_id, th.ticker, th.shares, th.created_at, th.updated_at
		FROM ticker_holdings th
		JOIN assets a ON a.id = th.asset_id
		WHERE th.asset_id = $1 AND a.user_email = $2
		ORDER BY th.ticker
	`, assetID, email)
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

func (r *Repo) GetAllHoldings(email string) (map[int][]Holding, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT th.id, th.asset_id, th.ticker, th.shares, th.created_at, th.updated_at
		FROM ticker_holdings th
		JOIN assets a ON a.id = th.asset_id
		WHERE a.user_email = $1
		ORDER BY th.asset_id, th.ticker
	`, email)
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

func (r *Repo) UpdateHolding(id int, h Holding, email string) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		UPDATE ticker_holdings th SET ticker = $1, shares = $2, updated_at = NOW()
		FROM assets a
		WHERE th.id = $3 AND th.asset_id = a.id AND a.user_email = $4
	`, h.Ticker, h.Shares, id, email)
	return err
}

func (r *Repo) DeleteHolding(id int, email string) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		DELETE FROM ticker_holdings th
		USING assets a
		WHERE th.id = $1 AND th.asset_id = a.id AND a.user_email = $2
	`, id, email)
	return err
}

// --- Tickers ---

// GetDistinctBourseTickers returns unique tickers belonging to bourse assets only.
func (r *Repo) GetDistinctBourseTickers() ([]string, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`
		SELECT DISTINCT th.ticker
		FROM ticker_holdings th
		JOIN assets a ON a.id = th.asset_id
		WHERE a.type = 'bourse' AND th.ticker != ''
	`)
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

// GetTickerCategories returns a map of ticker → category for the given user.
func (r *Repo) GetTickerCategories(email string) (map[string]string, error) {
	if err := r.requireDB(); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`SELECT ticker, category FROM ticker_categories WHERE user_email = $1`, email)
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

// UpsertTickerCategory sets or updates the category for a ticker for the given user.
func (r *Repo) UpsertTickerCategory(ticker, category, email string) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`
		INSERT INTO ticker_categories (ticker, category, user_email) VALUES ($1, $2, $3)
		ON CONFLICT (ticker, user_email) DO UPDATE SET category = $2
	`, ticker, category, email)
	return err
}

// DeleteTickerCategory removes the category for a ticker for the given user.
func (r *Repo) DeleteTickerCategory(ticker, email string) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	_, err := r.db.Exec(`DELETE FROM ticker_categories WHERE ticker = $1 AND user_email = $2`, ticker, email)
	return err
}

// --- Daily refresh ---

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
