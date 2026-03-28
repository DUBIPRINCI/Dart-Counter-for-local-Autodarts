package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Player struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Avatar    string    `json:"avatar"`
	CreatedAt time.Time `json:"createdAt"`
}

func (db *DB) CreatePlayer(name, avatar string) (*Player, error) {
	p := &Player{
		ID:        uuid.New().String(),
		Name:      name,
		Avatar:    avatar,
		CreatedAt: time.Now(),
	}
	_, err := db.Exec(
		"INSERT INTO players (id, name, avatar, created_at) VALUES (?, ?, ?, ?)",
		p.ID, p.Name, p.Avatar, p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert player: %w", err)
	}
	return p, nil
}

func (db *DB) GetPlayer(id string) (*Player, error) {
	p := &Player{}
	err := db.QueryRow(
		"SELECT id, name, avatar, created_at FROM players WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.Avatar, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get player: %w", err)
	}
	return p, nil
}

func (db *DB) ListPlayers() ([]Player, error) {
	rows, err := db.Query("SELECT id, name, avatar, created_at FROM players ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list players: %w", err)
	}
	defer rows.Close()

	var players []Player
	for rows.Next() {
		var p Player
		if err := rows.Scan(&p.ID, &p.Name, &p.Avatar, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan player: %w", err)
		}
		players = append(players, p)
	}
	return players, rows.Err()
}

func (db *DB) UpdatePlayer(id, name, avatar string) error {
	res, err := db.Exec("UPDATE players SET name = ?, avatar = ? WHERE id = ?", name, avatar, id)
	if err != nil {
		return fmt.Errorf("update player: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("player not found")
	}
	return nil
}

func (db *DB) DeletePlayer(id string) error {
	res, err := db.Exec("DELETE FROM players WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete player: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("player not found")
	}
	return nil
}
