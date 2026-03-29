package autodarts

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

type DartEvent struct {
	Throw BoardThrow
	Index int
}

type TurnEvent struct {
	Status string
}

type Poller struct {
	client       *Client
	pollInterval time.Duration
	onDart       func(DartEvent)
	onTurn       func(TurnEvent)
	lastHash     string
	lastThrows   int
	lastStatus   string
	running      bool
	mu           sync.Mutex
	stopCh       chan struct{}
	debugLog     bool
}

func NewPoller(client *Client, pollMS int) *Poller {
	return &Poller{
		client:       client,
		pollInterval: time.Duration(pollMS) * time.Millisecond,
		stopCh:       make(chan struct{}),
		debugLog:     true, // Enable debug logging by default for now
	}
}

func (p *Poller) OnDart(fn func(DartEvent)) {
	p.onDart = fn
}

func (p *Poller) OnTurn(fn func(TurnEvent)) {
	p.onTurn = fn
}

func (p *Poller) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	go p.loop()
	log.Printf("Autodarts poller started (interval: %v)", p.pollInterval)
}

func (p *Poller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	p.running = false
	close(p.stopCh)
	log.Println("Autodarts poller stopped")
}

func (p *Poller) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *Poller) SetDebug(on bool) {
	p.debugLog = on
}

func (p *Poller) loop() {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	errorBackoff := p.pollInterval
	maxBackoff := 5 * time.Second
	loggedFirstSuccess := false

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			state, err := p.client.GetState()
			if err != nil {
				errorBackoff = minDuration(errorBackoff*2, maxBackoff)
				ticker.Reset(errorBackoff)
				continue
			}

			// Log first successful connection and raw JSON
			if !loggedFirstSuccess {
				loggedFirstSuccess = true
				log.Printf("[AUTODARTS] Connected! Raw state: %s", state.Raw)
			}

			// Reset to normal interval on success
			if errorBackoff != p.pollInterval {
				errorBackoff = p.pollInterval
				ticker.Reset(p.pollInterval)
			}

			p.processState(state)
		}
	}
}

func (p *Poller) processState(state *BoardState) {
	// Check status change
	if state.Status != p.lastStatus {
		if p.lastStatus != "" {
			if p.debugLog {
				log.Printf("[AUTODARTS] Status changed: %q -> %q", p.lastStatus, state.Status)
			}
			if p.onTurn != nil {
				p.onTurn(TurnEvent{Status: state.Status})
			}
		}
		p.lastStatus = state.Status
	}

	// Hash throws to detect changes
	hash := p.hashThrows(state.Throws)
	if hash == p.lastHash {
		return
	}

	if p.debugLog {
		log.Printf("[AUTODARTS] Throws changed: count %d -> %d, hash %s -> %s",
			p.lastThrows, len(state.Throws), p.lastHash, hash)
		for i, t := range state.Throws {
			seg := t.Segment.ToSegmentString()
			log.Printf("[AUTODARTS]   throw[%d]: name=%q number=%d multiplier=%d bed=%q -> segment=%q coords=(%.2f, %.2f)",
				i, t.Segment.Name, t.Segment.Number, t.Segment.Multiplier, t.Segment.Bed, seg, t.Coords.X, t.Coords.Y)
		}
	}

	p.lastHash = hash

	// Detect new throws
	if len(state.Throws) > p.lastThrows {
		for i := p.lastThrows; i < len(state.Throws); i++ {
			seg := state.Throws[i].Segment.ToSegmentString()
			log.Printf("[AUTODARTS] NEW DART #%d: %s (name=%q num=%d mult=%d bed=%q)",
				i, seg, state.Throws[i].Segment.Name, state.Throws[i].Segment.Number,
				state.Throws[i].Segment.Multiplier, state.Throws[i].Segment.Bed)

			if p.onDart != nil {
				p.onDart(DartEvent{
					Throw: state.Throws[i],
					Index: i,
				})
			}
		}
	} else if len(state.Throws) == 0 && p.lastThrows > 0 {
		log.Printf("[AUTODARTS] Throws reset (darts removed)")
		if p.onTurn != nil {
			p.onTurn(TurnEvent{Status: "newTurn"})
		}
	}

	p.lastThrows = len(state.Throws)
}

func (p *Poller) hashThrows(throws []BoardThrow) string {
	if len(throws) == 0 {
		return ""
	}
	data, _ := json.Marshal(throws)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
