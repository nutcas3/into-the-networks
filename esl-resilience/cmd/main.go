package main

import (
	"log"

	"github.com/nutcas3/esl-resilience/internal"
)

func main() {
	log.Println("Starting ESL Resilience Server...")

	// Create server with default configuration
	config := internal.DefaultConfig()
	server := internal.NewServer(config)

	// Run server with graceful shutdown
	if err := server.Run(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}

	log.Println("Server stopped")
}
