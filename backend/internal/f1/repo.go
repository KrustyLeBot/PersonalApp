package f1

import (
	"database/sql"
	"time"

	"helloauth/internal/db"
)

type Repo struct {
	db *db.Database
}

func NewRepo(database *db.Database) *Repo {
	return &Repo{db: database}
}

// --- Races ---

func (r *Repo) UpsertRaces(races []Race) error {
	for _, race := range races {
		_, err := r.db.Exec(`
			INSERT INTO f1_races (season, round, race_name, circuit_id, circuit_name, locality, country, race_date, race_time, fetched_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
			ON CONFLICT (season, round) DO UPDATE SET
				race_name    = EXCLUDED.race_name,
				circuit_id   = EXCLUDED.circuit_id,
				circuit_name = EXCLUDED.circuit_name,
				locality     = EXCLUDED.locality,
				country      = EXCLUDED.country,
				race_date    = EXCLUDED.race_date,
				race_time    = EXCLUDED.race_time,
				fetched_at   = EXCLUDED.fetched_at
		`, race.Season, race.Round, race.RaceName, race.CircuitID, race.CircuitName,
			race.Locality, race.Country, race.RaceDate, nullableTime(race.RaceTime))
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) GetRaces(season int) ([]Race, error) {
	rows, err := r.db.Query(`
		SELECT season, round, race_name, circuit_id, circuit_name, locality, country,
		       to_char(race_date, 'YYYY-MM-DD'), COALESCE(race_time::text, ''), fetched_at
		FROM f1_races
		WHERE season = $1
		ORDER BY round ASC
	`, season)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	today := time.Now().UTC().Format("2006-01-02")
	var races []Race
	for rows.Next() {
		var race Race
		var fetchedAt time.Time
		if err := rows.Scan(
			&race.Season, &race.Round, &race.RaceName, &race.CircuitID, &race.CircuitName,
			&race.Locality, &race.Country, &race.RaceDate, &race.RaceTime, &fetchedAt,
		); err != nil {
			return nil, err
		}
		race.IsPast = race.RaceDate < today
		race.FetchedAt = fetchedAt
		races = append(races, race)
	}
	return races, rows.Err()
}

// RoundsWithoutResults returns rounds where race_date < today but no results exist yet.
func (r *Repo) RoundsWithoutResults(season int) ([]int, error) {
	today := time.Now().UTC().Format("2006-01-02")
	rows, err := r.db.Query(`
		SELECT r.round
		FROM f1_races r
		WHERE r.season = $1
		  AND r.race_date < $2
		  AND NOT EXISTS (
		        SELECT 1 FROM f1_race_results rr
		        WHERE rr.season = r.season AND rr.round = r.round
		  )
		ORDER BY r.round ASC
	`, season, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rounds []int
	for rows.Next() {
		var round int
		if err := rows.Scan(&round); err != nil {
			return nil, err
		}
		rounds = append(rounds, round)
	}
	return rounds, rows.Err()
}

// --- Race Results ---

func (r *Repo) UpsertRaceResults(results []RaceResult) error {
	for _, res := range results {
		_, err := r.db.Exec(`
			INSERT INTO f1_race_results (
				season, round, position, driver_id, driver_code,
				driver_given_name, driver_family_name,
				constructor_id, constructor_name,
				grid, laps, points, status,
				fastest_lap_rank, fastest_lap_time, fetched_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW()
			)
			ON CONFLICT (season, round, driver_id) DO UPDATE SET
				position          = EXCLUDED.position,
				driver_code       = EXCLUDED.driver_code,
				driver_given_name = EXCLUDED.driver_given_name,
				driver_family_name = EXCLUDED.driver_family_name,
				constructor_id    = EXCLUDED.constructor_id,
				constructor_name  = EXCLUDED.constructor_name,
				grid              = EXCLUDED.grid,
				laps              = EXCLUDED.laps,
				points            = EXCLUDED.points,
				status            = EXCLUDED.status,
				fastest_lap_rank  = EXCLUDED.fastest_lap_rank,
				fastest_lap_time  = EXCLUDED.fastest_lap_time,
				fetched_at        = EXCLUDED.fetched_at
		`,
			res.Season, res.Round, res.Position, res.DriverID, res.DriverCode,
			res.DriverGivenName, res.DriverFamilyName,
			res.ConstructorID, res.ConstructorName,
			res.Grid, res.Laps, res.Points, res.Status,
			res.FastestLapRank, nullableString(res.FastestLapTime),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) GetRaceResults(season, round int) ([]RaceResult, error) {
	rows, err := r.db.Query(`
		SELECT season, round, position, driver_id, driver_code,
		       driver_given_name, driver_family_name,
		       constructor_id, constructor_name,
		       grid, laps, points, status,
		       COALESCE(fastest_lap_rank, 0), COALESCE(fastest_lap_time, '')
		FROM f1_race_results
		WHERE season = $1 AND round = $2
		ORDER BY position ASC
	`, season, round)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RaceResult
	for rows.Next() {
		var res RaceResult
		if err := rows.Scan(
			&res.Season, &res.Round, &res.Position, &res.DriverID, &res.DriverCode,
			&res.DriverGivenName, &res.DriverFamilyName,
			&res.ConstructorID, &res.ConstructorName,
			&res.Grid, &res.Laps, &res.Points, &res.Status,
			&res.FastestLapRank, &res.FastestLapTime,
		); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, rows.Err()
}

// --- Qualifying Results ---

func (r *Repo) UpsertQualifyingResults(results []QualifyingResult) error {
	for _, res := range results {
		_, err := r.db.Exec(`
			INSERT INTO f1_qualifying_results (
				season, round, position, driver_id, driver_code,
				driver_given_name, driver_family_name,
				constructor_id, constructor_name,
				q1, q2, q3, fetched_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW())
			ON CONFLICT (season, round, driver_id) DO UPDATE SET
				position           = EXCLUDED.position,
				driver_code        = EXCLUDED.driver_code,
				driver_given_name  = EXCLUDED.driver_given_name,
				driver_family_name = EXCLUDED.driver_family_name,
				constructor_id     = EXCLUDED.constructor_id,
				constructor_name   = EXCLUDED.constructor_name,
				q1                 = EXCLUDED.q1,
				q2                 = EXCLUDED.q2,
				q3                 = EXCLUDED.q3,
				fetched_at         = EXCLUDED.fetched_at
		`,
			res.Season, res.Round, res.Position, res.DriverID, res.DriverCode,
			res.DriverGivenName, res.DriverFamilyName,
			res.ConstructorID, res.ConstructorName,
			nullableString(res.Q1), nullableString(res.Q2), nullableString(res.Q3),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) GetQualifyingResults(season, round int) ([]QualifyingResult, error) {
	rows, err := r.db.Query(`
		SELECT season, round, position, driver_id, driver_code,
		       driver_given_name, driver_family_name,
		       constructor_id, constructor_name,
		       COALESCE(q1, ''), COALESCE(q2, ''), COALESCE(q3, '')
		FROM f1_qualifying_results
		WHERE season = $1 AND round = $2
		ORDER BY position ASC
	`, season, round)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []QualifyingResult
	for rows.Next() {
		var res QualifyingResult
		if err := rows.Scan(
			&res.Season, &res.Round, &res.Position, &res.DriverID, &res.DriverCode,
			&res.DriverGivenName, &res.DriverFamilyName,
			&res.ConstructorID, &res.ConstructorName,
			&res.Q1, &res.Q2, &res.Q3,
		); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, rows.Err()
}

// RoundsWithoutQualifying returns past rounds that have no qualifying data stored.
func (r *Repo) RoundsWithoutQualifying(season int) ([]int, error) {
	today := time.Now().UTC().Format("2006-01-02")
	rows, err := r.db.Query(`
		SELECT r.round
		FROM f1_races r
		WHERE r.season = $1
		  AND r.race_date < $2
		  AND NOT EXISTS (
		        SELECT 1 FROM f1_qualifying_results qr
		        WHERE qr.season = r.season AND qr.round = r.round
		  )
		ORDER BY r.round ASC
	`, season, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rounds []int
	for rows.Next() {
		var round int
		if err := rows.Scan(&round); err != nil {
			return nil, err
		}
		rounds = append(rounds, round)
	}
	return rounds, rows.Err()
}

// --- Driver Standings ---

func (r *Repo) UpsertDriverStandings(season int, standings []DriverStanding) error {
	_, err := r.db.Exec(`DELETE FROM f1_driver_standings WHERE season = $1`, season)
	if err != nil {
		return err
	}
	for _, s := range standings {
		_, err := r.db.Exec(`
			INSERT INTO f1_driver_standings (
				season, driver_id, driver_code, driver_given_name, driver_family_name,
				constructor_name, position, points, wins, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		`, season, s.DriverID, s.DriverCode, s.DriverGivenName, s.DriverFamilyName,
			s.ConstructorName, s.Position, s.Points, s.Wins)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) GetDriverStandings(season int) ([]DriverStanding, error) {
	rows, err := r.db.Query(`
		SELECT position, driver_id, driver_code, driver_given_name, driver_family_name,
		       constructor_name, points, wins
		FROM f1_driver_standings
		WHERE season = $1
		ORDER BY position ASC
	`, season)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var standings []DriverStanding
	for rows.Next() {
		var s DriverStanding
		if err := rows.Scan(
			&s.Position, &s.DriverID, &s.DriverCode, &s.DriverGivenName, &s.DriverFamilyName,
			&s.ConstructorName, &s.Points, &s.Wins,
		); err != nil {
			return nil, err
		}
		standings = append(standings, s)
	}
	return standings, rows.Err()
}

// --- Constructor Standings ---

func (r *Repo) UpsertConstructorStandings(season int, standings []ConstructorStanding) error {
	_, err := r.db.Exec(`DELETE FROM f1_constructor_standings WHERE season = $1`, season)
	if err != nil {
		return err
	}
	for _, s := range standings {
		_, err := r.db.Exec(`
			INSERT INTO f1_constructor_standings (
				season, constructor_id, constructor_name, position, points, wins, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, NOW())
		`, season, s.ConstructorID, s.ConstructorName, s.Position, s.Points, s.Wins)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Repo) GetConstructorStandings(season int) ([]ConstructorStanding, error) {
	rows, err := r.db.Query(`
		SELECT position, constructor_id, constructor_name, points, wins
		FROM f1_constructor_standings
		WHERE season = $1
		ORDER BY position ASC
	`, season)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var standings []ConstructorStanding
	for rows.Next() {
		var s ConstructorStanding
		if err := rows.Scan(
			&s.Position, &s.ConstructorID, &s.ConstructorName, &s.Points, &s.Wins,
		); err != nil {
			return nil, err
		}
		standings = append(standings, s)
	}
	return standings, rows.Err()
}

// --- Daily refresh ---

func (r *Repo) WasRefreshedToday() (bool, error) {
	today := time.Now().UTC().Format("2006-01-02")
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM f1_daily_refresh WHERE refresh_date = $1`, today).Scan(&count)
	return count > 0, err
}

func (r *Repo) GetLastRefreshTime() *string {
	var ts string
	err := r.db.QueryRow(`SELECT refreshed_at FROM f1_daily_refresh ORDER BY refresh_date DESC LIMIT 1`).Scan(&ts)
	if err != nil {
		return nil
	}
	return &ts
}

func (r *Repo) RecordDailyRefresh() error {
	today := time.Now().UTC().Format("2006-01-02")
	_, err := r.db.Exec(`
		INSERT INTO f1_daily_refresh (refresh_date, refreshed_at)
		VALUES ($1, NOW())
		ON CONFLICT (refresh_date) DO UPDATE SET refreshed_at = NOW()
	`, today)
	return err
}

// --- helpers ---

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullableTime(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// HasResults returns true if at least one result row exists for the given round.
func (r *Repo) HasResults(season, round int) (bool, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM f1_race_results WHERE season = $1 AND round = $2`,
		season, round,
	).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	return count > 0, nil
}
