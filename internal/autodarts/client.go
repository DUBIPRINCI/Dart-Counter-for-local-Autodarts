package autodarts

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(host string, port int) *Client {
	return &Client{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

// GetRawState returns the raw JSON from the Board Manager (for debug)
func (c *Client) GetRawState() ([]byte, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/api/state")
	if err != nil {
		return nil, fmt.Errorf("request board state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("board manager returned %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// GetState fetches and parses Board Manager state, handling unknown JSON structures
func (c *Client) GetState() (*BoardState, error) {
	body, err := c.GetRawState()
	if err != nil {
		return nil, err
	}

	// First, parse as generic map to understand the structure
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse board state: %w", err)
	}

	state := &BoardState{
		Raw: string(body),
	}

	// Extract status
	if v, ok := raw["status"].(string); ok {
		state.Status = v
	}

	// Try multiple possible field names for throws
	var throwsRaw []interface{}
	for _, key := range []string{"throws", "darts", "segments", "points", "detections"} {
		if v, ok := raw[key].([]interface{}); ok {
			throwsRaw = v
			break
		}
	}

	// Also try nested paths like "game.throws" or "board.throws"
	if throwsRaw == nil {
		for _, parent := range []string{"game", "board", "detection", "data"} {
			if obj, ok := raw[parent].(map[string]interface{}); ok {
				for _, key := range []string{"throws", "darts", "segments"} {
					if v, ok := obj[key].([]interface{}); ok {
						throwsRaw = v
						break
					}
				}
			}
		}
	}

	state.NumThrows = len(throwsRaw)

	// Parse each throw flexibly
	for _, tr := range throwsRaw {
		throwMap, ok := tr.(map[string]interface{})
		if !ok {
			continue
		}
		bt := parseThrowFlexible(throwMap)
		state.Throws = append(state.Throws, bt)
	}

	return state, nil
}

// parseThrowFlexible handles various possible JSON structures for a dart throw
func parseThrowFlexible(m map[string]interface{}) BoardThrow {
	bt := BoardThrow{}

	// Try to find segment info - it might be nested under "segment" or flat
	segMap := m
	if sub, ok := m["segment"].(map[string]interface{}); ok {
		segMap = sub
	}

	// Extract name (e.g. "T20", "S5", "D16", "BULL", "MISS", "25", "SB", "DB")
	if name, ok := segMap["name"].(string); ok {
		bt.Segment.Name = name
	}

	// Extract number
	if n, ok := segMap["number"].(float64); ok {
		bt.Segment.Number = int(n)
	}

	// Extract multiplier
	if m, ok := segMap["multiplier"].(float64); ok {
		bt.Segment.Multiplier = int(m)
	}

	// Extract bed
	if bed, ok := segMap["bed"].(string); ok {
		bt.Segment.Bed = bed
	}

	// If we have a name but missing multiplier/number, parse from name
	if bt.Segment.Name != "" && (bt.Segment.Multiplier == 0 && bt.Segment.Number == 0) {
		bt.Segment.Number, bt.Segment.Multiplier = parseSegmentName(bt.Segment.Name)
	}

	// If we have multiplier and number but no name, build name
	if bt.Segment.Name == "" && bt.Segment.Number > 0 {
		switch bt.Segment.Multiplier {
		case 3:
			bt.Segment.Name = fmt.Sprintf("T%d", bt.Segment.Number)
		case 2:
			bt.Segment.Name = fmt.Sprintf("D%d", bt.Segment.Number)
		default:
			bt.Segment.Name = fmt.Sprintf("S%d", bt.Segment.Number)
		}
	}

	// Extract coords
	if coords, ok := m["coords"].(map[string]interface{}); ok {
		if x, ok := coords["x"].(float64); ok {
			bt.Coords.X = x
		}
		if y, ok := coords["y"].(float64); ok {
			bt.Coords.Y = y
		}
	}
	// Also try flat x/y
	if x, ok := m["x"].(float64); ok {
		bt.Coords.X = x
	}
	if y, ok := m["y"].(float64); ok {
		bt.Coords.Y = y
	}

	return bt
}

// parseSegmentName extracts number and multiplier from segment name like "T20", "S5", "D16", "BULL", "DB", "SB"
func parseSegmentName(name string) (number int, multiplier int) {
	name = strings.ToUpper(strings.TrimSpace(name))

	switch name {
	case "MISS", "0", "OUT", "OUTSIDE":
		return 0, 0
	case "BULL", "SB", "SINGLE BULL", "25":
		return 25, 1
	case "DBULL", "DB", "DOUBLE BULL", "BULLSEYE", "D25":
		return 25, 2
	}

	if len(name) < 2 {
		return 0, 0
	}

	prefix := name[0]
	numStr := name[1:]

	n := 0
	for _, c := range numStr {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}

	if n == 0 {
		return 0, 0
	}

	switch prefix {
	case 'T':
		return n, 3
	case 'D':
		return n, 2
	case 'S':
		return n, 1
	default:
		// Maybe it's just a number like "20"
		fullNum := 0
		for _, c := range name {
			if c >= '0' && c <= '9' {
				fullNum = fullNum*10 + int(c-'0')
			} else {
				return 0, 0
			}
		}
		if fullNum > 0 && fullNum <= 25 {
			return fullNum, 1
		}
		return 0, 0
	}
}

func (c *Client) IsConnected() bool {
	resp, err := c.httpClient.Get(c.baseURL + "/api/version")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

// ListEndpoints tries common Board Manager endpoints and returns which ones respond
func (c *Client) ListEndpoints() map[string]string {
	endpoints := map[string]string{}
	paths := []string{
		"/api/state",
		"/api/config",
		"/api/host",
		"/api/version",
		"/api/board",
		"/api/game",
		"/api/status",
		"/api/detection",
		"/api/throws",
	}

	for _, path := range paths {
		resp, err := c.httpClient.Get(c.baseURL + path)
		if err != nil {
			endpoints[path] = "error: " + err.Error()
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			// Truncate to 500 chars
			s := string(body)
			if len(s) > 500 {
				s = s[:500] + "..."
			}
			endpoints[path] = s
		} else {
			endpoints[path] = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	}

	log.Printf("Board Manager endpoints scan: %+v", endpoints)
	return endpoints
}
