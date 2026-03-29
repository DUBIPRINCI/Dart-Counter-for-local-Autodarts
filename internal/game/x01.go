package game

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type X01Engine struct {
	state     GameState
	history   []GameState
	legThrows [][]Visit
}

func NewX01Engine(opts GameOptions) *X01Engine {
	if opts.StartScore == 0 {
		switch opts.Variant {
		case "301":
			opts.StartScore = 301
		case "701":
			opts.StartScore = 701
		case "901":
			opts.StartScore = 901
		default:
			opts.StartScore = 501
		}
	}
	if opts.Sets == 0 {
		opts.Sets = 1
	}
	if opts.Legs == 0 {
		opts.Legs = 1
	}
	if opts.OutMode == "" {
		opts.OutMode = "double"
	}
	if opts.InMode == "" {
		opts.InMode = "straight"
	}

	players := make([]PlayerState, len(opts.PlayerIDs))
	for i, id := range opts.PlayerIDs {
		startScore := opts.StartScore
		if h, ok := opts.Handicaps[id]; ok {
			startScore -= h
		}
		name := ""
		if i < len(opts.PlayerNames) {
			name = opts.PlayerNames[i]
		}
		players[i] = PlayerState{
			PlayerID:   id,
			PlayerName: name,
			Score:      startScore,
			IsActive:   i == 0,
			HasStarted: opts.InMode == "straight",
		}
	}

	now := time.Now()
	e := &X01Engine{
		state: GameState{
			ID:            uuid.New().String(),
			GameType:      "x01",
			Variant:       opts.Variant,
			Options:       opts,
			Status:        "active",
			Players:       players,
			CurrentPlayer: 0,
			CurrentDart:   0,
			CurrentSet:    1,
			CurrentLeg:    1,
			StartedAt:     &now,
		},
		legThrows: make([][]Visit, len(opts.PlayerIDs)),
	}
	e.updateCheckoutHint()
	return e
}

func (e *X01Engine) GetID() string { return e.state.ID }

func (e *X01Engine) State() *GameState {
	s := e.state
	s.Players = make([]PlayerState, len(e.state.Players))
	copy(s.Players, e.state.Players)
	for i := range s.Players {
		s.Players[i].CurrentVisit.Darts = make([]Throw, len(e.state.Players[i].CurrentVisit.Darts))
		copy(s.Players[i].CurrentVisit.Darts, e.state.Players[i].CurrentVisit.Darts)
	}
	return &s
}

func (e *X01Engine) CheckoutHint(score int) string {
	if hint, ok := CheckoutTable[score]; ok {
		return hint
	}
	return ""
}

func (e *X01Engine) IsVisitComplete() bool {
	return e.state.WaitingTakeout || e.state.CurrentDart >= 3
}

func (e *X01Engine) ProcessThrow(t Throw) ThrowResult {
	// Ignore throws while waiting for takeout
	if e.state.WaitingTakeout {
		return ThrowResult{State: *e.State()}
	}

	e.saveHistory()

	p := &e.state.Players[e.state.CurrentPlayer]
	sounds := []string{"throw"}
	event := ""

	// Double-in check
	if !p.HasStarted && e.state.Options.InMode == "double" {
		if t.Multiplier != 2 {
			p.CurrentVisit.Darts = append(p.CurrentVisit.Darts, t)
			p.DartsThrown++
			e.state.CurrentDart++
			sounds = append(sounds, "miss")
			if e.state.CurrentDart >= 3 {
				e.markVisitDone(p, false)
			}
			e.updateCheckoutHint()
			return ThrowResult{State: *e.State(), SoundEvents: sounds}
		}
		p.HasStarted = true
	}

	// Record score before this visit (for bust revert)
	scoreBeforeVisit := e.scoreAtStartOfVisit(p)

	// Apply dart
	p.Score -= t.Score
	p.DartsThrown++
	p.CurrentVisit.Darts = append(p.CurrentVisit.Darts, t)
	p.CurrentVisit.TotalScore += t.Score
	e.state.CurrentDart++

	// Dart sound
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

	// Check bust
	if e.isBust(p) {
		// Revert score to start of visit
		p.Score = scoreBeforeVisit
		p.CurrentVisit.IsBust = true
		e.state.CurrentDart = 3
		event = "bust"
		sounds = []string{"bust"}
		e.markVisitDone(p, false)
		e.updateCheckoutHint()
		e.state.LastEvent = event
		e.state.SoundEvents = sounds
		return ThrowResult{State: *e.State(), SoundEvents: sounds, Event: event}
	}

	// Check leg/match won
	if p.Score == 0 {
		p.LegsWon++
		legsNeeded := (e.state.Options.Legs/2 + 1)
		if p.LegsWon >= legsNeeded {
			p.SetsWon++
			setsNeeded := (e.state.Options.Sets/2 + 1)
			if p.SetsWon >= setsNeeded {
				event = "matchshot"
				sounds = []string{"matchshot"}
				e.state.WinnerID = p.PlayerID
				e.state.Status = "finished"
				now := time.Now()
				e.state.FinishedAt = &now
			} else {
				event = "gameshot"
				sounds = []string{"gameshot"}
				// newSet is called in FinishTakeout
			}
		} else {
			event = "gameshot"
			sounds = []string{"gameshot"}
			// newLeg is called in FinishTakeout
		}
		e.markVisitDone(p, true)
		e.state.LastEvent = event
		e.state.SoundEvents = sounds
		e.updateCheckoutHint()
		return ThrowResult{State: *e.State(), SoundEvents: sounds, Event: event}
	}

	// Check visit complete (3 darts thrown, no bust, no finish)
	if e.state.CurrentDart >= 3 {
		visitScore := p.CurrentVisit.TotalScore
		switch {
		case visitScore == 180:
			event = "180"
			sounds = append(sounds, "180")
		case visitScore >= 140:
			sounds = append(sounds, "highTon")
		case visitScore >= 100:
			sounds = append(sounds, "lowTon")
		}
		if len(p.CurrentVisit.Darts) == 3 {
			allTriple, allBull := true, true
			for _, d := range p.CurrentVisit.Darts {
				if d.Multiplier != 3 {
					allTriple = false
				}
				if d.Number != 25 {
					allBull = false
				}
			}
			if allTriple || allBull {
				sounds = append(sounds, "hatTrick")
			}
		}
		e.markVisitDone(p, false)
	}

	e.updateAverage(p)
	e.updateCheckoutHint()
	e.state.LastEvent = event
	e.state.SoundEvents = sounds
	return ThrowResult{State: *e.State(), SoundEvents: sounds, Event: event}
}

// markVisitDone records the visit and sets WaitingTakeout.
// Does NOT advance to next player — that happens in FinishTakeout().
func (e *X01Engine) markVisitDone(p *PlayerState, legOver bool) {
	idx := e.playerIndex(p.PlayerID)
	if idx >= 0 {
		e.legThrows[idx] = append(e.legThrows[idx], p.CurrentVisit)
	}
	// Keep CurrentVisit visible (don't clear) until FinishTakeout()
	e.state.WaitingTakeout = true
	// Store whether a new leg/set is needed (handled in FinishTakeout)
	_ = legOver
}

// FinishTakeout is called when darts are removed from the board.
// Clears the visit display, advances to the next player (or starts new leg/set).
func (e *X01Engine) FinishTakeout() *GameState {
	if !e.state.WaitingTakeout {
		return e.State()
	}

	e.state.WaitingTakeout = false
	p := &e.state.Players[e.state.CurrentPlayer]

	// Determine what happens next
	lastEvent := e.state.LastEvent

	// Clear the visit display
	p.CurrentVisit = Visit{}

	switch lastEvent {
	case "gameshot":
		// Check if we need new leg or new set
		legsNeeded := e.state.Options.Legs/2 + 1
		if p.LegsWon >= legsNeeded {
			e.newSet()
		} else {
			e.newLeg()
		}
	case "matchshot":
		// Game is finished, nothing to advance
	case "bust":
		// Score already reverted, advance to next player
		e.advancePlayer()
	default:
		// Normal 3-dart visit
		e.advancePlayer()
	}

	e.state.LastEvent = ""
	e.state.SoundEvents = nil
	e.updateCheckoutHint()
	return e.State()
}

// NextPlayer can be called manually (e.g. NEXT button) — acts as FinishTakeout
func (e *X01Engine) NextPlayer() {
	if e.state.WaitingTakeout {
		e.FinishTakeout()
		return
	}
	// Manual skip (no darts thrown yet this turn)
	e.advancePlayer()
	e.updateCheckoutHint()
}

func (e *X01Engine) advancePlayer() {
	e.state.Players[e.state.CurrentPlayer].IsActive = false
	e.state.CurrentPlayer = (e.state.CurrentPlayer + 1) % len(e.state.Players)
	e.state.Players[e.state.CurrentPlayer].IsActive = true
	e.state.CurrentDart = 0
}

func (e *X01Engine) newLeg() {
	opts := e.state.Options
	for i := range e.state.Players {
		startScore := opts.StartScore
		if h, ok := opts.Handicaps[e.state.Players[i].PlayerID]; ok {
			startScore -= h
		}
		e.state.Players[i].Score = startScore
		e.state.Players[i].CurrentVisit = Visit{}
		e.state.Players[i].HasStarted = opts.InMode == "straight"
		e.state.Players[i].DartsThrown = 0
		e.state.Players[i].Average = 0
	}
	e.state.CurrentLeg++
	e.state.CurrentDart = 0
	e.legThrows = make([][]Visit, len(e.state.Players))
	// Rotate starting player
	e.state.Players[e.state.CurrentPlayer].IsActive = false
	e.state.CurrentPlayer = (e.state.CurrentLeg - 1) % len(e.state.Players)
	e.state.Players[e.state.CurrentPlayer].IsActive = true
}

func (e *X01Engine) newSet() {
	for i := range e.state.Players {
		e.state.Players[i].LegsWon = 0
	}
	e.state.CurrentSet++
	e.newLeg()
}

func (e *X01Engine) scoreAtStartOfVisit(p *PlayerState) int {
	startScore := e.state.Options.StartScore
	if h, ok := e.state.Options.Handicaps[p.PlayerID]; ok {
		startScore -= h
	}
	idx := e.playerIndex(p.PlayerID)
	if idx >= 0 {
		for _, v := range e.legThrows[idx] {
			if !v.IsBust {
				startScore -= v.TotalScore
			}
		}
	}
	return startScore
}

func (e *X01Engine) playerIndex(id string) int {
	for i, p := range e.state.Players {
		if p.PlayerID == id {
			return i
		}
	}
	return -1
}

func (e *X01Engine) isBust(p *PlayerState) bool {
	if p.Score < 0 {
		return true
	}
	if p.Score == 1 && e.state.Options.OutMode == "double" {
		return true
	}
	if p.Score == 0 {
		return !e.isValidOut(p)
	}
	return false
}

func (e *X01Engine) isValidOut(p *PlayerState) bool {
	if len(p.CurrentVisit.Darts) == 0 {
		return false
	}
	last := p.CurrentVisit.Darts[len(p.CurrentVisit.Darts)-1]
	switch e.state.Options.OutMode {
	case "double":
		return last.Multiplier == 2
	case "master":
		return last.Multiplier == 2 || last.Multiplier == 3
	default:
		return true
	}
}

func (e *X01Engine) updateAverage(p *PlayerState) {
	if p.DartsThrown == 0 {
		p.Average = 0
		return
	}
	startScore := e.state.Options.StartScore
	if h, ok := e.state.Options.Handicaps[p.PlayerID]; ok {
		startScore -= h
	}
	scored := startScore - p.Score
	p.Average = float64(scored) / float64(p.DartsThrown) * 3
}

func (e *X01Engine) updateCheckoutHint() {
	p := &e.state.Players[e.state.CurrentPlayer]
	if hint, ok := CheckoutTable[p.Score]; ok {
		e.state.CheckoutHint = hint
	} else {
		e.state.CheckoutHint = ""
	}
}

func (e *X01Engine) saveHistory() {
	e.history = append(e.history, *e.State())
}

func (e *X01Engine) Undo() *GameState {
	if len(e.history) == 0 {
		return e.State()
	}
	e.state = e.history[len(e.history)-1]
	e.history = e.history[:len(e.history)-1]
	e.updateCheckoutHint()
	return e.State()
}

func (e *X01Engine) CorrectThrow(dartIndex int, newThrow Throw) ThrowResult {
	p := &e.state.Players[e.state.CurrentPlayer]
	if dartIndex < 0 || dartIndex >= len(p.CurrentVisit.Darts) {
		return ThrowResult{State: *e.State(), Event: "error"}
	}
	e.saveHistory()
	p.CurrentVisit.Darts[dartIndex] = newThrow
	p.CurrentVisit.TotalScore = 0
	for _, d := range p.CurrentVisit.Darts {
		p.CurrentVisit.TotalScore += d.Score
	}
	p.Score = e.scoreAtStartOfVisit(p)
	for _, d := range p.CurrentVisit.Darts {
		p.Score -= d.Score
	}
	e.updateAverage(p)
	e.updateCheckoutHint()
	return ThrowResult{State: *e.State(), Event: fmt.Sprintf("corrected dart %d", dartIndex+1)}
}
