package game

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type X01Engine struct {
	state     GameState
	history   []GameState // for undo
	legThrows [][]Visit   // throws per player for current leg
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

	// Set checkout hint for first player
	e.updateCheckoutHint()

	return e
}

func (e *X01Engine) GetID() string {
	return e.state.ID
}

func (e *X01Engine) State() *GameState {
	s := e.state
	return &s
}

func (e *X01Engine) CheckoutHint(score int) string {
	if hint, ok := CheckoutTable[score]; ok {
		return hint
	}
	return ""
}

func (e *X01Engine) IsVisitComplete() bool {
	return e.state.CurrentDart >= 3
}

func (e *X01Engine) ProcessThrow(t Throw) ThrowResult {
	// Save state for undo
	e.saveHistory()

	p := &e.state.Players[e.state.CurrentPlayer]
	sounds := []string{"throw"}
	event := ""

	// Check double-in requirement
	if !p.HasStarted && e.state.Options.InMode == "double" {
		if t.Multiplier != 2 {
			// Dart doesn't count, but still track it
			p.CurrentVisit.Darts = append(p.CurrentVisit.Darts, t)
			p.DartsThrown++
			e.state.CurrentDart++
			sounds = append(sounds, "miss")

			if e.state.CurrentDart >= 3 {
				e.finishVisit(p)
			}
			e.updateCheckoutHint()
			return ThrowResult{State: e.state, SoundEvents: sounds, Event: event}
		}
		p.HasStarted = true
	}

	// Apply score
	previousScore := p.Score
	p.Score -= t.Score
	p.DartsThrown++
	p.CurrentVisit.Darts = append(p.CurrentVisit.Darts, t)
	p.CurrentVisit.TotalScore += t.Score
	e.state.CurrentDart++

	// Sound for the dart type
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
		p.Score = previousScore + p.CurrentVisit.TotalScore - t.Score // revert all visit darts
		// Actually revert the entire visit score
		p.Score = previousScore
		for _, d := range p.CurrentVisit.Darts[:len(p.CurrentVisit.Darts)-1] {
			p.Score += d.Score
		}
		p.Score = previousScore // simpler: revert to score before this visit
		// Recalculate: revert all darts in this visit
		visitScore := 0
		for _, d := range p.CurrentVisit.Darts {
			visitScore += d.Score
		}
		p.Score = previousScore               // score before the bust dart
		p.Score += t.Score                     // add back the bust dart we just subtracted
		p.Score = previousScore                // actually just reset to start of visit
		// Let me simplify: at start of visit, score was previousScore + all previous darts this visit
		startOfVisitScore := previousScore
		for i := 0; i < len(p.CurrentVisit.Darts)-1; i++ {
			startOfVisitScore += p.CurrentVisit.Darts[i].Score
		}
		p.Score = startOfVisitScore + t.Score // this is wrong direction

		// Simplest approach: recalculate from start-of-visit
		p.Score = e.scoreAtStartOfVisit(p)
		p.CurrentVisit.IsBust = true
		e.state.CurrentDart = 3 // force end of visit
		event = "bust"
		sounds = []string{"bust"} // bust overrides other sounds
		e.finishVisit(p)
		e.updateCheckoutHint()
		return ThrowResult{State: e.state, SoundEvents: sounds, Event: event}
	}

	// Check leg won
	if p.Score == 0 {
		event = "gameshot"
		sounds = []string{"gameshot"}
		p.LegsWon++

		// Check set won
		legsNeeded := (e.state.Options.Legs / 2) + 1
		if p.LegsWon >= legsNeeded {
			p.SetsWon++
			// Check match won
			setsNeeded := (e.state.Options.Sets / 2) + 1
			if p.SetsWon >= setsNeeded {
				event = "matchshot"
				sounds = []string{"matchshot"}
				e.state.WinnerID = p.PlayerID
				e.state.Status = "finished"
				now := time.Now()
				e.state.FinishedAt = &now
			} else {
				e.newSet()
			}
		} else {
			e.newLeg()
		}

		e.state.LastEvent = event
		e.state.SoundEvents = sounds
		e.updateCheckoutHint()
		return ThrowResult{State: e.state, SoundEvents: sounds, Event: event}
	}

	// Check visit complete (3 darts)
	if e.state.CurrentDart >= 3 {
		// Visit score events
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

		// Check hat trick (3 triples or 3 bulls)
		if len(p.CurrentVisit.Darts) == 3 {
			allTriple := true
			allBull := true
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

		e.finishVisit(p)
	}

	e.updateAverage(p)
	e.updateCheckoutHint()

	e.state.LastEvent = event
	e.state.SoundEvents = sounds
	return ThrowResult{State: e.state, SoundEvents: sounds, Event: event}
}

func (e *X01Engine) scoreAtStartOfVisit(p *PlayerState) int {
	// Reconstruct the score at start of current visit from options
	startScore := e.state.Options.StartScore
	if h, ok := e.state.Options.Handicaps[p.PlayerID]; ok {
		startScore -= h
	}

	// Sum all completed visits for this player
	idx := e.playerIndex(p.PlayerID)
	if idx >= 0 && idx < len(e.legThrows) {
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
	if p.Score == 0 {
		return !e.isValidOut(p)
	}
	if p.Score == 1 && e.state.Options.OutMode == "double" {
		return true // can't finish on 1 with double out
	}
	return false
}

func (e *X01Engine) isValidOut(p *PlayerState) bool {
	lastDart := p.CurrentVisit.Darts[len(p.CurrentVisit.Darts)-1]
	switch e.state.Options.OutMode {
	case "double":
		return lastDart.Multiplier == 2
	case "master":
		return lastDart.Multiplier == 2 || lastDart.Multiplier == 3
	default: // straight
		return true
	}
}

func (e *X01Engine) finishVisit(p *PlayerState) {
	idx := e.playerIndex(p.PlayerID)
	if idx >= 0 {
		e.legThrows[idx] = append(e.legThrows[idx], p.CurrentVisit)
	}
	p.CurrentVisit = Visit{}
	e.NextPlayer()
}

func (e *X01Engine) NextPlayer() {
	e.state.Players[e.state.CurrentPlayer].IsActive = false
	e.state.CurrentPlayer = (e.state.CurrentPlayer + 1) % len(e.state.Players)
	e.state.Players[e.state.CurrentPlayer].IsActive = true
	e.state.CurrentDart = 0
}

func (e *X01Engine) newLeg() {
	for i := range e.state.Players {
		startScore := e.state.Options.StartScore
		if h, ok := e.state.Options.Handicaps[e.state.Players[i].PlayerID]; ok {
			startScore -= h
		}
		e.state.Players[i].Score = startScore
		e.state.Players[i].CurrentVisit = Visit{}
		e.state.Players[i].HasStarted = e.state.Options.InMode == "straight"
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

func (e *X01Engine) updateAverage(p *PlayerState) {
	if p.DartsThrown == 0 {
		p.Average = 0
		return
	}
	totalScored := e.state.Options.StartScore - p.Score
	if h, ok := e.state.Options.Handicaps[p.PlayerID]; ok {
		totalScored = (e.state.Options.StartScore - h) - p.Score
	}
	// 3-dart average
	p.Average = float64(totalScored) / float64(p.DartsThrown) * 3
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
	snapshot := e.state
	snapshot.Players = make([]PlayerState, len(e.state.Players))
	copy(snapshot.Players, e.state.Players)
	for i := range snapshot.Players {
		snapshot.Players[i].CurrentVisit.Darts = make([]Throw, len(e.state.Players[i].CurrentVisit.Darts))
		copy(snapshot.Players[i].CurrentVisit.Darts, e.state.Players[i].CurrentVisit.Darts)
	}
	e.history = append(e.history, snapshot)
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
		return ThrowResult{State: e.state, Event: "error"}
	}

	// Save for undo
	e.saveHistory()

	// Replace dart
	oldDart := p.CurrentVisit.Darts[dartIndex]
	p.CurrentVisit.Darts[dartIndex] = newThrow

	// Recalculate visit total
	p.CurrentVisit.TotalScore = 0
	for _, d := range p.CurrentVisit.Darts {
		p.CurrentVisit.TotalScore += d.Score
	}

	// Recalculate player score
	p.Score = e.scoreAtStartOfVisit(p)
	for _, d := range p.CurrentVisit.Darts {
		p.Score -= d.Score
	}

	_ = oldDart
	e.updateAverage(p)
	e.updateCheckoutHint()

	return ThrowResult{State: e.state, SoundEvents: []string{}, Event: fmt.Sprintf("corrected dart %d", dartIndex+1)}
}
