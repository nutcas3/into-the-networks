package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"freeswitch-cdr/internal/cdr"
	"freeswitch-cdr/internal/config"
	"freeswitch-cdr/internal/db"
	"freeswitch-cdr/internal/esl"
	"freeswitch-cdr/internal/service"
)

func main() {
	log.Println("Starting FreeSWITCH CDR Service...")

	// Load configuration
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// Initialize database
	database, err := db.New(db.Config{
		Host:     cfg.PostgreSQL.Host,
		Port:     cfg.PostgreSQL.Port,
		User:     cfg.PostgreSQL.User,
		Password: cfg.PostgreSQL.Password,
		DBName:   cfg.PostgreSQL.DBName,
	})
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer database.Close()

	// Connect to FreeSWITCH via ESL
	address := fmt.Sprintf("%s:%s", cfg.FreeSWITCH.Host, cfg.FreeSWITCH.Port)
	log.Printf("Connecting to FreeSWITCH at %s...", address)

	eslConn, err := esl.Dial(address, esl.Options{Password: cfg.FreeSWITCH.Password})
	if err != nil {
		log.Fatalf("Error connecting to FreeSWITCH: %v", err)
	}
	defer eslConn.ExitAndClose()

	log.Println("Authenticated successfully")

	// Initialize CDR repository
	cdrRepo := cdr.NewRepository(database)

	// Initialize service
	svc := service.New(eslConn, cdrRepo)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start service in goroutine
	go func() {
		if err := svc.Start(); err != nil {
			log.Printf("Service error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutting down...")
}
