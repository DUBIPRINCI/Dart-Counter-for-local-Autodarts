package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"sync"

	"dartcounter/internal/autodarts"
	"dartcounter/internal/config"
	"dartcounter/internal/game"
	"dartcounter/internal/sound"
	"dartcounter/internal/storage"
	"dartcounter/internal/ws"
)

type Server struct {
	cfg        *config.Config
	db         *storage.DB
	hub        *ws.Hub
	poller     *autodarts.Poller
	bmClient   *autodarts.Client
	engine     game.Engine
	engineMu   sync.RWMutex
	mux        *http.ServeMux
	webFS      fs.FS
}

func New(cfg *config.Config, db *storage.DB, webFS fs.FS) *Server {
	hub := ws.NewHub()
	bmClient := autodarts.NewClient(cfg.BMHost, cfg.BMPort)
	poller := autodarts.NewPoller(bmClient, cfg.PollMS)

	s := &Server{
		cfg:      cfg,
		db:       db,
		hub:      hub,
		poller:   poller,
		bmClient: bmClient,
		mux:      http.NewServeMux(),
		webFS:    webFS,
	}

	s.setupRoutes()
	s.setupPoller()

	return s
}

func (s *Server) Start() error {
	go s.hub.Run()
	s.poller.Start()

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("DartCounter running on http://localhost%s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) setupPoller() {
	s.poller.OnDart(func(evt autodarts.DartEvent) {
		s.engineMu.RLock()
		eng := s.engine
		s.engineMu.RUnlock()

		if eng == nil {
			return
		}

		segment := evt.Throw.Segment.ToSegmentString()
		t := game.NewThrowWithCoords(segment, evt.Throw.Coords.X, evt.Throw.Coords.Y)

		result := eng.ProcessThrow(t)

		s.hub.Broadcast(ws.MsgState, result.State)
		if len(result.SoundEvents) > 0 {
			s.hub.Broadcast(ws.MsgSound, ws.SoundData{Events: result.SoundEvents})
		}
		if result.Event != "" {
			s.hub.Broadcast(ws.MsgEvent, ws.EventData{Event: result.Event})
		}

		// Persist game if finished
		if result.State.Status == "finished" {
			winnerID := result.State.WinnerID
			s.db.UpdateGameStatus(eng.GetID(), "finished", &winnerID)
		}
	})

	s.poller.OnTurn(func(evt autodarts.TurnEvent) {
		s.hub.Broadcast(ws.MsgEvent, ws.EventData{Event: "turn:" + evt.Status})
	})
}

func (s *Server) setupRoutes() {
	// WebSocket
	s.mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWs(s.hub, w, r, func(msg ws.Message) {
			s.handleWSMessage(msg)
		})
	})

	// API routes
	s.mux.HandleFunc("POST /api/games", s.handleCreateGame)
	s.mux.HandleFunc("GET /api/games/current", s.handleGetCurrentGame)
	s.mux.HandleFunc("DELETE /api/games/current", s.handleAbandonGame)
	s.mux.HandleFunc("POST /api/games/current/throw", s.handleManualThrow)
	s.mux.HandleFunc("POST /api/games/current/undo", s.handleUndo)
	s.mux.HandleFunc("POST /api/games/current/correct", s.handleCorrect)
	s.mux.HandleFunc("POST /api/games/current/next", s.handleNextPlayer)

	// Players
	s.mux.HandleFunc("GET /api/players", s.handleListPlayers)
	s.mux.HandleFunc("POST /api/players", s.handleCreatePlayer)
	s.mux.HandleFunc("PUT /api/players/{id}", s.handleUpdatePlayer)
	s.mux.HandleFunc("DELETE /api/players/{id}", s.handleDeletePlayer)

	// History
	s.mux.HandleFunc("GET /api/history", s.handleListGames)

	// Sounds
	s.mux.HandleFunc("GET /api/sounds/packs", s.handleListSoundPacks)

	// Autodarts status
	s.mux.HandleFunc("GET /api/autodarts/status", s.handleAutodartsStatus)

	// Sound files
	soundFS := http.FileServer(http.Dir(s.cfg.SoundsDir))
	s.mux.Handle("/sounds/", http.StripPrefix("/sounds/", soundFS))

	// Static files (SPA)
	if s.webFS != nil {
		fileServer := http.FileServerFS(s.webFS)
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Try to serve the file; fall back to index.html for SPA routing
			path := r.URL.Path
			if path == "/" {
				path = "/index.html"
			}
			// Check if file exists
			if f, err := s.webFS.Open(path[1:]); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
			// SPA fallback
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
		})
	}
}

func (s *Server) handleWSMessage(msg ws.Message) {
	switch msg.Type {
	case ws.MsgManualThrow:
		var data ws.ManualThrowData
		json.Unmarshal(msg.Data, &data)
		s.processManualThrow(data.Segment)

	case ws.MsgCorrect:
		var data ws.CorrectData
		json.Unmarshal(msg.Data, &data)
		s.processCorrection(data.DartIndex, data.Segment)

	case ws.MsgUndo:
		s.processUndo()

	case ws.MsgNextPlayer:
		s.processNextPlayer()
	}
}

func (s *Server) processManualThrow(segment string) {
	s.engineMu.RLock()
	eng := s.engine
	s.engineMu.RUnlock()

	if eng == nil {
		return
	}

	t := game.NewThrow(segment)
	t.IsManual = true
	result := eng.ProcessThrow(t)

	s.hub.Broadcast(ws.MsgState, result.State)
	if len(result.SoundEvents) > 0 {
		s.hub.Broadcast(ws.MsgSound, ws.SoundData{Events: result.SoundEvents})
	}
	if result.Event != "" {
		s.hub.Broadcast(ws.MsgEvent, ws.EventData{Event: result.Event})
	}

	if result.State.Status == "finished" {
		winnerID := result.State.WinnerID
		s.db.UpdateGameStatus(eng.GetID(), "finished", &winnerID)
	}
}

func (s *Server) processCorrection(dartIndex int, segment string) {
	s.engineMu.RLock()
	eng := s.engine
	s.engineMu.RUnlock()

	if eng == nil {
		return
	}

	if x01, ok := eng.(*game.X01Engine); ok {
		t := game.NewThrow(segment)
		result := x01.CorrectThrow(dartIndex, t)
		s.hub.Broadcast(ws.MsgState, result.State)
	}
}

func (s *Server) processUndo() {
	s.engineMu.RLock()
	eng := s.engine
	s.engineMu.RUnlock()

	if eng == nil {
		return
	}

	state := eng.Undo()
	s.hub.Broadcast(ws.MsgState, state)
}

func (s *Server) processNextPlayer() {
	s.engineMu.RLock()
	eng := s.engine
	s.engineMu.RUnlock()

	if eng == nil {
		return
	}

	eng.NextPlayer()
	s.hub.Broadcast(ws.MsgState, eng.State())
}

// JSON helpers
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Route handlers
func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	var opts game.GameOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		writeError(w, http.StatusBadRequest, "invalid game options")
		return
	}

	eng := game.NewEngine(opts)

	s.engineMu.Lock()
	s.engine = eng
	s.engineMu.Unlock()

	state := eng.State()

	// Persist
	s.db.CreateGame(state.ID, state.GameType, state.Variant, opts)

	s.hub.Broadcast(ws.MsgState, state)
	s.hub.Broadcast(ws.MsgSound, ws.SoundData{Events: []string{"gameon"}})

	writeJSON(w, http.StatusCreated, state)
}

func (s *Server) handleGetCurrentGame(w http.ResponseWriter, r *http.Request) {
	s.engineMu.RLock()
	eng := s.engine
	s.engineMu.RUnlock()

	if eng == nil {
		writeError(w, http.StatusNotFound, "no active game")
		return
	}
	writeJSON(w, http.StatusOK, eng.State())
}

func (s *Server) handleAbandonGame(w http.ResponseWriter, r *http.Request) {
	s.engineMu.Lock()
	eng := s.engine
	s.engine = nil
	s.engineMu.Unlock()

	if eng != nil {
		s.db.UpdateGameStatus(eng.GetID(), "abandoned", nil)
	}

	s.hub.Broadcast(ws.MsgEvent, ws.EventData{Event: "gameAbandoned"})
	writeJSON(w, http.StatusOK, map[string]string{"status": "abandoned"})
}

func (s *Server) handleManualThrow(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Segment string `json:"segment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "invalid throw data")
		return
	}
	s.processManualThrow(data.Segment)
	s.engineMu.RLock()
	eng := s.engine
	s.engineMu.RUnlock()
	if eng != nil {
		writeJSON(w, http.StatusOK, eng.State())
	}
}

func (s *Server) handleUndo(w http.ResponseWriter, r *http.Request) {
	s.processUndo()
	s.engineMu.RLock()
	eng := s.engine
	s.engineMu.RUnlock()
	if eng != nil {
		writeJSON(w, http.StatusOK, eng.State())
	}
}

func (s *Server) handleCorrect(w http.ResponseWriter, r *http.Request) {
	var data ws.CorrectData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "invalid correction data")
		return
	}
	s.processCorrection(data.DartIndex, data.Segment)
	s.engineMu.RLock()
	eng := s.engine
	s.engineMu.RUnlock()
	if eng != nil {
		writeJSON(w, http.StatusOK, eng.State())
	}
}

func (s *Server) handleNextPlayer(w http.ResponseWriter, r *http.Request) {
	s.processNextPlayer()
	s.engineMu.RLock()
	eng := s.engine
	s.engineMu.RUnlock()
	if eng != nil {
		writeJSON(w, http.StatusOK, eng.State())
	}
}

func (s *Server) handleListPlayers(w http.ResponseWriter, r *http.Request) {
	players, err := s.db.ListPlayers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if players == nil {
		players = []storage.Player{}
	}
	writeJSON(w, http.StatusOK, players)
}

func (s *Server) handleCreatePlayer(w http.ResponseWriter, r *http.Request) {
	var data struct {
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "invalid player data")
		return
	}
	if data.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	player, err := s.db.CreatePlayer(data.Name, data.Avatar)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, player)
}

func (s *Server) handleUpdatePlayer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var data struct {
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeError(w, http.StatusBadRequest, "invalid player data")
		return
	}
	if err := s.db.UpdatePlayer(id, data.Name, data.Avatar); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleDeletePlayer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.db.DeletePlayer(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListGames(w http.ResponseWriter, r *http.Request) {
	games, err := s.db.ListGames(50, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if games == nil {
		games = []storage.GameRecord{}
	}
	writeJSON(w, http.StatusOK, games)
}

func (s *Server) handleListSoundPacks(w http.ResponseWriter, r *http.Request) {
	packs, err := sound.ListPacks(s.cfg.SoundsDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, packs)
}

func (s *Server) handleAutodartsStatus(w http.ResponseWriter, r *http.Request) {
	connected := s.bmClient.IsConnected()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"connected": connected,
		"host":      s.cfg.BMHost,
		"port":      s.cfg.BMPort,
		"polling":   s.poller.IsRunning(),
	})
}
