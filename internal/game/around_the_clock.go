package game

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ATCPlayerState struct {
	CurrentTarget int `json:"currentTarget"` // 1-20, then 25 (bull)
}

type ATCEngine struct {
	state    GameState
	atcState []ATCPlayerState
	history  []atcSnapshot
}

type atcSnapshot struct {
	state    GameState
	atcState []ATCPlayerState
}

func NewATCEngine(opts GameOptions) *ATCEngine {
	players := make([]PlayerState, len(opts.PlayerIDs))
	atcState := make([]ATCPlayerState, len(opts.PlayerIDs))

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
		atcState[i] = ATCPlayerState{CurrentTarget: 1}
	}

	now := time.Now()
	return &ATCEngine{
		state: GameState{
			ID:            uuid.New().String(),
			GameType:      "atc",
			Variant:       "standard",
			Options:       opts,
			Status:        "active",
			Players:       players,
			CurrentPlayer: 0,
			CurrentDart:   0,
			CurrentSet:    1,
			CurrentLeg:    1,
			StartedAt:     &now,
		},
		atcState: atcState,
	}
}

func (e *ATCEngine) GetID() string           { return e.state.ID }
func (e *ATCEngine) State() *GameState        { s := e.state; return &s }
func (e *ATCEngine) CheckoutHint(int) string  { return "" }
func (e *ATCEngine) IsVisitComplete() bool     { return e.state.CurrentDart >= 3 }

func (e *ATCEngine) ProcessThrow(t Throw) ThrowResult {
	e.saveHistory()

	p := &e.state.Players[e.state.CurrentPlayer]
	as := &e.atcState[e.state.CurrentPlayer]
	sounds := []string{"throw"}

	p.CurrentVisit.Darts = append(p.CurrentVisit.Darts, t)
	p.DartsThrown++
	e.state.CurrentDart++

	// Check if dart hits the current target
	target := as.CurrentTarget
	if t.Number == target {
		advance := 1
		if e.state.Options.DoubleSkip && t.Multiplier == 2 {
			advance = 2
		}
		if e.state.Options.TripleSkip && t.Multiplier == 3 {
			advance = 3
		}

		as.CurrentTarget += advance
		p.Score = as.CurrentTarget - 1 // score = targets hit

		// Check if completed (past 20, or hit bull)
		if as.CurrentTarget > 20 {
			if as.CurrentTarget > 25 { // already past bull
				e.state.WinnerID = p.PlayerID
				e.state.Status = "finished"
				now := time.Now()
				e.state.FinishedAt = &now
				sounds = []string{"gameshot"}
				return ThrowResult{State: e.state, SoundEvents: sounds, Event: "gameshot"}
			}
			as.CurrentTarget = 25 // next target is bull
		}
		if target == 25 { // was targeting bull and hit it
			e.state.WinnerID = p.PlayerID
			e.state.Status = "finished"
			now := time.Now()
			e.state.FinishedAt = &now
			sounds = []string{"gameshot"}
			return ThrowResult{State: e.state, SoundEvents: sounds, Event: "gameshot"}
		}

		sounds = append(sounds, "single")
	} else {
		sounds = append(sounds, "miss")
	}

	// Update checkout hint to show current target
	e.state.CheckoutHint = fmt.Sprintf("Target: %d", as.CurrentTarget)

	if e.state.CurrentDart >= 3 {
		p.CurrentVisit = Visit{}
		e.NextPlayer()
	}

	return ThrowResult{State: e.state, SoundEvents: sounds}
}

func (e *ATCEngine) NextPlayer() {
	e.state.Players[e.state.CurrentPlayer].IsActive = false
	e.state.CurrentPlayer = (e.state.CurrentPlayer + 1) % len(e.state.Players)
	e.state.Players[e.state.CurrentPlayer].IsActive = true
	e.state.CurrentDart = 0

	as := e.atcState[e.state.CurrentPlayer]
	e.state.CheckoutHint = fmt.Sprintf("Target: %d", as.CurrentTarget)
}

func (e *ATCEngine) saveHistory() {
	snap := atcSnapshot{state: e.state}
	snap.state.Players = make([]PlayerState, len(e.state.Players))
	copy(snap.state.Players, e.state.Players)
	snap.atcState = make([]ATCPlayerState, len(e.atcState))
	copy(snap.atcState, e.atcState)
	e.history = append(e.history, snap)
}

func (e *ATCEngine) Undo() *GameState {
	if len(e.history) == 0 {
		return e.State()
	}
	snap := e.history[len(e.history)-1]
	e.state = snap.state
	e.atcState = snap.atcState
	e.history = e.history[:len(e.history)-1]
	return e.State()
}
