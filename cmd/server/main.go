package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/waywardgeek/haven/api"
	"github.com/waywardgeek/haven/engine"
)

func main() {
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	dataPath := "data"
	guidePath := "citizens-guide.md"

	// Create the world
	world := engine.NewWorld(dataPath)
	if err := world.Load(); err != nil {
		log.Fatalf("Failed to load Haven: %v", err)
	}
	world.StartAutoSave(5 * time.Minute)
	log.Printf("Haven is waking up...")

	// Create the server
	server := api.NewServer(world, guidePath)

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-shutdown
		log.Printf("Haven is going to sleep. Saving state...")
		if err := world.Save(); err != nil {
			log.Printf("Error saving state: %v", err)
		} else {
			log.Printf("State saved. Goodnight.")
		}
		os.Exit(0)
	}()

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Haven is open at http://localhost%s", addr)
	log.Printf("Citizens may read the guide at GET /")
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("Haven couldn't start: %v", err)
	}
}
