package storage

import "fmt"

var migrations = []string{
	// v1: initial schema
	`CREATE TABLE IF NOT EXISTS players (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		avatar TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE TABLE IF NOT EXISTS games (
		id TEXT PRIMARY KEY,
		game_type TEXT NOT NULL,
		variant TEXT NOT NULL,
		options TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'setup',
		winner_id TEXT,
		started_at DATETIME,
		finished_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (winner_id) REFERENCES players(id)
	)`,

	`CREATE TABLE IF NOT EXISTS game_players (
		game_id TEXT NOT NULL,
		player_id TEXT NOT NULL,
		position INTEGER NOT NULL,
		handicap INTEGER DEFAULT 0,
		PRIMARY KEY (game_id, player_id),
		FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE,
		FOREIGN KEY (player_id) REFERENCES players(id)
	)`,

	`CREATE TABLE IF NOT EXISTS sets (
		id TEXT PRIMARY KEY,
		game_id TEXT NOT NULL,
		set_number INTEGER NOT NULL,
		winner_id TEXT,
		FOREIGN KEY (game_id) REFERENCES games(id) ON DELETE CASCADE,
		FOREIGN KEY (winner_id) REFERENCES players(id)
	)`,

	`CREATE TABLE IF NOT EXISTS legs (
		id TEXT PRIMARY KEY,
		set_id TEXT NOT NULL,
		leg_number INTEGER NOT NULL,
		winner_id TEXT,
		FOREIGN KEY (set_id) REFERENCES sets(id) ON DELETE CASCADE,
		FOREIGN KEY (winner_id) REFERENCES players(id)
	)`,

	`CREATE TABLE IF NOT EXISTS throws (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		leg_id TEXT NOT NULL,
		player_id TEXT NOT NULL,
		turn_number INTEGER NOT NULL,
		dart_number INTEGER NOT NULL,
		segment TEXT NOT NULL,
		number INTEGER NOT NULL,
		multiplier INTEGER NOT NULL,
		score INTEGER NOT NULL,
		x REAL,
		y REAL,
		is_manual BOOLEAN DEFAULT 0,
		thrown_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (leg_id) REFERENCES legs(id) ON DELETE CASCADE,
		FOREIGN KEY (player_id) REFERENCES players(id)
	)`,

	`CREATE INDEX IF NOT EXISTS idx_throws_leg ON throws(leg_id)`,
	`CREATE INDEX IF NOT EXISTS idx_throws_player ON throws(player_id)`,
	`CREATE INDEX IF NOT EXISTS idx_legs_set ON legs(set_id)`,
	`CREATE INDEX IF NOT EXISTS idx_sets_game ON sets(game_id)`,
	`CREATE INDEX IF NOT EXISTS idx_games_status ON games(status)`,

	`CREATE TABLE IF NOT EXISTS leg_stats (
		leg_id TEXT NOT NULL,
		player_id TEXT NOT NULL,
		darts_thrown INTEGER DEFAULT 0,
		total_score INTEGER DEFAULT 0,
		average REAL DEFAULT 0,
		first9_avg REAL DEFAULT 0,
		highest_visit INTEGER DEFAULT 0,
		doubles_hit INTEGER DEFAULT 0,
		doubles_attempted INTEGER DEFAULT 0,
		triples_hit INTEGER DEFAULT 0,
		one_eighties INTEGER DEFAULT 0,
		ton_plus INTEGER DEFAULT 0,
		PRIMARY KEY (leg_id, player_id),
		FOREIGN KEY (leg_id) REFERENCES legs(id) ON DELETE CASCADE,
		FOREIGN KEY (player_id) REFERENCES players(id)
	)`,
}

func (db *DB) migrate() error {
	// Create migration tracking table
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY
	)`)
	if err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	var current int
	row := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version")
	if err := row.Scan(&current); err != nil {
		return fmt.Errorf("get schema version: %w", err)
	}

	for i := current; i < len(migrations); i++ {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", i+1); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", i+1, err)
		}
	}

	return nil
}
