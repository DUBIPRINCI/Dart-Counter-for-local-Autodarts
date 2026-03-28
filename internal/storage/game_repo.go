package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type GameRecord struct {
	ID         string     `json:"id"`
	GameType   string     `json:"gameType"`
	Variant    string     `json:"variant"`
	Options    string     `json:"options"`
	Status     string     `json:"status"`
	WinnerID   *string    `json:"winnerId"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

func (db *DB) CreateGame(id, gameType, variant string, options interface{}) error {
	optsJSON, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("marshal options: %w", err)
	}

	_, err = db.Exec(
		"INSERT INTO games (id, game_type, variant, options, status, started_at) VALUES (?, ?, ?, ?, 'active', ?)",
		id, gameType, variant, string(optsJSON), time.Now(),
	)
	return err
}

func (db *DB) UpdateGameStatus(id, status string, winnerID *string) error {
	if status == "finished" {
		_, err := db.Exec(
			"UPDATE games SET status = ?, winner_id = ?, finished_at = ? WHERE id = ?",
			status, winnerID, time.Now(), id,
		)
		return err
	}
	_, err := db.Exec("UPDATE games SET status = ? WHERE id = ?", status, id)
	return err
}

func (db *DB) GetGame(id string) (*GameRecord, error) {
	g := &GameRecord{}
	err := db.QueryRow(
		"SELECT id, game_type, variant, options, status, winner_id, started_at, finished_at, created_at FROM games WHERE id = ?", id,
	).Scan(&g.ID, &g.GameType, &g.Variant, &g.Options, &g.Status, &g.WinnerID, &g.StartedAt, &g.FinishedAt, &g.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return g, err
}

func (db *DB) ListGames(limit, offset int) ([]GameRecord, error) {
	rows, err := db.Query(
		"SELECT id, game_type, variant, options, status, winner_id, started_at, finished_at, created_at FROM games ORDER BY created_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []GameRecord
	for rows.Next() {
		var g GameRecord
		if err := rows.Scan(&g.ID, &g.GameType, &g.Variant, &g.Options, &g.Status, &g.WinnerID, &g.StartedAt, &g.FinishedAt, &g.CreatedAt); err != nil {
			return nil, err
		}
		games = append(games, g)
	}
	return games, rows.Err()
}

func (db *DB) AddGamePlayer(gameID, playerID string, position, handicap int) error {
	_, err := db.Exec(
		"INSERT INTO game_players (game_id, player_id, position, handicap) VALUES (?, ?, ?, ?)",
		gameID, playerID, position, handicap,
	)
	return err
}

func (db *DB) DeleteGame(id string) error {
	_, err := db.Exec("DELETE FROM games WHERE id = ?", id)
	return err
}
