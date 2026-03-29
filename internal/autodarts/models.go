package autodarts

import (
	"fmt"
	"strings"
)

// BoardState represents the response from Board Manager /api/state
type BoardState struct {
	Status    string       `json:"status"`
	NumThrows int          `json:"numThrows"`
	Throws    []BoardThrow `json:"throws"`
	Raw       string       `json:"raw,omitempty"` // raw JSON for debugging
}

// BoardThrow represents a single dart throw from the Board Manager
type BoardThrow struct {
	Segment BoardSegment `json:"segment"`
	Coords  BoardCoords  `json:"coords"`
}

// BoardSegment describes the segment hit
type BoardSegment struct {
	Name       string `json:"name"`
	Number     int    `json:"number"`
	Bed        string `json:"bed"`
	Multiplier int    `json:"multiplier"`
}

// BoardCoords represents dart position on the board
type BoardCoords struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ToSegmentString converts a BoardSegment to our internal segment notation
// Handles multiple possible formats from the Board Manager
func (s BoardSegment) ToSegmentString() string {
	// First try: use the Name field directly if it looks like a segment name
	if s.Name != "" {
		upper := strings.ToUpper(strings.TrimSpace(s.Name))
		switch upper {
		case "MISS", "0", "OUT", "OUTSIDE":
			return "MISS"
		case "BULL", "SB", "SINGLE BULL", "25":
			return "BULL"
		case "DBULL", "DB", "DOUBLE BULL", "BULLSEYE", "D25":
			return "DBULL"
		}
		// Check if name starts with T/D/S + number
		if len(upper) >= 2 {
			prefix := upper[0]
			if prefix == 'T' || prefix == 'D' || prefix == 'S' {
				numStr := upper[1:]
				n := 0
				valid := true
				for _, c := range numStr {
					if c >= '0' && c <= '9' {
						n = n*10 + int(c-'0')
					} else {
						valid = false
						break
					}
				}
				if valid && n >= 1 && n <= 20 {
					switch prefix {
					case 'T':
						return fmt.Sprintf("t%d", n)
					case 'D':
						return fmt.Sprintf("d%d", n)
					case 'S':
						return fmt.Sprintf("s%d", n)
					}
				}
			}
		}
	}

	// Second try: use Multiplier + Number
	if s.Number > 0 || s.Multiplier > 0 {
		if s.Number == 25 {
			if s.Multiplier == 2 {
				return "DBULL"
			}
			return "BULL"
		}
		if s.Number >= 1 && s.Number <= 20 {
			switch s.Multiplier {
			case 3:
				return fmt.Sprintf("t%d", s.Number)
			case 2:
				return fmt.Sprintf("d%d", s.Number)
			case 1:
				return fmt.Sprintf("s%d", s.Number)
			}
		}
	}

	// Third try: use Bed + Number
	if s.Bed != "" && s.Number > 0 {
		bed := strings.ToLower(strings.TrimSpace(s.Bed))
		if s.Number == 25 {
			if strings.Contains(bed, "double") || strings.Contains(bed, "inner") {
				return "DBULL"
			}
			return "BULL"
		}
		switch {
		case strings.Contains(bed, "triple") || strings.Contains(bed, "treble"):
			return fmt.Sprintf("t%d", s.Number)
		case strings.Contains(bed, "double"):
			return fmt.Sprintf("d%d", s.Number)
		case strings.Contains(bed, "single") || strings.Contains(bed, "outer") || strings.Contains(bed, "inner"):
			return fmt.Sprintf("s%d", s.Number)
		}
	}

	return "MISS"
}
