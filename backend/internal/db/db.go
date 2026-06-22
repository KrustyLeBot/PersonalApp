package db

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
)

// Database wraps sql.DB and owns the connection lifecycle.
type Database struct {
	*sql.DB
}

// New opens and verifies a PostgreSQL connection from DATABASE_URL.
// Returns an empty Database (no error) if DATABASE_URL is unset — the app
// runs in degraded mode without persistence.
func New() (*Database, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Println("DATABASE_URL not set; database features disabled")
		return &Database{}, nil
	}

	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		return nil, err
	}

	log.Println("database connected")
	return &Database{conn}, nil
}

// IsConnected reports whether the underlying connection is available.
func (d *Database) IsConnected() bool {
	return d.DB != nil
}

// Migrate creates all required tables if they do not already exist.
// Must be extended when new features introduce new tables.
func (d *Database) Migrate() error {
	if !d.IsConnected() {
		return nil
	}
	_, err := d.Exec(`
		CREATE TABLE IF NOT EXISTS assets (
			id         SERIAL PRIMARY KEY,
			type       VARCHAR(20)   NOT NULL,
			name       VARCHAR(255)  NOT NULL,
			value      DECIMAL(15,2) DEFAULT 0,
			created_at TIMESTAMPTZ   DEFAULT NOW(),
			updated_at TIMESTAMPTZ   DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS ticker_prices (
			ticker     VARCHAR(20)   PRIMARY KEY,
			price      DECIMAL(15,4) NOT NULL,
			currency   VARCHAR(10)   DEFAULT 'EUR',
			updated_at TIMESTAMPTZ   DEFAULT NOW()
		);

		-- Shared by bourse and crypto assets (any type that holds ticker positions).
		-- Renamed from bourse_holdings — migrate existing data if the old table exists.
		DO $$ BEGIN
			IF EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'bourse_holdings')
			   AND NOT EXISTS (SELECT FROM pg_tables WHERE schemaname = 'public' AND tablename = 'ticker_holdings') THEN
				ALTER TABLE bourse_holdings RENAME TO ticker_holdings;
			END IF;
		END $$;

		CREATE TABLE IF NOT EXISTS ticker_holdings (
			id         SERIAL PRIMARY KEY,
			asset_id   INTEGER       NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
			ticker     VARCHAR(20)   NOT NULL,
			shares     DECIMAL(15,6) NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ   DEFAULT NOW(),
			updated_at TIMESTAMPTZ   DEFAULT NOW()
		);

		-- Optional grouping label for a ticker, used to merge positions in charts.
		CREATE TABLE IF NOT EXISTS ticker_categories (
			ticker   VARCHAR(20)  PRIMARY KEY,
			category VARCHAR(100) NOT NULL
		);

		CREATE TABLE IF NOT EXISTS daily_refresh (
			refresh_date DATE        PRIMARY KEY,
			refreshed_at TIMESTAMPTZ NOT NULL
		);

		CREATE TABLE IF NOT EXISTS dette_assets (
			id               SERIAL PRIMARY KEY,
			asset_id         INTEGER        NOT NULL UNIQUE REFERENCES assets(id) ON DELETE CASCADE,
			start_date       DATE           NOT NULL,
			duration_months  INTEGER        NOT NULL,
			taeg             DECIMAL(6,4)   NOT NULL,
			amount_borrowed  DECIMAL(15,2)  NOT NULL
		);

		CREATE TABLE IF NOT EXISTS projection_rates (
			key           VARCHAR(100) PRIMARY KEY,
			label         VARCHAR(255) NOT NULL,
			rate          DECIMAL(8,4) NOT NULL,
			source_url    TEXT         NOT NULL DEFAULT '',
			rate_override DECIMAL(8,4) DEFAULT NULL,
			updated_at    TIMESTAMPTZ  DEFAULT NOW()
		);

		-- Add rate_override to existing deployments.
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = 'projection_rates' AND column_name = 'rate_override'
			) THEN
				ALTER TABLE projection_rates ADD COLUMN rate_override DECIMAL(8,4) DEFAULT NULL;
			END IF;
		END $$;

		-- Add user_email to data tables for per-user isolation.
		DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'assets' AND column_name = 'user_email') THEN
				ALTER TABLE assets ADD COLUMN user_email VARCHAR(255) NOT NULL DEFAULT '';
				ALTER TABLE assets ALTER COLUMN user_email DROP DEFAULT;
			END IF;
			IF NOT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'ticker_categories' AND column_name = 'user_email') THEN
				ALTER TABLE ticker_categories DROP CONSTRAINT ticker_categories_pkey;
				ALTER TABLE ticker_categories ADD COLUMN user_email VARCHAR(255) NOT NULL DEFAULT '';
				ALTER TABLE ticker_categories ALTER COLUMN user_email DROP DEFAULT;
				ALTER TABLE ticker_categories ADD PRIMARY KEY (ticker, user_email);
			END IF;
		END $$;

		-- Migrate telework_preset from singleton (id=1) to per-user (user_email PK).
		DO $$ BEGIN
			IF EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = 'telework_preset' AND column_name = 'id'
			) AND NOT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = 'telework_preset' AND column_name = 'user_email'
			) THEN
				ALTER TABLE telework_preset ADD COLUMN user_email VARCHAR(255) NOT NULL DEFAULT '';
				ALTER TABLE telework_preset DROP CONSTRAINT IF EXISTS telework_preset_pkey;
				ALTER TABLE telework_preset DROP CONSTRAINT IF EXISTS telework_preset_id_check;
				ALTER TABLE telework_preset DROP COLUMN id;
				ALTER TABLE telework_preset ADD PRIMARY KEY (user_email);
			END IF;
		END $$;

		CREATE TABLE IF NOT EXISTS telework_preset (
			user_email  VARCHAR(255) PRIMARY KEY,
			remote_days TEXT NOT NULL DEFAULT '[4,5]'
		);

		CREATE TABLE IF NOT EXISTS telework_leaves (
			leave_date DATE    PRIMARY KEY,
			year       INTEGER NOT NULL
		);

		-- Per-day overrides: supersede the weekly preset for a specific date.
		-- type: 'leave' | 'remote' | 'office'
		CREATE TABLE IF NOT EXISTS telework_overrides (
			override_date DATE         NOT NULL,
			year          INTEGER      NOT NULL,
			type          VARCHAR(10)  NOT NULL CHECK (type IN ('leave','remote','office')),
			user_email    VARCHAR(255) NOT NULL DEFAULT '',
			PRIMARY KEY (override_date, user_email)
		);

		-- Migrate existing leaves into overrides (idempotent).
		INSERT INTO telework_overrides (override_date, year, type)
		SELECT leave_date, year, 'leave' FROM telework_leaves
		ON CONFLICT DO NOTHING;

		-- Add user_email to telework_overrides if upgrading from old schema.
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = 'telework_overrides' AND column_name = 'user_email'
			) THEN
				ALTER TABLE telework_overrides DROP CONSTRAINT IF EXISTS telework_overrides_pkey;
				ALTER TABLE telework_overrides ADD COLUMN user_email VARCHAR(255) NOT NULL DEFAULT '';
				ALTER TABLE telework_overrides ADD PRIMARY KEY (override_date, user_email);
			END IF;
		END $$;

		-- Add user_email to telework_leaves if still used.
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = 'telework_leaves' AND column_name = 'user_email'
			) THEN
				ALTER TABLE telework_leaves DROP CONSTRAINT IF EXISTS telework_leaves_pkey;
				ALTER TABLE telework_leaves ADD COLUMN user_email VARCHAR(255) NOT NULL DEFAULT '';
				ALTER TABLE telework_leaves ADD PRIMARY KEY (leave_date, user_email);
			END IF;
		END $$;

		CREATE TABLE IF NOT EXISTS lol_leagues (
			slug       VARCHAR(30)  NOT NULL,
			name       VARCHAR(60)  NOT NULL,
			league_id  VARCHAR(30)  NOT NULL,
			region     VARCHAR(30)  NOT NULL DEFAULT '',
			image_url  TEXT         NOT NULL DEFAULT '',
			enabled    BOOLEAN      NOT NULL DEFAULT TRUE,
			user_email VARCHAR(255) NOT NULL DEFAULT '',
			PRIMARY KEY (slug, user_email)
		);

		-- Add region/image_url to existing deployments.
		DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'lol_leagues' AND column_name = 'region') THEN
				ALTER TABLE lol_leagues ADD COLUMN region VARCHAR(30) NOT NULL DEFAULT '';
			END IF;
			IF NOT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'lol_leagues' AND column_name = 'image_url') THEN
				ALTER TABLE lol_leagues ADD COLUMN image_url TEXT NOT NULL DEFAULT '';
			END IF;
		END $$;

		-- Add user_email to lol_leagues if upgrading from old schema (PK was slug alone).
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = 'lol_leagues' AND column_name = 'user_email'
			) THEN
				ALTER TABLE lol_leagues DROP CONSTRAINT IF EXISTS lol_leagues_pkey;
				ALTER TABLE lol_leagues ADD COLUMN user_email VARCHAR(255) NOT NULL DEFAULT '';
				ALTER TABLE lol_leagues ADD PRIMARY KEY (slug, user_email);
			END IF;
		END $$;

		CREATE TABLE IF NOT EXISTS lol_matches (
			match_id           VARCHAR(30)  NOT NULL,
			user_email         VARCHAR(255) NOT NULL DEFAULT '',
			league_name        VARCHAR(30)  NOT NULL,
			league_slug        VARCHAR(30)  NOT NULL,
			team1_name         VARCHAR(60),
			team1_code         VARCHAR(10),
			team1_image        TEXT,
			team1_wins         INT          NOT NULL DEFAULT 0,
			team1_outcome      VARCHAR(10)  NOT NULL DEFAULT '',
			team2_name         VARCHAR(60),
			team2_code         VARCHAR(10),
			team2_image        TEXT,
			team2_wins         INT          NOT NULL DEFAULT 0,
			team2_outcome      VARCHAR(10)  NOT NULL DEFAULT '',
			scheduled_at       TIMESTAMPTZ  NOT NULL,
			stage              VARCHAR(60),
			best_of            INT,
			state              VARCHAR(20)  NOT NULL,
			is_spoiler         BOOLEAN      NOT NULL DEFAULT FALSE,
			spoiler_dismissed  BOOLEAN      NOT NULL DEFAULT FALSE,
			fetched_at         TIMESTAMPTZ  NOT NULL,
			PRIMARY KEY (match_id, user_email)
		);

		-- Add spoiler_dismissed to existing deployments.
		DO $$ BEGIN
			IF NOT EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'lol_matches' AND column_name = 'spoiler_dismissed') THEN
				ALTER TABLE lol_matches ADD COLUMN spoiler_dismissed BOOLEAN NOT NULL DEFAULT FALSE;
			END IF;
		END $$;

		-- Add user_email to lol_matches if upgrading from old schema (PK was match_id alone).
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = 'lol_matches' AND column_name = 'user_email'
			) THEN
				ALTER TABLE lol_matches DROP CONSTRAINT IF EXISTS lol_matches_pkey;
				ALTER TABLE lol_matches ADD COLUMN user_email VARCHAR(255) NOT NULL DEFAULT '';
				ALTER TABLE lol_matches ADD PRIMARY KEY (match_id, user_email);
			END IF;
		END $$;

		CREATE TABLE IF NOT EXISTS lol_daily_refresh (
			refresh_date DATE         NOT NULL,
			user_email   VARCHAR(255) NOT NULL DEFAULT '',
			refreshed_at TIMESTAMPTZ  NOT NULL,
			PRIMARY KEY (refresh_date, user_email)
		);

		-- Add user_email to lol_daily_refresh if upgrading from old schema.
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = 'lol_daily_refresh' AND column_name = 'user_email'
			) THEN
				ALTER TABLE lol_daily_refresh DROP CONSTRAINT IF EXISTS lol_daily_refresh_pkey;
				ALTER TABLE lol_daily_refresh ADD COLUMN user_email VARCHAR(255) NOT NULL DEFAULT '';
				ALTER TABLE lol_daily_refresh ADD PRIMARY KEY (refresh_date, user_email);
			END IF;
		END $$;

		CREATE TABLE IF NOT EXISTS f1_races (
			season       INT          NOT NULL,
			round        INT          NOT NULL,
			race_name    VARCHAR(100) NOT NULL,
			circuit_id   VARCHAR(50),
			circuit_name VARCHAR(100),
			locality     VARCHAR(100),
			country      VARCHAR(100),
			race_date    DATE         NOT NULL,
			race_time    TIME,
			fetched_at   TIMESTAMPTZ  NOT NULL,
			PRIMARY KEY (season, round)
		);

		CREATE TABLE IF NOT EXISTS f1_race_results (
			season             INT          NOT NULL,
			round              INT          NOT NULL,
			position           INT,
			driver_id          VARCHAR(50)  NOT NULL,
			driver_code        VARCHAR(5),
			driver_given_name  VARCHAR(50),
			driver_family_name VARCHAR(50),
			constructor_id     VARCHAR(50)  NOT NULL,
			constructor_name   VARCHAR(100) NOT NULL,
			grid               INT,
			laps               INT,
			points             NUMERIC(5,1),
			status             VARCHAR(50),
			fastest_lap_rank   INT,
			fastest_lap_time   VARCHAR(20),
			fetched_at         TIMESTAMPTZ  NOT NULL,
			PRIMARY KEY (season, round, driver_id),
			FOREIGN KEY (season, round) REFERENCES f1_races(season, round)
		);

		CREATE TABLE IF NOT EXISTS f1_driver_standings (
			season             INT          NOT NULL,
			driver_id          VARCHAR(50)  NOT NULL,
			driver_code        VARCHAR(5),
			driver_given_name  VARCHAR(50),
			driver_family_name VARCHAR(50),
			constructor_name   VARCHAR(100),
			position           INT          NOT NULL,
			points             NUMERIC(6,1) NOT NULL,
			wins               INT          DEFAULT 0,
			updated_at         TIMESTAMPTZ  NOT NULL,
			PRIMARY KEY (season, driver_id)
		);

		CREATE TABLE IF NOT EXISTS f1_constructor_standings (
			season           INT          NOT NULL,
			constructor_id   VARCHAR(50)  NOT NULL,
			constructor_name VARCHAR(100) NOT NULL,
			position         INT          NOT NULL,
			points           NUMERIC(6,1) NOT NULL,
			wins             INT          DEFAULT 0,
			updated_at       TIMESTAMPTZ  NOT NULL,
			PRIMARY KEY (season, constructor_id)
		);

		CREATE TABLE IF NOT EXISTS f1_qualifying_results (
			season             INT          NOT NULL,
			round              INT          NOT NULL,
			position           INT          NOT NULL,
			driver_id          VARCHAR(50)  NOT NULL,
			driver_code        VARCHAR(5),
			driver_given_name  VARCHAR(50),
			driver_family_name VARCHAR(50),
			constructor_id     VARCHAR(50)  NOT NULL,
			constructor_name   VARCHAR(100) NOT NULL,
			q1                 VARCHAR(20),
			q2                 VARCHAR(20),
			q3                 VARCHAR(20),
			fetched_at         TIMESTAMPTZ  NOT NULL,
			PRIMARY KEY (season, round, driver_id),
			FOREIGN KEY (season, round) REFERENCES f1_races(season, round)
		);

		CREATE TABLE IF NOT EXISTS f1_daily_refresh (
			refresh_date DATE        NOT NULL PRIMARY KEY,
			refreshed_at TIMESTAMPTZ NOT NULL
		);

		CREATE TABLE IF NOT EXISTS user_settings (
			user_email        VARCHAR(255) PRIMARY KEY,
			enabled_features  TEXT         NOT NULL DEFAULT '[]'
		);
	`)
	return err
}

// HealthHandler returns a protected handler that reports DB connectivity.
// The route must be registered behind RequireAuth in main.
func (d *Database) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Intentionally minimal response — do not leak internal details.
		status := `{"status":"ok","db":"disconnected"}`
		if d.IsConnected() {
			if err := d.QueryRow("SELECT 1").Scan(new(int)); err == nil {
				status = `{"status":"ok","db":"connected"}`
			} else {
				status = `{"status":"ok","db":"error"}`
			}
		}
		w.Write([]byte(status))
	}
}
