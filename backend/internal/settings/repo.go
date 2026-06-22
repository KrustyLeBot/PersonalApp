package settings

import (
	"database/sql"
	"encoding/json"

	"helloauth/internal/db"
)

type Repo struct {
	db *db.Database
}

func NewRepo(database *db.Database) *Repo {
	return &Repo{db: database}
}

func (r *Repo) GetFeatures(email string) ([]string, error) {
	var raw string
	err := r.db.QueryRow(
		`SELECT enabled_features FROM user_settings WHERE user_email = $1`, email,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var features []string
	if err := json.Unmarshal([]byte(raw), &features); err != nil {
		return []string{}, nil
	}
	return features, nil
}

func (r *Repo) SetFeatures(email string, features []string) error {
	raw, err := json.Marshal(features)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(`
		INSERT INTO user_settings (user_email, enabled_features)
		VALUES ($1, $2)
		ON CONFLICT (user_email) DO UPDATE SET enabled_features = EXCLUDED.enabled_features
	`, email, string(raw))
	return err
}
