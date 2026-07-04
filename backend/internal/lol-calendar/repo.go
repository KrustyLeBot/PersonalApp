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

func (r *Repo) GetLeagues(email string) ([]League, error) {
	rows, err := r.db.Query(`
		SELECT slug, name, league_id, COALESCE(region,''), COALESCE(image_url,''), enabled
		FROM lol_leagues WHERE user_email = $1 ORDER BY name
	`, email)
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

// UpsertLeague inserts or updates a league entry for a user.
func (r *Repo) UpsertLeague(l League, email string) error {
	_, err := r.db.Exec(`
		INSERT INTO lol_leagues (slug, name, league_id, region, image_url, enabled, user_email)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (slug, user_email) DO UPDATE SET
			name      = EXCLUDED.name,
			league_id = EXCLUDED.league_id,
			region    = EXCLUDED.region,
			image_url = EXCLUDED.image_url,
			enabled   = EXCLUDED.enabled
	`, l.Slug, l.Name, l.LeagueID, l.Region, l.ImageURL, l.Enabled, email)
	return err
}

func (r *Repo) GetEnabledLeagueIDs(email string) ([]string, error) {
	rows, err := r.db.Query(`SELECT league_id FROM lol_leagues WHERE enabled = TRUE AND user_email = $1`, email)
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

func (r *Repo) Upsert(matches []Match, email string) error {
	for _, m := range matches {
		_, err := r.db.Exec(`
			INSERT INTO lol_matches (
				match_id, user_email, league_name, league_slug,
				team1_name, team1_code, team1_image, team1_wins, team1_outcome,
				team2_name, team2_code, team2_image, team2_wins, team2_outcome,
				scheduled_at, stage, best_of, state, is_spoiler, fetched_at
			) VALUES (
				$1, $2, $3, $4,
				$5, $6, $7, $8, $9,
				$10, $11, $12, $13, $14,
				$15, $16, $17, $18, $19, $20
			)
			ON CONFLICT (match_id, user_email) DO UPDATE SET
				league_name   = EXCLUDED.league_name,
				league_slug   = EXCLUDED.league_slug,
				scheduled_at  = EXCLUDED.scheduled_at,
				stage         = EXCLUDED.stage,
				best_of       = EXCLUDED.best_of,
				state         = EXCLUDED.state,
				team1_name    = EXCLUDED.team1_name,
				team1_code    = EXCLUDED.team1_code,
				team1_image   = EXCLUDED.team1_image,
				team1_wins    = EXCLUDED.team1_wins,
				team1_outcome = EXCLUDED.team1_outcome,
				team2_name    = EXCLUDED.team2_name,
				team2_code    = EXCLUDED.team2_code,
				team2_image   = EXCLUDED.team2_image,
				team2_wins    = EXCLUDED.team2_wins,
				team2_outcome = EXCLUDED.team2_outcome,
				is_spoiler    = EXCLUDED.is_spoiler,
				fetched_at    = EXCLUDED.fetched_at
		`,
			m.MatchID, email, m.LeagueName, m.LeagueSlug,
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

func (r *Repo) GetSchedule(pastDays int, email string) ([]Match, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -pastDays)
	rows, err := r.db.Query(`
		SELECT
			match_id, league_name, league_slug,
			team1_name, team1_code, team1_image, team1_wins, team1_outcome,
			team2_name, team2_code, team2_image, team2_wins, team2_outcome,
			scheduled_at, stage, best_of, state, is_spoiler, spoiler_dismissed, fetched_at
		FROM lol_matches
		WHERE scheduled_at >= $1 AND user_email = $2
		ORDER BY scheduled_at ASC
	`, cutoff, email)
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
func (r *Repo) GetLiveWindow(window time.Duration, email string) ([]Match, error) {
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
		WHERE user_email = $1 AND (state = 'inProgress' OR (scheduled_at >= $2 AND scheduled_at <= $3))
		ORDER BY scheduled_at ASC
	`, email, from, to)
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

func (r *Repo) DismissSpoiler(matchID, email string) error {
	_, err := r.db.Exec(`UPDATE lol_matches SET spoiler_dismissed = TRUE WHERE match_id = $1 AND user_email = $2`, matchID, email)
	return err
}

// --- Daily refresh ---

func (r *Repo) GetLastRefreshTime(email string) *string {
	var ts string
	err := r.db.QueryRow(`SELECT refreshed_at FROM lol_daily_refresh WHERE user_email = $1 ORDER BY refresh_date DESC LIMIT 1`, email).Scan(&ts)
	if err != nil {
		return nil
	}
	return &ts
}

func (r *Repo) RecordDailyRefresh(email string) error {
	today := time.Now().UTC().Format("2006-01-02")
	_, err := r.db.Exec(`
		INSERT INTO lol_daily_refresh (refresh_date, user_email, refreshed_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (refresh_date, user_email) DO UPDATE SET refreshed_at = NOW()
	`, today, email)
	return err
}
