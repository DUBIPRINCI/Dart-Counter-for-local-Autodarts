package game

import "time"

// Throw represents a single dart throw
type Throw struct {
	Segment    string  `json:"segment"`    // e.g. "s20", "d20", "t20", "BULL", "DBULL", "MISS"
	Number     int     `json:"number"`     // 0-20, 25
	Multiplier int     `json:"multiplier"` // 0 (miss), 1, 2, 3
	Score      int     `json:"score"`      // number * multiplier
	X          float64 `json:"x,omitempty"`
	Y          float64 `json:"y,omitempty"`
	IsManual   bool    `json:"isManual"`
}

// Visit represents a player's turn (up to 3 darts)
type Visit struct {
	Darts      []Throw `json:"darts"`
	TotalScore int     `json:"totalScore"`
	IsBust     bool    `json:"isBust"`
}

// PlayerState holds the live state of a player in a game
type PlayerState struct {
	PlayerID    string  `json:"playerId"`
	PlayerName  string  `json:"playerName"`
	Score       int     `json:"score"`       // remaining for X01, earned for others
	SetsWon     int     `json:"setsWon"`
	LegsWon     int     `json:"legsWon"`
	DartsThrown int     `json:"dartsThrown"`
	Average     float64 `json:"average"`
	CurrentVisit Visit  `json:"currentVisit"`
	IsActive    bool    `json:"isActive"`
	HasStarted  bool    `json:"hasStarted"` // for double-in
}

// GameState holds the full live state of a game
type GameState struct {
	ID              string        `json:"id"`
	GameType        string        `json:"gameType"`
	Variant         string        `json:"variant"`
	Options         GameOptions   `json:"options"`
	Status          string        `json:"status"` // "setup", "active", "finished", "abandoned"
	Players         []PlayerState `json:"players"`
	CurrentPlayer   int           `json:"currentPlayer"`
	CurrentDart     int           `json:"currentDart"` // 0, 1, 2
	CurrentSet      int           `json:"currentSet"`
	CurrentLeg      int           `json:"currentLeg"`
	WinnerID        string        `json:"winnerId,omitempty"`
	CheckoutHint    string        `json:"checkoutHint,omitempty"`
	LastEvent       string        `json:"lastEvent,omitempty"`
	SoundEvents     []string      `json:"soundEvents,omitempty"`
	// WaitingTakeout = true : visit complete, darts still on board, waiting for removal
	// The current player's visit darts stay visible until FinishTakeout() is called
	WaitingTakeout  bool          `json:"waitingTakeout"`
	StartedAt       *time.Time    `json:"startedAt,omitempty"`
	FinishedAt      *time.Time    `json:"finishedAt,omitempty"`
}

// GameOptions configures a game
type GameOptions struct {
	GameType    string            `json:"gameType"`    // "x01", "cricket", "atc", "shanghai", "highscore"
	Variant     string            `json:"variant"`     // "501", "301", "standard", "cut-throat", etc.
	StartScore  int               `json:"startScore"`  // for X01
	InMode      string            `json:"inMode"`      // "straight", "double"
	OutMode     string            `json:"outMode"`     // "straight", "double", "master"
	Sets        int               `json:"sets"`        // best of N sets
	Legs        int               `json:"legs"`        // best of N legs per set
	PlayerIDs   []string          `json:"playerIds"`
	PlayerNames []string          `json:"playerNames"`
	Handicaps   map[string]int    `json:"handicaps,omitempty"`
	// ATC options
	DoubleSkip  bool              `json:"doubleSkip,omitempty"`  // double advances +2
	TripleSkip  bool              `json:"tripleSkip,omitempty"`  // triple advances +3
	// Shanghai options
	Rounds      int               `json:"rounds,omitempty"`      // number of rounds
	// Cricket options
	CricketMode string            `json:"cricketMode,omitempty"` // "standard", "cutthroat", "noscore"
}

// ThrowResult is returned by the engine after processing a throw
type ThrowResult struct {
	State       GameState `json:"state"`
	SoundEvents []string  `json:"soundEvents"`
	Event       string    `json:"event,omitempty"` // "bust", "gameshot", "matchshot", "180", etc.
}

// ParseSegment converts a segment string to number and multiplier
func ParseSegment(seg string) (number int, multiplier int) {
	switch seg {
	case "MISS":
		return 0, 0
	case "BULL":
		return 25, 1
	case "DBULL":
		return 25, 2
	default:
		if len(seg) < 2 {
			return 0, 0
		}
		prefix := seg[0]
		numStr := seg[1:]
		n := 0
		for _, c := range numStr {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			}
		}
		switch prefix {
		case 's', 'S':
			return n, 1
		case 'd', 'D':
			return n, 2
		case 't', 'T':
			return n, 3
		}
		return 0, 0
	}
}

// NewThrow creates a Throw from a segment string
func NewThrow(segment string) Throw {
	number, multiplier := ParseSegment(segment)
	return Throw{
		Segment:    segment,
		Number:     number,
		Multiplier: multiplier,
		Score:      number * multiplier,
	}
}

// NewThrowWithCoords creates a Throw with coordinates
func NewThrowWithCoords(segment string, x, y float64) Throw {
	t := NewThrow(segment)
	t.X = x
	t.Y = y
	return t
}
