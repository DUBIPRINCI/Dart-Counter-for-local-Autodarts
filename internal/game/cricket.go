package game

import (
	"time"

	"github.com/google/uuid"
)

var cricketNumbers = []int{15, 16, 17, 18, 19, 20, 25}

type CricketPlayerState struct {
	Marks map[int]int `json:"marks"` // number -> marks count (0-3+)
}

type CricketEngine struct {
	state        GameState
	cricketState []CricketPlayerState
	history      []cricketSnapshot
}

type cricketSnapshot struct {
	state        GameState
	cricketState []CricketPlayerState
}

func NewCricketEngine(opts GameOptions) *CricketEngine {
	if opts.CricketMode == "" {
		opts.CricketMode = "standard"
	}
	if opts.Sets == 0 {
		opts.Sets = 1
	}
	if opts.Legs == 0 {
		opts.Legs = 1
	}

	players := make([]PlayerState, len(opts.PlayerIDs))
	cricketState := make([]CricketPlayerState, len(opts.PlayerIDs))

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
		cricketState[i] = CricketPlayerState{
			Marks: make(map[int]int),
		}
	}

	now := time.Now()
	return &CricketEngine{
		state: GameState{
			ID:            uuid.New().String(),
			GameType:      "cricket",
			Variant:       opts.CricketMode,
			Options:       opts,
			Status:        "active",
			Players:       players,
			CurrentPlayer: 0,
			CurrentDart:   0,
			CurrentSet:    1,
			CurrentLeg:    1,
			StartedAt:     &now,
		},
		cricketState: cricketState,
	}
}

func (e *CricketEngine) GetID() string           { return e.state.ID }
func (e *CricketEngine) State() *GameState        { s := e.state; return &s }
func (e *CricketEngine) CheckoutHint(int) string  { return "" }
func (e *CricketEngine) IsVisitComplete() bool     { return e.state.CurrentDart >= 3 }

func (e *CricketEngine) ProcessThrow(t Throw) ThrowResult {
	e.saveHistory()

	p := &e.state.Players[e.state.CurrentPlayer]
	cs := &e.cricketState[e.state.CurrentPlayer]
	sounds := []string{"throw"}

	p.CurrentVisit.Darts = append(p.CurrentVisit.Darts, t)
	p.DartsThrown++
	e.state.CurrentDart++

	// Check if the number is a cricket number
	if isCricketNumber(t.Number) {
		marks := t.Multiplier
		currentMarks := cs.Marks[t.Number]

		if currentMarks < 3 {
			marksToClose := 3 - currentMarks
			if marks <= marksToClose {
				cs.Marks[t.Number] += marks
			} else {
				cs.Marks[t.Number] = 3
				extraMarks := marks - marksToClose
				e.scoreExtraMarks(t.Number, extraMarks, p)
			}
		} else {
			// Already closed, score points
			e.scoreExtraMarks(t.Number, marks, p)
		}

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

	// Check win
	if e.checkCricketWin() {
		winner := e.findCricketWinner()
		if winner >= 0 {
			e.state.WinnerID = e.state.Players[winner].PlayerID
			e.state.Status = "finished"
			now := time.Now()
			e.state.FinishedAt = &now
			sounds = []string{"gameshot"}
			return ThrowResult{State: e.state, SoundEvents: sounds, Event: "gameshot"}
		}
	}

	if e.state.CurrentDart >= 3 {
		p.CurrentVisit.TotalScore = e.visitScore(p)
		p.CurrentVisit = Visit{}
		e.NextPlayer()
	}

	return ThrowResult{State: e.state, SoundEvents: sounds}
}

func (e *CricketEngine) scoreExtraMarks(number, marks int, scorer *PlayerState) {
	mode := e.state.Options.CricketMode

	// Check if all opponents have closed this number
	allClosed := true
	for i, cs := range e.cricketState {
		if e.state.Players[i].PlayerID == scorer.PlayerID {
			continue
		}
		if cs.Marks[number] < 3 {
			allClosed = false
			break
		}
	}

	if allClosed {
		return // no one to score against
	}

	pointValue := number * marks

	switch mode {
	case "standard":
		scorer.Score += pointValue
	case "cutthroat":
		for i := range e.state.Players {
			if e.state.Players[i].PlayerID == scorer.PlayerID {
				continue
			}
			if e.cricketState[i].Marks[number] < 3 {
				e.state.Players[i].Score += pointValue
			}
		}
	case "noscore":
		// no scoring
	}
}

func (e *CricketEngine) checkCricketWin() bool {
	for i, cs := range e.cricketState {
		allClosed := true
		for _, n := range cricketNumbers {
			if cs.Marks[n] < 3 {
				allClosed = false
				break
			}
		}
		if allClosed {
			mode := e.state.Options.CricketMode
			switch mode {
			case "standard":
				// Must have highest score
				isHighest := true
				for j, op := range e.state.Players {
					if j != i && op.Score >= e.state.Players[i].Score {
						isHighest = false
						break
					}
				}
				if isHighest {
					return true
				}
			case "cutthroat":
				// Must have lowest score
				isLowest := true
				for j, op := range e.state.Players {
					if j != i && op.Score <= e.state.Players[i].Score {
						isLowest = false
						break
					}
				}
				if isLowest {
					return true
				}
			case "noscore":
				return true
			}
		}
	}
	return false
}

func (e *CricketEngine) findCricketWinner() int {
	for i, cs := range e.cricketState {
		allClosed := true
		for _, n := range cricketNumbers {
			if cs.Marks[n] < 3 {
				allClosed = false
				break
			}
		}
		if allClosed {
			return i
		}
	}
	return -1
}

func (e *CricketEngine) visitScore(p *PlayerState) int {
	total := 0
	for _, d := range p.CurrentVisit.Darts {
		total += d.Score
	}
	return total
}

func (e *CricketEngine) NextPlayer() {
	e.state.Players[e.state.CurrentPlayer].IsActive = false
	e.state.CurrentPlayer = (e.state.CurrentPlayer + 1) % len(e.state.Players)
	e.state.Players[e.state.CurrentPlayer].IsActive = true
	e.state.CurrentDart = 0
}

func (e *CricketEngine) saveHistory() {
	snap := cricketSnapshot{state: e.state}
	snap.state.Players = make([]PlayerState, len(e.state.Players))
	copy(snap.state.Players, e.state.Players)
	snap.cricketState = make([]CricketPlayerState, len(e.cricketState))
	for i, cs := range e.cricketState {
		snap.cricketState[i] = CricketPlayerState{Marks: make(map[int]int)}
		for k, v := range cs.Marks {
			snap.cricketState[i].Marks[k] = v
		}
	}
	e.history = append(e.history, snap)
}

func (e *CricketEngine) Undo() *GameState {
	if len(e.history) == 0 {
		return e.State()
	}
	snap := e.history[len(e.history)-1]
	e.state = snap.state
	e.cricketState = snap.cricketState
	e.history = e.history[:len(e.history)-1]
	return e.State()
}

func isCricketNumber(n int) bool {
	for _, cn := range cricketNumbers {
		if cn == n {
			return true
		}
	}
	return false
}
