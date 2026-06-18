package projection

import (
	"database/sql"

	"helloauth/internal/db"
)

// Repo handles all database operations for the projection feature.
type Repo struct {
	db *db.Database
}

func NewRepo(database *db.Database) *Repo {
	return &Repo{db: database}
}

// GetAllRates returns all stored projection rates.
func (r *Repo) GetAllRates() ([]Rate, error) {
	if !r.db.IsConnected() {
		return nil, nil
	}
	rows, err := r.db.Query(`
		SELECT key, label, rate, source_url, rate_override, updated_at
		FROM projection_rates
		ORDER BY key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rates []Rate
	for rows.Next() {
		var rate Rate
		var override sql.NullFloat64
		if err := rows.Scan(&rate.Key, &rate.Label, &rate.Rate, &rate.SourceURL, &override, &rate.UpdatedAt); err != nil {
			return nil, err
		}
		if override.Valid {
			rate.RateOverride = &override.Float64
		}
		rates = append(rates, rate)
	}
	return rates, nil
}

// UpsertRate inserts or updates a projection rate. rate_override is preserved on conflict.
func (r *Repo) UpsertRate(rate Rate) error {
	if !r.db.IsConnected() {
		return nil
	}
	_, err := r.db.Exec(`
		INSERT INTO projection_rates (key, label, rate, source_url, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (key) DO UPDATE SET label = $2, rate = $3, source_url = $4, updated_at = NOW()
	`, rate.Key, rate.Label, rate.Rate, rate.SourceURL)
	return err
}

// SetRateOverride sets or clears the rate_override for a key. Pass nil to clear.
func (r *Repo) SetRateOverride(key string, override *float64) error {
	if !r.db.IsConnected() {
		return nil
	}
	var val sql.NullFloat64
	if override != nil {
		val = sql.NullFloat64{Float64: *override, Valid: true}
	}
	_, err := r.db.Exec(`
		UPDATE projection_rates SET rate_override = $2, updated_at = NOW() WHERE key = $1
	`, key, val)
	return err
}

// SeedDefaults inserts default rates for keys that don't exist yet.
func (r *Repo) SeedDefaults() error {
	if !r.db.IsConnected() {
		return nil
	}
	for _, rate := range defaultRates {
		_, err := r.db.Exec(`
			INSERT INTO projection_rates (key, label, rate, source_url)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (key) DO NOTHING
		`, rate.Key, rate.Label, rate.Rate, rate.SourceURL)
		if err != nil {
			return err
		}
	}
	return nil
}

// HasCategoryRates reports whether any category_* rates exist in the DB.
func (r *Repo) HasCategoryRates() bool {
	if !r.db.IsConnected() {
		return false
	}
	var count int
	r.db.QueryRow(`SELECT COUNT(*) FROM projection_rates WHERE key LIKE 'category_%'`).Scan(&count)
	return count > 0
}

// GetRate returns a single rate by key.
func (r *Repo) GetRate(key string) (*Rate, error) {
	if !r.db.IsConnected() {
		return nil, nil
	}
	var rate Rate
	var override sql.NullFloat64
	err := r.db.QueryRow(`
		SELECT key, label, rate, source_url, rate_override, updated_at
		FROM projection_rates WHERE key = $1
	`, key).Scan(&rate.Key, &rate.Label, &rate.Rate, &rate.SourceURL, &override, &rate.UpdatedAt)
	if err != nil {
		return nil, nil
	}
	if override.Valid {
		rate.RateOverride = &override.Float64
	}
	return &rate, nil
}
