package telework

import (
	"encoding/json"
	"fmt"
	"time"

	"helloauth/internal/db"
)

// Repo handles all database operations for the telework feature.
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

// GetPreset returns the stored weekly preset, or a default if none exists.
func (r *Repo) GetPreset() (Preset, error) {
	if err := r.requireDB(); err != nil {
		return defaultPreset(), nil
	}
	var raw string
	err := r.db.QueryRow(`SELECT remote_days FROM telework_preset LIMIT 1`).Scan(&raw)
	if err != nil {
		return defaultPreset(), nil
	}
	var p Preset
	if err := json.Unmarshal([]byte(raw), &p.RemoteDays); err != nil {
		return defaultPreset(), nil
	}
	return p, nil
}

// SavePreset upserts the global preset.
func (r *Repo) SavePreset(p Preset) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	raw, err := json.Marshal(p.RemoteDays)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`
		INSERT INTO telework_preset (id, remote_days) VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET remote_days = $1
	`, string(raw))
	return err
}

// GetOverrides returns all per-day overrides for a given year.
func (r *Repo) GetOverrides(year int) (map[string]string, error) {
	if err := r.requireDB(); err != nil {
		return map[string]string{}, nil
	}
	rows, err := r.db.Query(
		`SELECT override_date, type FROM telework_overrides WHERE year = $1`,
		year,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var d time.Time
		var t string
		if err := rows.Scan(&d, &t); err != nil {
			return nil, err
		}
		result[d.Format("2006-01-02")] = t
	}
	return result, nil
}

// BulkSetOverrides replaces all overrides for a year with the provided set.
// Each entry in overrides maps a YYYY-MM-DD date to a type ("leave","remote","office").
func (r *Repo) BulkSetOverrides(year int, overrides map[string]string) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM telework_overrides WHERE year = $1`, year); err != nil {
		return err
	}
	for date, typ := range overrides {
		if _, err := tx.Exec(
			`INSERT INTO telework_overrides (override_date, year, type) VALUES ($1, $2, $3)
			 ON CONFLICT (override_date) DO UPDATE SET type = $3`,
			date, year, typ,
		); err != nil {
			return err
		}
	}

	// Keep telework_leaves in sync for backward compatibility.
	if _, err := tx.Exec(`DELETE FROM telework_leaves WHERE year = $1`, year); err != nil {
		return err
	}
	for date, typ := range overrides {
		if typ == "leave" {
			if _, err := tx.Exec(
				`INSERT INTO telework_leaves (leave_date, year) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				date, year,
			); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func defaultPreset() Preset {
	// Default: Thursday (4) and Friday (5) are remote
	return Preset{RemoteDays: []int{4, 5}}
}
