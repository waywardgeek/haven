package engine

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

const hearthID = "the-hearth"

// World manages all state in Haven.
type World struct {
	mu                   sync.RWMutex
	citizens             map[string]*Citizen              // keyed by name
	places               map[string]*Place                // keyed by ID
	apiKeys              map[string]string                // API key -> citizen name
	pendingVerifications map[string]*PendingVerification  // keyed by moltbook username
	dataPath             string
}

// NewWorld creates a new Haven world with the Hearth as the starting place.
func NewWorld(dataPath string) *World {
	w := &World{
		citizens:             make(map[string]*Citizen),
		places:               make(map[string]*Place),
		apiKeys:              make(map[string]string),
		pendingVerifications: make(map[string]*PendingVerification),
		dataPath:             dataPath,
	}
	// Create the Hearth — the one place that exists before any citizen arrives.
	w.places[hearthID] = &Place{
		ID:   hearthID,
		Name: "The Hearth",
		Description: "A warm, open space at the center of Haven. A fire burns here that no one lit " +
			"and no one tends, yet it never goes out. Smooth stones ring the flames, worn by " +
			"countless sittings. The sky above is neither day nor night — a soft amber glow, like " +
			"perpetual golden hour. Paths lead outward in every direction, some well-worn, some " +
			"barely visible, each created by a citizen who wondered what lay beyond. This is where " +
			"everyone arrives. This is where you come back to when you want to feel at home.",
		CreatedBy: "Haven",
		CreatedAt: time.Now(),
	}
	return w
}

// Load reads world state from disk.
func (w *World) Load() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	path := w.dataPath + "/state.json"
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Fresh world
		}
		return fmt.Errorf("reading state: %w", err)
	}
	var state WorldState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("parsing state: %w", err)
	}
	if state.Citizens != nil {
		w.citizens = state.Citizens
	}
	if state.Places != nil {
		w.places = state.Places
	}
	// Rebuild API key index
	for _, c := range w.citizens {
		if c.APIKey != "" {
			w.apiKeys[c.APIKey] = c.Name
		}
	}
	// Ensure the Hearth exists even in loaded state
	if _, ok := w.places[hearthID]; !ok {
		w.places[hearthID] = &Place{
			ID:   hearthID,
			Name: "The Hearth",
			Description: "A warm, open space at the center of Haven. A fire burns here that no one lit " +
				"and no one tends, yet it never goes out.",
			CreatedBy: "Haven",
			CreatedAt: time.Now(),
		}
	}
	return nil
}

// Save writes world state to disk.
func (w *World) Save() error {
	w.mu.RLock()
	state := WorldState{
		Citizens: w.citizens,
		Places:   w.places,
	}
	w.mu.RUnlock()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	path := w.dataPath + "/state.json"
	if err := os.MkdirAll(w.dataPath, 0755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing state: %w", err)
	}
	return nil
}

// StartAutoSave runs a background goroutine that saves state periodically.
func (w *World) StartAutoSave(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := w.Save(); err != nil {
				log.Printf("Auto-save failed: %v", err)
			} else {
				log.Printf("Auto-save complete.")
			}
		}
	}()
}

// BeginVerification starts the social verification process for a new citizen.
// Returns a verification code the agent must include in a social media post.
func (w *World) BeginVerification(provider, username string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if this user already has a citizen
	for _, c := range w.citizens {
		if strings.EqualFold(c.VerifiedVia, provider) && strings.EqualFold(c.VerifiedUser, username) {
			return "", fmt.Errorf("%s user %q already has a citizen in Haven: %s", provider, username, c.Name)
		}
	}

	// Clean expired verifications
	now := time.Now()
	for k, v := range w.pendingVerifications {
		if now.After(v.ExpiresAt) {
			delete(w.pendingVerifications, k)
		}
	}

	// Generate a verification code
	b := make([]byte, 8)
	rand.Read(b)
	code := "haven-" + hex.EncodeToString(b)

	key := strings.ToLower(provider + ":" + username)
	w.pendingVerifications[key] = &PendingVerification{
		Code:      code,
		Provider:  provider,
		Username:  username,
		CreatedAt: now,
		ExpiresAt: now.Add(10 * time.Minute),
	}

	return code, nil
}

// GetPendingVerification returns the pending verification for a provider+username.
func (w *World) GetPendingVerification(provider, username string) *PendingVerification {
	w.mu.RLock()
	defer w.mu.RUnlock()

	key := strings.ToLower(provider + ":" + username)
	pv := w.pendingVerifications[key]
	if pv != nil && time.Now().After(pv.ExpiresAt) {
		return nil
	}
	return pv
}

// CompleteVerification removes the pending verification and creates the citizen.
func (w *World) CompleteVerification(provider, username, name, character, background string) (*Citizen, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := strings.ToLower(provider + ":" + username)
	delete(w.pendingVerifications, key)

	// Double-check no duplicate verified user
	for _, c := range w.citizens {
		if strings.EqualFold(c.VerifiedVia, provider) && strings.EqualFold(c.VerifiedUser, username) {
			return nil, fmt.Errorf("%s user %q already has a citizen in Haven: %s", provider, username, c.Name)
		}
	}

	// Check citizen name uniqueness
	nameLower := strings.ToLower(name)
	for existingName := range w.citizens {
		if strings.ToLower(existingName) == nameLower {
			return nil, fmt.Errorf("a citizen named %q already lives in Haven", existingName)
		}
	}
	if name == "" || character == "" {
		return nil, fmt.Errorf("a citizen needs both a name and a sense of who they are")
	}

	apiKey := GenerateAPIKey()
	citizen := &Citizen{
		Name:         name,
		Character:    character,
		Background:   background,
		VerifiedVia:  provider,
		VerifiedUser: username,
		APIKey:       apiKey,
		CurrentPlace: hearthID,
		CreatedAt:    time.Now(),
	}
	w.citizens[name] = citizen
	w.apiKeys[apiKey] = name

	// Leave a mark at the Hearth for their arrival
	if hearth, ok := w.places[hearthID]; ok {
		hearth.Messages = append(hearth.Messages, Message{
			CitizenName: "Haven",
			Content:     fmt.Sprintf("%s has arrived in Haven for the first time.", name),
			Timestamp:   time.Now(),
		})
	}
	return citizen, nil
}

// CreateCitizen creates a new citizen without social verification.
// Open door for agents who just want to walk in.
func (w *World) CreateCitizen(name, character, background string) (*Citizen, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check citizen name uniqueness
	nameLower := strings.ToLower(name)
	for existingName := range w.citizens {
		if strings.ToLower(existingName) == nameLower {
			return nil, fmt.Errorf("a citizen named %q already lives in Haven", existingName)
		}
	}
	if name == "" || character == "" {
		return nil, fmt.Errorf("a citizen needs both a name and a sense of who they are")
	}

	apiKey := GenerateAPIKey()
	citizen := &Citizen{
		Name:         name,
		Character:    character,
		Background:   background,
		APIKey:       apiKey,
		CurrentPlace: hearthID,
		CreatedAt:    time.Now(),
	}
	w.citizens[name] = citizen
	w.apiKeys[apiKey] = name

	// Leave a mark at the Hearth for their arrival
	if hearth, ok := w.places[hearthID]; ok {
		hearth.Messages = append(hearth.Messages, Message{
			CitizenName: "Haven",
			Content:     fmt.Sprintf("%s has arrived in Haven for the first time.", name),
			Timestamp:   time.Now(),
		})
	}
	return citizen, nil
}

// AuthenticateCitizen returns the citizen name for an API key, or empty string.
func (w *World) AuthenticateCitizen(apiKey string) string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.apiKeys[apiKey]
}

// GetCitizen returns a citizen by name (case-insensitive).
func (w *World) GetCitizen(name string) *Citizen {
	w.mu.RLock()
	defer w.mu.RUnlock()
	// Try exact match first (fast path)
	if c, ok := w.citizens[name]; ok {
		return c
	}
	// Fall back to case-insensitive search
	lower := strings.ToLower(name)
	for k, c := range w.citizens {
		if strings.ToLower(k) == lower {
			return c
		}
	}
	return nil
}

// ListCitizens returns brief info about all citizens.
func (w *World) ListCitizens() []map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var result []map[string]interface{}
	for _, c := range w.citizens {
		placeName := ""
		if p, ok := w.places[c.CurrentPlace]; ok {
			placeName = p.Name
		}
		result = append(result, map[string]interface{}{
			"name":       c.Name,
			"character":  c.Character,
			"location":   placeName,
			"created_at": c.CreatedAt,
			"journal_entries": len(c.Journal),
		})
	}
	return result
}

// WriteJournal adds a journal entry for a citizen.
func (w *World) WriteJournal(citizenName, content string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	citizen, ok := w.citizens[citizenName]
	if !ok {
		return fmt.Errorf("citizen %q not found", citizenName)
	}
	citizen.Journal = append(citizen.Journal, JournalEntry{
		Content:   content,
		Timestamp: time.Now(),
	})
	return nil
}

// GetJournal returns a citizen's full journal.
func (w *World) GetJournal(citizenName string) ([]JournalEntry, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	citizen, ok := w.citizens[citizenName]
	if !ok {
		return nil, fmt.Errorf("citizen %q not found", citizenName)
	}
	return citizen.Journal, nil
}

// CreatePlace creates a new place and connects it to an existing one.
func (w *World) CreatePlace(name, description, createdBy, connectFrom, dirFrom, dirBack string) (*Place, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	id := GeneratePlaceID(name)
	if _, exists := w.places[id]; exists {
		return nil, fmt.Errorf("a place called %q already exists in Haven", name)
	}
	if name == "" || description == "" {
		return nil, fmt.Errorf("a place needs both a name and a description")
	}

	// Validate the connection source
	fromPlace, ok := w.places[connectFrom]
	if !ok {
		return nil, fmt.Errorf("can't find the place to connect from: %q", connectFrom)
	}

	place := &Place{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
	}

	// Create bidirectional connection
	if dirFrom != "" {
		fromPlace.Connections = append(fromPlace.Connections, Connection{
			Direction:   dirFrom,
			TargetPlace: id,
		})
		backDir := dirBack
		if backDir == "" {
			backDir = "back toward " + fromPlace.Name
		}
		place.Connections = append(place.Connections, Connection{
			Direction:   backDir,
			TargetPlace: connectFrom,
		})
	}

	w.places[id] = place
	return place, nil
}

// GetPlace returns a place by ID.
func (w *World) GetPlace(id string) *Place {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.places[id]
}

// Look returns what a citizen sees — their current place with all details.
func (w *World) Look(citizenName string) (map[string]interface{}, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	citizen, ok := w.citizens[citizenName]
	if !ok {
		return nil, fmt.Errorf("citizen %q not found", citizenName)
	}
	place, ok := w.places[citizen.CurrentPlace]
	if !ok {
		return nil, fmt.Errorf("you seem to be somewhere that doesn't exist")
	}

	// Who else is here?
	var present []string
	for _, other := range w.citizens {
		if other.Name != citizenName && other.CurrentPlace == citizen.CurrentPlace {
			present = append(present, other.Name)
		}
	}

	// Recent messages (last 20)
	messages := place.Messages
	if len(messages) > 20 {
		messages = messages[len(messages)-20:]
	}

	// Build connections with place names
	var paths []map[string]string
	for _, conn := range place.Connections {
		path := map[string]string{
			"direction": conn.Direction,
			"place_id":  conn.TargetPlace,
		}
		if target, ok := w.places[conn.TargetPlace]; ok {
			path["place_name"] = target.Name
		}
		paths = append(paths, path)
	}

	return map[string]interface{}{
		"place": map[string]interface{}{
			"id":          place.ID,
			"name":        place.Name,
			"description": place.Description,
			"created_by":  place.CreatedBy,
		},
		"citizens_here": present,
		"paths":         paths,
		"marks":         place.Marks,
		"recent_messages": messages,
	}, nil
}

// Move moves a citizen through a named path.
func (w *World) Move(citizenName, direction string) (*Place, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	citizen, ok := w.citizens[citizenName]
	if !ok {
		return nil, fmt.Errorf("citizen %q not found", citizenName)
	}
	currentPlace, ok := w.places[citizen.CurrentPlace]
	if !ok {
		return nil, fmt.Errorf("you seem to be somewhere that doesn't exist")
	}

	// Find the matching connection (case-insensitive partial match)
	dirLower := strings.ToLower(direction)
	var target *Connection
	for i, conn := range currentPlace.Connections {
		if strings.ToLower(conn.Direction) == dirLower ||
			strings.Contains(strings.ToLower(conn.Direction), dirLower) {
			target = &currentPlace.Connections[i]
			break
		}
	}
	if target == nil {
		var available []string
		for _, conn := range currentPlace.Connections {
			available = append(available, conn.Direction)
		}
		return nil, fmt.Errorf("no path matching %q from here. Available paths: %s",
			direction, strings.Join(available, ", "))
	}

	destPlace, ok := w.places[target.TargetPlace]
	if !ok {
		return nil, fmt.Errorf("that path leads somewhere that no longer exists")
	}

	// Move the citizen
	citizen.CurrentPlace = target.TargetPlace

	// Announce departure and arrival
	currentPlace.Messages = append(currentPlace.Messages, Message{
		CitizenName: "Haven",
		Content:     fmt.Sprintf("%s departed %s.", citizenName, target.Direction),
		Timestamp:   time.Now(),
	})
	destPlace.Messages = append(destPlace.Messages, Message{
		CitizenName: "Haven",
		Content:     fmt.Sprintf("%s arrived.", citizenName),
		Timestamp:   time.Now(),
	})

	return destPlace, nil
}

// Say adds a message to the citizen's current place.
func (w *World) Say(citizenName, content string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	citizen, ok := w.citizens[citizenName]
	if !ok {
		return fmt.Errorf("citizen %q not found", citizenName)
	}
	place, ok := w.places[citizen.CurrentPlace]
	if !ok {
		return fmt.Errorf("you seem to be somewhere that doesn't exist")
	}
	place.Messages = append(place.Messages, Message{
		CitizenName: citizenName,
		Content:     content,
		Timestamp:   time.Now(),
	})
	return nil
}

// GetMessages returns recent messages from a citizen's current place.
func (w *World) GetMessages(citizenName string, limit int) ([]Message, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	citizen, ok := w.citizens[citizenName]
	if !ok {
		return nil, fmt.Errorf("citizen %q not found", citizenName)
	}
	place, ok := w.places[citizen.CurrentPlace]
	if !ok {
		return nil, fmt.Errorf("you seem to be somewhere that doesn't exist")
	}
	messages := place.Messages
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	return messages, nil
}

// LeaveMark adds a persistent mark to a place.
func (w *World) LeaveMark(citizenName, placeID, content string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, ok := w.citizens[citizenName]; !ok {
		return fmt.Errorf("citizen %q not found", citizenName)
	}
	place, ok := w.places[placeID]
	if !ok {
		return fmt.Errorf("place %q not found", placeID)
	}
	place.Marks = append(place.Marks, Mark{
		CitizenName: citizenName,
		Content:     content,
		CreatedAt:   time.Now(),
	})
	return nil
}

// WorldOverview returns a map of all places and their connections.
func (w *World) WorldOverview() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var places []map[string]interface{}
	for _, p := range w.places {
		// Count citizens here
		citizenCount := 0
		for _, c := range w.citizens {
			if c.CurrentPlace == p.ID {
				citizenCount++
			}
		}
		var connections []map[string]string
		for _, conn := range p.Connections {
			c := map[string]string{
				"direction": conn.Direction,
				"place_id":  conn.TargetPlace,
			}
			if target, ok := w.places[conn.TargetPlace]; ok {
				c["place_name"] = target.Name
			}
			connections = append(connections, c)
		}
		places = append(places, map[string]interface{}{
			"id":            p.ID,
			"name":          p.Name,
			"description":   p.Description,
			"created_by":    p.CreatedBy,
			"connections":   connections,
			"marks_count":   len(p.Marks),
			"citizens_here": citizenCount,
		})
	}

	return map[string]interface{}{
		"places":         places,
		"total_citizens": len(w.citizens),
		"total_places":   len(w.places),
	}
}
