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
	Throw  BoardThrow
	Index  int
}

type TurnEvent struct {
	Status string // "throw" = new turn started, "takeout" = darts being removed
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
}

func NewPoller(client *Client, pollMS int) *Poller {
	return &Poller{
		client:       client,
		pollInterval: time.Duration(pollMS) * time.Millisecond,
		stopCh:       make(chan struct{}),
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

func (p *Poller) loop() {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	errorBackoff := p.pollInterval
	maxBackoff := 5 * time.Second

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			state, err := p.client.GetState()
			if err != nil {
				// Backoff on error
				errorBackoff = min(errorBackoff*2, maxBackoff)
				ticker.Reset(errorBackoff)
				continue
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
	if state.Status != p.lastStatus && p.lastStatus != "" {
		if p.onTurn != nil {
			p.onTurn(TurnEvent{Status: state.Status})
		}
	}
	p.lastStatus = state.Status

	// Hash throws to detect changes
	hash := p.hashThrows(state.Throws)
	if hash == p.lastHash {
		return // no change
	}
	p.lastHash = hash

	// Detect new throws
	if len(state.Throws) > p.lastThrows {
		// New darts detected
		for i := p.lastThrows; i < len(state.Throws); i++ {
			if p.onDart != nil {
				p.onDart(DartEvent{
					Throw: state.Throws[i],
					Index: i,
				})
			}
		}
	} else if len(state.Throws) == 0 && p.lastThrows > 0 {
		// Throws reset (darts removed, new turn)
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

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
