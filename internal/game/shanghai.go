package game

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ShanghaiEngine struct {
	state      GameState
	round      int // current round (1-based)
	maxRounds  int
	history    []shanghaiSnapshot
}

type shanghaiSnapshot struct {
	state GameState
	round int
}

func NewShanghaiEngine(opts GameOptions) *ShanghaiEngine {
	rounds := opts.Rounds
	if rounds == 0 {
		rounds = 7
	}

	players := make([]PlayerState, len(opts.PlayerIDs))
	for i, id := range opts.PlayerIDs {
		name := ""
		if i < len(opts.PlayerNames) {
			name = opts.PlayerNames[i]
		}
		players[i] = PlayerState{
			PlayerID:   id,
			PlayerName: name,
			Score:      0,
			IsActive:   i == 0,
			HasStarted: true,
		}
	}

	now := time.Now()
	return &ShanghaiEngine{
		state: GameState{
			ID:            uuid.New().String(),
			GameType:      "shanghai",
			Variant:       fmt.Sprintf("%d rounds", rounds),
			Options:       opts,
			Status:        "active",
			Players:       players,
			CurrentPlayer: 0,
			CurrentDart:   0,
			CurrentSet:    1,
			CurrentLeg:    1,
			StartedAt:     &now,
			CheckoutHint:  "Target: 1",
		},
		round:     1,
		maxRounds: rounds,
	}
}

func (e *ShanghaiEngine) GetID() string           { return e.state.ID }
func (e *ShanghaiEngine) State() *GameState        { s := e.state; return &s }
func (e *ShanghaiEngine) CheckoutHint(int) string  { return fmt.Sprintf("Target: %d", e.round) }
func (e *ShanghaiEngine) IsVisitComplete() bool     { return e.state.CurrentDart >= 3 }

func (e *ShanghaiEngine) ProcessThrow(t Throw) ThrowResult {
	e.saveHistory()

	p := &e.state.Players[e.state.CurrentPlayer]
	sounds := []string{"throw"}

	p.CurrentVisit.Darts = append(p.CurrentVisit.Darts, t)
	p.DartsThrown++
	e.state.CurrentDart++

	// Only score hits on the current round's number
	if t.Number == e.round {
		p.Score += t.Score
		p.CurrentVisit.TotalScore += t.Score
		switch t.Multiplier {
		case 3:
			sounds = append(sounds, "triple")
		case 2:
			sounds = append(sounds, "double")
		default:
			sounds = append(sounds, "single")
		}
	} else {
		sounds = append(sounds, "miss")
	}

	// Check Shanghai (single + double + triple of target in one visit)
	if e.state.CurrentDart >= 3 || len(p.CurrentVisit.Darts) == 3 {
		if e.checkShanghai(p) {
			e.state.WinnerID = p.PlayerID
			e.state.Status = "finished"
			now := time.Now()
			e.state.FinishedAt = &now
			sounds = []string{"gameshot"}
			return ThrowResult{State: e.state, SoundEvents: sounds, Event: "gameshot"}
		}
	}

	if e.state.CurrentDart >= 3 {
		p.CurrentVisit = Visit{}
		e.advancePlayer()
	}

	e.state.CheckoutHint = fmt.Sprintf("Target: %d", e.round)
	return ThrowResult{State: e.state, SoundEvents: sounds}
}

func (e *ShanghaiEngine) checkShanghai(p *PlayerState) bool {
	if len(p.CurrentVisit.Darts) != 3 {
		return false
	}
	hasSingle, hasDouble, hasTriple := false, false, false
	for _, d := range p.CurrentVisit.Darts {
		if d.Number != e.round {
			continue
		}
		switch d.Multiplier {
		case 1:
			hasSingle = true
		case 2:
			hasDouble = true
		case 3:
			hasTriple = true
		}
	}
	return hasSingle && hasDouble && hasTriple
}

func (e *ShanghaiEngine) advancePlayer() {
	e.state.Players[e.state.CurrentPlayer].IsActive = false
	e.state.CurrentPlayer = (e.state.CurrentPlayer + 1) % len(e.state.Players)
	e.state.Players[e.state.CurrentPlayer].IsActive = true
	e.state.CurrentDart = 0

	// If we've gone through all players, advance the round
	if e.state.CurrentPlayer == 0 {
		e.round++
		if e.round > e.maxRounds {
			// Game over - highest score wins
			maxScore := -1
			winnerIdx := 0
			for i, p := range e.state.Players {
				if p.Score > maxScore {
					maxScore = p.Score
					winnerIdx = i
				}
			}
			e.state.WinnerID = e.state.Players[winnerIdx].PlayerID
			e.state.Status = "finished"
			now := time.Now()
			e.state.FinishedAt = &now
		}
	}
}

func (e *ShanghaiEngine) NextPlayer() {
	e.advancePlayer()
}

func (e *ShanghaiEngine) saveHistory() {
	snap := shanghaiSnapshot{state: e.state, round: e.round}
	snap.state.Players = make([]PlayerState, len(e.state.Players))
	copy(snap.state.Players, e.state.Players)
	e.history = append(e.history, snap)
}

func (e *ShanghaiEngine) Undo() *GameState {
	if len(e.history) == 0 {
		return e.State()
	}
	snap := e.history[len(e.history)-1]
	e.state = snap.state
	e.round = snap.round
	e.history = e.history[:len(e.history)-1]
	return e.State()
}
