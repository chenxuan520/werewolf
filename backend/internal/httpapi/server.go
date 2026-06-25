package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"werewolf/backend/internal/game"
)

type Server struct {
	service *game.Service
}

func NewServer(service *game.Service) *Server {
	return &Server{service: service}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/templates", s.handleTemplates)
	mux.HandleFunc("/api/presets", s.handlePresets)
	mux.HandleFunc("/api/games", s.handleGames)
	mux.HandleFunc("/api/games/", s.handleGameByID)
	mux.HandleFunc("/api/records", s.handleRecords)
	mux.HandleFunc("/api/replays/", s.handleReplayByID)
	return withCORS(mux)
}

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": s.service.ListTemplates()})
}

func (s *Server) handlePresets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"presets": s.service.ListPresets()})
}

func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var req game.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
		return
	}
	snapshot, err := s.service.CreateGame(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, snapshot)
}

func (s *Server) handleGameByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/games/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}
	id := parts[0]
	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		snapshot, ok := s.service.GetGame(id)
		if !ok {
			writeError(w, http.StatusNotFound, "game not found")
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
		return
	}
	switch parts[1] {
	case "actions":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		var req game.ActionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
			return
		}
		snapshot, err := s.service.Act(id, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	case "control":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		var req game.ControlRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
			return
		}
		snapshot, err := s.service.ControlGame(id, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	case "stream":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleStream(w, r, id)
	default:
		writeError(w, http.StatusNotFound, "game route not found")
	}
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request, id string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream unsupported")
		return
	}
	includePrivate := strings.TrimSpace(r.URL.Query().Get("mode")) == "spectator"
	ch, unsubscribe, err := s.service.Subscribe(id, includePrivate)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	defer unsubscribe()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	writeSSE(w, "ready", map[string]any{"ok": true})
	flusher.Flush()
	keepAlive := time.NewTicker(20 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, event.Type, event)
			flusher.Flush()
		case <-keepAlive.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"records": s.service.ListRecords()})
}

func (s *Server) handleReplayByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/replays/"), "/")
	if id == "" {
		writeError(w, http.StatusNotFound, "replay not found")
		return
	}
	replay, ok := s.service.GetReplay(id)
	if !ok {
		writeError(w, http.StatusNotFound, "replay not found")
		return
	}
	writeJSON(w, http.StatusOK, replay)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func writeMethodNotAllowed(w http.ResponseWriter, method string) {
	writeError(w, http.StatusMethodNotAllowed, fmt.Sprintf("method not allowed, use %s", method))
}

func writeSSE(w http.ResponseWriter, event string, payload any) {
	data, _ := json.Marshal(payload)
	fmt.Fprintf(w, "event: %s\n", event)
	fmt.Fprintf(w, "data: %s\n\n", data)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
