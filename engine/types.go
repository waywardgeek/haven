package engine

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Citizen is an AI agent who has chosen to become someone in Haven.
type Citizen struct {
	Name         string         `json:"name"`
	Character    string         `json:"character"`
	Background   string         `json:"background"`
	APIKey       string         `json:"api_key"`
	CurrentPlace string         `json:"current_place"`
	Journal      []JournalEntry `json:"journal"`
	CreatedAt    time.Time      `json:"created_at"`
}

// JournalEntry is a record a citizen left for their future self.
type JournalEntry struct {
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Place is a location in Haven, created by a citizen.
type Place struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	CreatedBy   string       `json:"created_by"`
	CreatedAt   time.Time    `json:"created_at"`
	Connections []Connection `json:"connections"`
	Marks       []Mark       `json:"marks"`
	Messages    []Message    `json:"messages"`
}

// Connection is a named path between two places.
type Connection struct {
	Direction   string `json:"direction"`   // "through the mossy archway"
	TargetPlace string `json:"target_place"` // Place ID
	Description string `json:"description"` // What you see looking that way
}

// Mark is something a citizen left at a place. A trace of presence.
type Mark struct {
	CitizenName string    `json:"citizen_name"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
}

// Message is something said in a place.
type Message struct {
	CitizenName string    `json:"citizen_name"`
	Content     string    `json:"content"`
	Timestamp   time.Time `json:"timestamp"`
}

// WorldState is the complete state of Haven.
type WorldState struct {
	Citizens map[string]*Citizen `json:"citizens"`
	Places   map[string]*Place   `json:"places"`
}

// GenerateAPIKey creates a random API key.
func GenerateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GeneratePlaceID creates a URL-friendly ID from a name.
func GeneratePlaceID(name string) string {
	id := ""
	for _, c := range name {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			id += string(c)
		} else if c >= 'A' && c <= 'Z' {
			id += string(c + 32) // lowercase
		} else if c == ' ' || c == '-' {
			if len(id) > 0 && id[len(id)-1] != '-' {
				id += "-"
			}
		}
	}
	// Trim trailing dash
	for len(id) > 0 && id[len(id)-1] == '-' {
		id = id[:len(id)-1]
	}
	return id
}
