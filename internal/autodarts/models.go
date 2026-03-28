package autodarts

// BoardState represents the response from Board Manager /api/state
type BoardState struct {
	Status    string       `json:"status"`    // "Throw", "Takeout"
	NumThrows int          `json:"numThrows"`
	Throws    []BoardThrow `json:"throws"`
}

// BoardThrow represents a single dart throw from the Board Manager
type BoardThrow struct {
	Segment BoardSegment `json:"segment"`
	Coords  BoardCoords  `json:"coords"`
}

// BoardSegment describes the segment hit
type BoardSegment struct {
	Name       string `json:"name"`       // "T20", "S5", "D16", "BULL", "MISS"
	Number     int    `json:"number"`     // 0-20, 25
	Bed        string `json:"bed"`        // "Triple", "Double", "Single", "Bull", "Miss"
	Multiplier int    `json:"multiplier"` // 0, 1, 2, 3
}

// BoardCoords represents dart position on the board
type BoardCoords struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ToSegmentString converts a BoardSegment to our internal segment notation
func (s BoardSegment) ToSegmentString() string {
	switch s.Bed {
	case "Triple":
		return "t" + itoa(s.Number)
	case "Double":
		if s.Number == 25 {
			return "DBULL"
		}
		return "d" + itoa(s.Number)
	case "Single":
		if s.Number == 25 {
			return "BULL"
		}
		return "s" + itoa(s.Number)
	case "Bull":
		return "BULL"
	default:
		return "MISS"
	}
}

func itoa(n int) string {
	if n < 0 {
		return "0"
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
