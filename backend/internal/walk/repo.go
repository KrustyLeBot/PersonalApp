package walk

import (
	"fmt"
	"time"

	"helloauth/internal/db"
)

// Repo handles all database operations for the walk feature.
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

// GetYear returns every recorded distance of a year, keyed by YYYY-MM-DD.
func (r *Repo) GetYear(year int, email string) (map[string]float64, error) {
	if err := r.requireDB(); err != nil {
		return map[string]float64{}, nil
	}
	rows, err := r.db.Query(
		`SELECT entry_date, km FROM walk_entries WHERE year = $1 AND user_email = $2`,
		year, email,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var d time.Time
		var km float64
		if err := rows.Scan(&d, &km); err != nil {
			return nil, err
		}
		result[d.Format("2006-01-02")] = km
	}
	return result, rows.Err()
}

// SumRange returns the total distance walked between from and to, inclusive.
func (r *Repo) SumRange(from, to string, email string) (float64, error) {
	if err := r.requireDB(); err != nil {
		return 0, nil
	}
	var total float64
	err := r.db.QueryRow(
		`SELECT COALESCE(SUM(km), 0) FROM walk_entries
		 WHERE user_email = $1 AND entry_date >= $2 AND entry_date <= $3`,
		email, from, to,
	).Scan(&total)
	return total, err
}

// SetDay upserts one day's distance. A distance of zero removes the entry.
func (r *Repo) SetDay(e Entry, email string) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	if e.Km <= 0 {
		_, err := r.db.Exec(
			`DELETE FROM walk_entries WHERE entry_date = $1 AND user_email = $2`,
			e.Date, email,
		)
		return err
	}
	d, err := time.Parse("2006-01-02", e.Date)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`
		INSERT INTO walk_entries (entry_date, year, km, user_email)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (entry_date, user_email) DO UPDATE SET km = EXCLUDED.km, updated_at = NOW()
	`, e.Date, d.Year(), e.Km, email)
	return err
}
