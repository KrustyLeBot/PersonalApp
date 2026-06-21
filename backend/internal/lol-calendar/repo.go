package lolcalendar

import (
	"time"

	"helloauth/internal/db"
)

type Repo struct {
	db *db.Database
}

func NewRepo(database *db.Database) *Repo {
	return &Repo{db: database}
}

// --- League config ---

func (r *Repo) SeedLeagues() error {
	for _, l := range defaultLeagues {
		_, err := r.db.Exec(`
			INSERT INTO lol_leagues (slug, name, league_id, enabled)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (slug) DO NOTHING
		`, l.Slug, l.Name, l.LeagueID, l.Enabled)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) GetLeagues() ([]League, error) {
	rows, err := r.db.Query(`SELECT slug, name, league_id, COALESCE(region,''), COALESCE(image_url,''), enabled FROM lol_leagues ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leagues []League
	for rows.Next() {
		var l League
		if err := rows.Scan(&l.Slug, &l.Name, &l.LeagueID, &l.Region, &l.ImageURL, &l.Enabled); err != nil {
			return nil, err
		}
		leagues = append(leagues, l)
	}
	return leagues, rows.Err()
}

// UpsertLeague inserts or updates a league entry (used when activating a league not yet in DB).
func (r *Repo) UpsertLeague(l League) error {
	_, err := r.db.Exec(`
		INSERT INTO lol_leagues (slug, name, league_id, region, image_url, enabled)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (slug) DO UPDATE SET
			name      = EXCLUDED.name,
			league_id = EXCLUDED.league_id,
			region    = EXCLUDED.region,
			image_url = EXCLUDED.image_url,
			enabled   = EXCLUDED.enabled
	`, l.Slug, l.Name, l.LeagueID, l.Region, l.ImageURL, l.Enabled)
	return err
}

func (r *Repo) SetLeagueEnabled(slug string, enabled bool) error {
	_, err := r.db.Exec(`UPDATE lol_leagues SET enabled = $1 WHERE slug = $2`, enabled, slug)
	return err
}

func (r *Repo) GetEnabledLeagueIDs() ([]string, error) {
	rows, err := r.db.Query(`SELECT league_id FROM lol_leagues WHERE enabled = TRUE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// --- Matches ---

func (r *Repo) Upsert(matches []Match) error {
	for _, m := range matches {
		_, err := r.db.Exec(`
			INSERT INTO lol_matches (
				match_id, league_name, league_slug,
				team1_name, team1_code, team1_image, team1_wins, team1_outcome,
				team2_name, team2_code, team2_image, team2_wins, team2_outcome,
				scheduled_at, stage, best_of, state, is_spoiler, fetched_at
			) VALUES (
				$1, $2, $3,
				$4, $5, $6, $7, $8,
				$9, $10, $11, $12, $13,
				$14, $15, $16, $17, $18, $19
			)
			ON CONFLICT (match_id) DO UPDATE SET
				state         = EXCLUDED.state,
				team1_wins    = EXCLUDED.team1_wins,
				team1_outcome = EXCLUDED.team1_outcome,
				team2_wins    = EXCLUDED.team2_wins,
				team2_outcome = EXCLUDED.team2_outcome,
				is_spoiler    = EXCLUDED.is_spoiler,
				fetched_at    = EXCLUDED.fetched_at
		`,
			m.MatchID, m.LeagueName, m.LeagueSlug,
			m.Team1.Name, m.Team1.Code, m.Team1.ImageURL, m.Team1.GameWins, m.Team1.Outcome,
			m.Team2.Name, m.Team2.Code, m.Team2.ImageURL, m.Team2.GameWins, m.Team2.Outcome,
			m.ScheduledAt, m.Stage, m.BestOf, m.State, m.IsSpoiler, m.FetchedAt,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) GetSchedule(pastDays int) ([]Match, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -pastDays)
	rows, err := r.db.Query(`
		SELECT
			match_id, league_name, league_slug,
			team1_name, team1_code, team1_image, team1_wins, team1_outcome,
			team2_name, team2_code, team2_image, team2_wins, team2_outcome,
			scheduled_at, stage, best_of, state, is_spoiler, spoiler_dismissed, fetched_at
		FROM lol_matches
		WHERE scheduled_at >= $1
		ORDER BY scheduled_at ASC
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []Match
	for rows.Next() {
		var m Match
		err := rows.Scan(
			&m.MatchID, &m.LeagueName, &m.LeagueSlug,
			&m.Team1.Name, &m.Team1.Code, &m.Team1.ImageURL, &m.Team1.GameWins, &m.Team1.Outcome,
			&m.Team2.Name, &m.Team2.Code, &m.Team2.ImageURL, &m.Team2.GameWins, &m.Team2.Outcome,
			&m.ScheduledAt, &m.Stage, &m.BestOf, &m.State, &m.IsSpoiler, &m.SpoilerDismissed, &m.FetchedAt,
		)
		if err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

// GetLiveWindow returns matches scheduled within ±window of now, plus any inProgress matches.
func (r *Repo) GetLiveWindow(window time.Duration) ([]Match, error) {
	now := time.Now().UTC()
	from := now.Add(-window)
	to := now.Add(window)
	rows, err := r.db.Query(`
		SELECT
			match_id, league_name, league_slug,
			team1_name, team1_code, team1_image, team1_wins, team1_outcome,
			team2_name, team2_code, team2_image, team2_wins, team2_outcome,
			scheduled_at, stage, best_of, state, is_spoiler, spoiler_dismissed, fetched_at
		FROM lol_matches
		WHERE state = 'inProgress' OR (scheduled_at >= $1 AND scheduled_at <= $2)
		ORDER BY scheduled_at ASC
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []Match
	for rows.Next() {
		var m Match
		err := rows.Scan(
			&m.MatchID, &m.LeagueName, &m.LeagueSlug,
			&m.Team1.Name, &m.Team1.Code, &m.Team1.ImageURL, &m.Team1.GameWins, &m.Team1.Outcome,
			&m.Team2.Name, &m.Team2.Code, &m.Team2.ImageURL, &m.Team2.GameWins, &m.Team2.Outcome,
			&m.ScheduledAt, &m.Stage, &m.BestOf, &m.State, &m.IsSpoiler, &m.SpoilerDismissed, &m.FetchedAt,
		)
		if err != nil {
			return nil, err
		}
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

func (r *Repo) DismissSpoiler(matchID string) error {
	_, err := r.db.Exec(`UPDATE lol_matches SET spoiler_dismissed = TRUE WHERE match_id = $1`, matchID)
	return err
}

// --- Daily refresh ---

func (r *Repo) WasRefreshedToday() (bool, error) {
	today := time.Now().UTC().Format("2006-01-02")
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM lol_daily_refresh WHERE refresh_date = $1`, today).Scan(&count)
	return count > 0, err
}

func (r *Repo) GetLastRefreshTime() *string {
	var ts string
	err := r.db.QueryRow(`SELECT refreshed_at FROM lol_daily_refresh ORDER BY refresh_date DESC LIMIT 1`).Scan(&ts)
	if err != nil {
		return nil
	}
	return &ts
}

func (r *Repo) RecordDailyRefresh() error {
	today := time.Now().UTC().Format("2006-01-02")
	_, err := r.db.Exec(`
		INSERT INTO lol_daily_refresh (refresh_date, refreshed_at)
		VALUES ($1, NOW())
		ON CONFLICT (refresh_date) DO UPDATE SET refreshed_at = NOW()
	`, today)
	return err
}
