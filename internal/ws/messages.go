package ws

import "encoding/json"

// Message types from server to client
const (
	MsgDart          = "dart"
	MsgState         = "state"
	MsgVisitComplete = "visitComplete"
	MsgSound         = "sound"
	MsgEvent         = "event"
	MsgError         = "error"
	MsgConnected     = "connected"
)

// Message types from client to server
const (
	MsgManualThrow  = "manualThrow"
	MsgCorrect      = "correct"
	MsgUndo         = "undo"
	MsgNextPlayer   = "nextPlayer"
	MsgFinishTakeout = "finishTakeout"
)

type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type ManualThrowData struct {
	Segment string `json:"segment"`
}

type CorrectData struct {
	DartIndex int    `json:"dartIndex"`
	Segment   string `json:"segment"`
}

type SoundData struct {
	Events []string `json:"events"`
}

type EventData struct {
	Event    string `json:"event"`
	PlayerID string `json:"playerId,omitempty"`
}
