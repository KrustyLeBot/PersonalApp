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
			if err := d.Ping(); err == nil {
				status = `{"status":"ok","db":"connected"}`
			} else {
				status = `{"status":"ok","db":"error"}`
			}
		}
		w.Write([]byte(status))
	}
}
