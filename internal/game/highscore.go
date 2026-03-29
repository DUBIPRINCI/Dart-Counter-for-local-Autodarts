package game

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type HighScoreEngine struct {
	state     GameState
	round     int
	maxRounds int
	history   []highscoreSnapshot
}

type highscoreSnapshot struct {
	state GameState
	round int
}

func NewHighScoreEngine(opts GameOptions) *HighScoreEngine {
	rounds := opts.Rounds
	if rounds == 0 {
		rounds = 10
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
	return &HighScoreEngine{
		state: GameState{
			ID:            uuid.New().String(),
			GameType:      "highscore",
			Variant:       fmt.Sprintf("%d rounds", rounds),
			Options:       opts,
			Status:        "active",
			Players:       players,
			CurrentPlayer: 0,
			CurrentDart:   0,
			CurrentSet:    1,
			CurrentLeg:    1,
			StartedAt:     &now,
			CheckoutHint:  fmt.Sprintf("Round 1/%d", rounds),
		},
		round:     1,
		maxRounds: rounds,
	}
}

func (e *HighScoreEngine) GetID() string           { return e.state.ID }
func (e *HighScoreEngine) State() *GameState        { s := e.state; return &s }
func (e *HighScoreEngine) CheckoutHint(int) string  { return fmt.Sprintf("Round %d/%d", e.round, e.maxRounds) }
func (e *HighScoreEngine) IsVisitComplete() bool     { return e.state.WaitingTakeout || e.state.CurrentDart >= 3 }

func (e *HighScoreEngine) FinishTakeout() *GameState {
	if !e.state.WaitingTakeout {
		return e.State()
	}
	e.state.WaitingTakeout = false
	e.state.Players[e.state.CurrentPlayer].CurrentVisit = Visit{}
	if e.state.Status != "finished" {
		e.advancePlayer()
	}
	return e.State()
}

func (e *HighScoreEngine) ProcessThrow(t Throw) ThrowResult {
	e.saveHistory()

	p := &e.state.Players[e.state.CurrentPlayer]
	sounds := []string{"throw"}

	p.Score += t.Score
	p.CurrentVisit.Darts = append(p.CurrentVisit.Darts, t)
	p.CurrentVisit.TotalScore += t.Score
	p.DartsThrown++
	e.state.CurrentDart++

	switch {
	case t.Multiplier == 3:
		sounds = append(sounds, "triple")
	case t.Multiplier == 2 && t.Number == 25:
		sounds = append(sounds, "dbull")
	case t.Number == 25:
		sounds = append(sounds, "bull")
	case t.Multiplier == 2:
		sounds = append(sounds, "double")
	case t.Multiplier == 0:
		sounds = append(sounds, "miss")
	default:
		sounds = append(sounds, "single")
	}

	if e.state.CurrentDart >= 3 {
		visitScore := p.CurrentVisit.TotalScore
		if visitScore == 180 {
			sounds = append(sounds, "180")
		} else if visitScore >= 140 {
			sounds = append(sounds, "highTon")
		} else if visitScore >= 100 {
			sounds = append(sounds, "lowTon")
		}

		e.updateAverage(p)
		// Don't clear visit or advance yet — wait for FinishTakeout()
		e.state.WaitingTakeout = true
	}

	e.state.CheckoutHint = fmt.Sprintf("Round %d/%d", e.round, e.maxRounds)
	return ThrowResult{State: e.state, SoundEvents: sounds}
}

func (e *HighScoreEngine) advancePlayer() {
	e.state.Players[e.state.CurrentPlayer].IsActive = false
	e.state.CurrentPlayer = (e.state.CurrentPlayer + 1) % len(e.state.Players)
	e.state.Players[e.state.CurrentPlayer].IsActive = true
	e.state.CurrentDart = 0

	if e.state.CurrentPlayer == 0 {
		e.round++
		if e.round > e.maxRounds {
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

func (e *HighScoreEngine) updateAverage(p *PlayerState) {
	if p.DartsThrown == 0 {
		return
	}
	p.Average = float64(p.Score) / float64(p.DartsThrown) * 3
}

func (e *HighScoreEngine) NextPlayer() {
	if e.state.WaitingTakeout {
		e.FinishTakeout()
		return
	}
	e.advancePlayer()
}

func (e *HighScoreEngine) saveHistory() {
	snap := highscoreSnapshot{state: e.state, round: e.round}
	snap.state.Players = make([]PlayerState, len(e.state.Players))
	copy(snap.state.Players, e.state.Players)
	e.history = append(e.history, snap)
}

func (e *HighScoreEngine) Undo() *GameState {
	if len(e.history) == 0 {
		return e.State()
	}
	snap := e.history[len(e.history)-1]
	e.state = snap.state
	e.round = snap.round
	e.history = e.history[:len(e.history)-1]
	return e.State()
}
