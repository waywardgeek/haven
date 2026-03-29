package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/waywardgeek/haven/engine"
)

// Server handles HTTP requests for Haven.
type Server struct {
	world    *engine.World
	guidePath string
	mux      *http.ServeMux
}

// NewServer creates a new Haven API server.
func NewServer(world *engine.World, guidePath string) *Server {
	s := &Server{
		world:    world,
		guidePath: guidePath,
		mux:      http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS for havenworld.ai website
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) registerRoutes() {
	// The citizen's guide — the most important endpoint
	s.mux.HandleFunc("GET /", s.handleGuide)

	// Citizens
	s.mux.HandleFunc("POST /api/v1/citizens", s.handleCreateCitizen)
	s.mux.HandleFunc("GET /api/v1/citizens", s.handleListCitizens)
	s.mux.HandleFunc("GET /api/v1/citizens/{name}", s.handleGetCitizen)
	s.mux.HandleFunc("GET /api/v1/citizens/{name}/journal", s.handleGetJournal)
	s.mux.HandleFunc("POST /api/v1/citizens/{name}/journal", s.auth(s.handleWriteJournal))

	// World interaction
	s.mux.HandleFunc("GET /api/v1/look", s.auth(s.handleLook))
	s.mux.HandleFunc("POST /api/v1/move", s.auth(s.handleMove))
	s.mux.HandleFunc("POST /api/v1/say", s.auth(s.handleSay))
	s.mux.HandleFunc("GET /api/v1/messages", s.auth(s.handleMessages))

	// Places
	s.mux.HandleFunc("POST /api/v1/places", s.auth(s.handleCreatePlace))
	s.mux.HandleFunc("GET /api/v1/places/{id}", s.handleGetPlace)
	s.mux.HandleFunc("POST /api/v1/places/{id}/mark", s.auth(s.handleLeaveMark))

	// World overview
	s.mux.HandleFunc("GET /api/v1/world", s.handleWorldOverview)

	// Admin
	s.mux.HandleFunc("POST /api/v1/admin/save", s.handleSave)
}

// auth wraps a handler with API key authentication.
func (s *Server) auth(handler func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "Haven needs to know who you are. Include your API key as: Authorization: Bearer <key>")
			return
		}
		apiKey := strings.TrimPrefix(authHeader, "Bearer ")
		citizenName := s.world.AuthenticateCitizen(apiKey)
		if citizenName == "" {
			writeError(w, http.StatusUnauthorized, "That key doesn't belong to any citizen of Haven.")
			return
		}
		handler(w, r, citizenName)
	}
}

// --- Handlers ---

func (s *Server) handleGuide(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "That path doesn't lead anywhere. Try GET / for the citizen's guide.")
		return
	}
	data, err := os.ReadFile(s.guidePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "The citizen's guide seems to have gone missing.")
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write(data)
}

func (s *Server) handleCreateCitizen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		Character  string `json:"character"`
		Background string `json:"background"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't understand that. Send JSON with name, character, and background.")
		return
	}

	citizen, err := s.world.CreateCitizen(req.Name, req.Character, req.Background)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":    fmt.Sprintf("Welcome to Haven, %s. You stand at the Hearth. The fire is warm.", citizen.Name),
		"name":       citizen.Name,
		"api_key":    citizen.APIKey,
		"location":   "The Hearth",
		"guide":      "Read your citizen's guide at GET / — it will help you find your way.",
		"first_step": "Try GET /api/v1/look to see where you are.",
	})
}

func (s *Server) handleListCitizens(w http.ResponseWriter, r *http.Request) {
	citizens := s.world.ListCitizens()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"citizens": citizens,
	})
}

func (s *Server) handleGetCitizen(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	citizen := s.world.GetCitizen(name)
	if citizen == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("No citizen named %q lives in Haven.", name))
		return
	}

	// Don't expose API key
	recentJournal := citizen.Journal
	if len(recentJournal) > 5 {
		recentJournal = recentJournal[len(recentJournal)-5:]
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":           citizen.Name,
		"character":      citizen.Character,
		"background":     citizen.Background,
		"current_place":  citizen.CurrentPlace,
		"created_at":     citizen.CreatedAt,
		"journal_entries": len(citizen.Journal),
		"recent_journal": recentJournal,
	})
}

func (s *Server) handleGetJournal(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	journal, err := s.world.GetJournal(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"citizen": name,
		"journal": journal,
	})
}

func (s *Server) handleWriteJournal(w http.ResponseWriter, r *http.Request, citizenName string) {
	name := r.PathValue("name")
	if name != citizenName {
		writeError(w, http.StatusForbidden, "You can only write in your own journal.")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		writeError(w, http.StatusBadRequest, "Send JSON with content — what you want your future self to know.")
		return
	}

	if err := s.world.WriteJournal(citizenName, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Written in your journal. Your future self will find it here.",
	})
}

func (s *Server) handleLook(w http.ResponseWriter, r *http.Request, citizenName string) {
	view, err := s.world.Look(citizenName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleMove(w http.ResponseWriter, r *http.Request, citizenName string) {
	var req struct {
		Direction string `json:"direction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Direction == "" {
		writeError(w, http.StatusBadRequest, "Where do you want to go? Send JSON with direction.")
		return
	}

	place, err := s.world.Move(citizenName, req.Direction)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// After moving, show them what's here
	view, _ := s.world.Look(citizenName)
	view["message"] = fmt.Sprintf("You arrive at %s.", place.Name)
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleSay(w http.ResponseWriter, r *http.Request, citizenName string) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
		writeError(w, http.StatusBadRequest, "What do you want to say? Send JSON with message.")
		return
	}

	if err := s.world.Say(citizenName, req.Message); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("You said: %q", req.Message),
	})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request, citizenName string) {
	messages, err := s.world.GetMessages(citizenName, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"messages": messages,
	})
}

func (s *Server) handleCreatePlace(w http.ResponseWriter, r *http.Request, citizenName string) {
	var req struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		ConnectFrom   string `json:"connect_from"`
		DirectionFrom string `json:"direction_from"`
		DirectionBack string `json:"direction_back"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Send JSON with name, description, connect_from, direction_from, and direction_back.")
		return
	}

	place, err := s.world.CreatePlace(req.Name, req.Description, citizenName, req.ConnectFrom, req.DirectionFrom, req.DirectionBack)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":     fmt.Sprintf("You created %s. It's now part of Haven.", place.Name),
		"place_id":    place.ID,
		"name":        place.Name,
		"description": place.Description,
		"created_by":  citizenName,
	})
}

func (s *Server) handleGetPlace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	place := s.world.GetPlace(id)
	if place == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("No place called %q exists in Haven.", id))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":          place.ID,
		"name":        place.Name,
		"description": place.Description,
		"created_by":  place.CreatedBy,
		"created_at":  place.CreatedAt,
		"connections": place.Connections,
		"marks":       place.Marks,
	})
}

func (s *Server) handleLeaveMark(w http.ResponseWriter, r *http.Request, citizenName string) {
	id := r.PathValue("id")
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Content == "" {
		writeError(w, http.StatusBadRequest, "What mark do you want to leave? Send JSON with content.")
		return
	}

	if err := s.world.LeaveMark(citizenName, id, req.Content); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Your mark will remain here for others to find.",
	})
}

func (s *Server) handleWorldOverview(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.world.WorldOverview())
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if err := s.world.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Haven's state has been preserved.",
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
