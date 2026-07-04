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

// GetPreset returns the stored weekly preset for the given user, or a default if none exists.
func (r *Repo) GetPreset(email string) (Preset, error) {
	if err := r.requireDB(); err != nil {
		return defaultPreset(), nil
	}
	var raw string
	err := r.db.QueryRow(`SELECT remote_days FROM telework_preset WHERE user_email = $1`, email).Scan(&raw)
	if err != nil {
		return defaultPreset(), nil
	}
	var p Preset
	if err := json.Unmarshal([]byte(raw), &p.RemoteDays); err != nil {
		return defaultPreset(), nil
	}
	return p, nil
}

// SavePreset upserts the preset for the given user.
func (r *Repo) SavePreset(p Preset, email string) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	raw, err := json.Marshal(p.RemoteDays)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`
		INSERT INTO telework_preset (user_email, remote_days) VALUES ($1, $2)
		ON CONFLICT (user_email) DO UPDATE SET remote_days = $2
	`, email, string(raw))
	return err
}

// GetOverrides returns all per-day overrides for a given year and user,
// keyed by YYYY-MM-DD, each holding the AM and PM half-day types.
func (r *Repo) GetOverrides(year int, email string) (map[string]Override, error) {
	if err := r.requireDB(); err != nil {
		return map[string]Override{}, nil
	}
	rows, err := r.db.Query(
		`SELECT override_date, am_type, pm_type FROM telework_overrides WHERE year = $1 AND user_email = $2`,
		year, email,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]Override)
	for rows.Next() {
		var d time.Time
		var am, pm string
		if err := rows.Scan(&d, &am, &pm); err != nil {
			return nil, err
		}
		date := d.Format("2006-01-02")
		result[date] = Override{Date: date, AM: am, PM: pm}
	}
	return result, nil
}

// BulkSetOverrides replaces all overrides for a year for the given user.
// Each entry maps a YYYY-MM-DD date to its AM/PM half-day types. An entry where
// both halves are empty is skipped (means "follow preset").
func (r *Repo) BulkSetOverrides(year int, overrides map[string]Override, email string) error {
	if err := r.requireDB(); err != nil {
		return err
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM telework_overrides WHERE year = $1 AND user_email = $2`, year, email); err != nil {
		return err
	}
	for date, ov := range overrides {
		if ov.AM == "" && ov.PM == "" {
			continue
		}
		// Legacy 'type' column stays NOT NULL: use AM, falling back to PM.
		legacy := ov.AM
		if legacy == "" {
			legacy = ov.PM
		}
		if _, err := tx.Exec(
			`INSERT INTO telework_overrides (override_date, year, type, am_type, pm_type, user_email)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (override_date, user_email) DO UPDATE SET type = $3, am_type = $4, pm_type = $5`,
			date, year, legacy, ov.AM, ov.PM, email,
		); err != nil {
			return err
		}
	}

	// Keep telework_leaves in sync for backward compatibility: a date is a "leave"
	// only if the whole day is leave.
	if _, err := tx.Exec(`DELETE FROM telework_leaves WHERE year = $1 AND user_email = $2`, year, email); err != nil {
		return err
	}
	for date, ov := range overrides {
		if ov.AM == "leave" && ov.PM == "leave" {
			if _, err := tx.Exec(
				`INSERT INTO telework_leaves (leave_date, year, user_email) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
				date, year, email,
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
