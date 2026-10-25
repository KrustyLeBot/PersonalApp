package export

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"helloauth/internal/db"
)

// Repo reads raw user data for export. It is read-only.
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

func (r *Repo) GetPortfolio(email string) (Portfolio, error) {
	if err := r.requireDB(); err != nil {
		return Portfolio{}, err
	}
	assets, err := r.assets(email)
	if err != nil {
		return Portfolio{}, err
	}
	cats, err := r.tickerCategories(email)
	if err != nil {
		return Portfolio{}, err
	}
	rates, err := r.projectionRates()
	if err != nil {
		return Portfolio{}, err
	}
	return Portfolio{Assets: assets, TickerCategories: cats, ProjectionRates: rates}, nil
}

func (r *Repo) assets(email string) ([]Asset, error) {
	rows, err := r.db.Query(`
		SELECT id, type, name, COALESCE(value, 0), created_at, updated_at
		FROM assets WHERE user_email = $1 ORDER BY id
	`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assets := []Asset{}
	index := map[int]int{}
	for rows.Next() {
		a := Asset{Holdings: []Holding{}}
		var created, updated time.Time
		if err := rows.Scan(&a.ID, &a.Type, &a.Name, &a.Value, &created, &updated); err != nil {
			return nil, err
		}
		a.CreatedAt = created.UTC().Format(time.RFC3339)
		a.UpdatedAt = updated.UTC().Format(time.RFC3339)
		index[a.ID] = len(assets)
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := r.attachHoldings(email, assets, index); err != nil {
		return nil, err
	}
	if err := r.attachDettes(email, assets, index); err != nil {
		return nil, err
	}
	return assets, nil
}

func (r *Repo) attachHoldings(email string, assets []Asset, index map[int]int) error {
	rows, err := r.db.Query(`
		SELECT th.asset_id, th.ticker, th.shares
		FROM ticker_holdings th
		JOIN assets a ON a.id = th.asset_id
		WHERE a.user_email = $1
		ORDER BY th.asset_id, th.ticker
	`, email)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var assetID int
		var h Holding
		if err := rows.Scan(&assetID, &h.Ticker, &h.Shares); err != nil {
			return err
		}
		i := index[assetID]
		assets[i].Holdings = append(assets[i].Holdings, h)
	}
	return rows.Err()
}

func (r *Repo) attachDettes(email string, assets []Asset, index map[int]int) error {
	rows, err := r.db.Query(`
		SELECT da.asset_id, da.start_date, da.duration_months, da.taeg, da.amount_borrowed
		FROM dette_assets da
		JOIN assets a ON a.id = da.asset_id
		WHERE a.user_email = $1
	`, email)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var assetID int
		var d Dette
		var start time.Time
		if err := rows.Scan(&assetID, &start, &d.DurationMonths, &d.TAEG, &d.AmountBorrowed); err != nil {
			return err
		}
		d.StartDate = start.Format("2006-01-02")
		assets[index[assetID]].Dette = &d
	}
	return rows.Err()
}

func (r *Repo) tickerCategories(email string) (map[string]string, error) {
	rows, err := r.db.Query(`SELECT ticker, category FROM ticker_categories WHERE user_email = $1`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cats := map[string]string{}
	for rows.Next() {
		var ticker, category string
		if err := rows.Scan(&ticker, &category); err != nil {
			return nil, err
		}
		cats[ticker] = category
	}
	return cats, rows.Err()
}

// projectionRates reads the global projection_rates table: it is not scoped per
// user, but it holds the user's overrides and per-asset rates.
func (r *Repo) projectionRates() ([]ProjectionRate, error) {
	rows, err := r.db.Query(`
		SELECT key, label, rate, source_url, rate_override
		FROM projection_rates ORDER BY key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rates := []ProjectionRate{}
	for rows.Next() {
		var pr ProjectionRate
		var override sql.NullFloat64
		if err := rows.Scan(&pr.Key, &pr.Label, &pr.Rate, &pr.SourceURL, &override); err != nil {
			return nil, err
		}
		if override.Valid {
			pr.RateOverride = &override.Float64
		}
		rates = append(rates, pr)
	}
	return rates, rows.Err()
}

func (r *Repo) GetTelework(email string) (Telework, error) {
	if err := r.requireDB(); err != nil {
		return Telework{}, err
	}
	tw := Telework{RemoteDays: []int{4, 5}, Overrides: map[string]TeleworkChoice{}}

	var raw string
	err := r.db.QueryRow(`SELECT remote_days FROM telework_preset WHERE user_email = $1`, email).Scan(&raw)
	switch {
	case err == sql.ErrNoRows:
	case err != nil:
		return Telework{}, err
	default:
		if err := json.Unmarshal([]byte(raw), &tw.RemoteDays); err != nil {
			return Telework{}, err
		}
	}

	rows, err := r.db.Query(
		`SELECT override_date, am_type, pm_type FROM telework_overrides WHERE user_email = $1 ORDER BY override_date`,
		email,
	)
	if err != nil {
		return Telework{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var d time.Time
		var c TeleworkChoice
		if err := rows.Scan(&d, &c.AM, &c.PM); err != nil {
			return Telework{}, err
		}
		tw.Overrides[d.Format("2006-01-02")] = c
	}
	return tw, rows.Err()
}

func (r *Repo) GetWalk(email string) (Walk, error) {
	if err := r.requireDB(); err != nil {
		return Walk{}, err
	}
	rows, err := r.db.Query(
		`SELECT entry_date, km FROM walk_entries WHERE user_email = $1 ORDER BY entry_date`, email,
	)
	if err != nil {
		return Walk{}, err
	}
	defer rows.Close()

	w := Walk{Entries: map[string]float64{}}
	for rows.Next() {
		var d time.Time
		var km float64
		if err := rows.Scan(&d, &km); err != nil {
			return Walk{}, err
		}
		w.Entries[d.Format("2006-01-02")] = km
	}
	return w, rows.Err()
}
