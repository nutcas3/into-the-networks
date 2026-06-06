package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nutcas3/multi-tenant-cdr/internal/api"
	"github.com/nutcas3/multi-tenant-cdr/internal/cdr"
	"github.com/nutcas3/multi-tenant-cdr/internal/db"
	"github.com/nutcas3/multi-tenant-cdr/internal/esl"
	"github.com/nutcas3/multi-tenant-cdr/internal/service"
	"github.com/sirupsen/logrus"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)

	// Load configuration
	config := api.Config{
		Port:         8080,
		DatabaseHost: getEnv("DB_HOST", "localhost"),
		DatabasePort: 5432,
		DatabaseUser: getEnv("DB_USER", "cdr_user"),
		DatabasePass: getEnv("DB_PASSWORD", "cdr_password"),
		DatabaseName: getEnv("DB_NAME", "cdr_db"),
		JWTSecret:    getEnv("JWT_SECRET", "your-secret-key"),
		JWTDuration:  24 * time.Hour,
		RedisAddr:    fmt.Sprintf("%s:%s", getEnv("REDIS_HOST", "localhost"), getEnv("REDIS_PORT", "6379")),
		RedisPass:    "",
		RedisDB:      0,
		Environment:  getEnv("ENVIRONMENT", "development"),
	}

	// FreeSWITCH configuration
	freeswitchAddr := fmt.Sprintf("%s:%s", getEnv("FREESWITCH_HOST", "localhost"), getEnv("FREESWITCH_PORT", "8021"))
	freeswitchPassword := getEnv("FREESWITCH_PASSWORD", "ClueCon")

	logLevel := getEnv("LOG_LEVEL", "info")
	switch logLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// Create API server
	server, err := api.NewServer(config)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create server")
	}

	// Initialize ESL connection for CDR capture
	eslConn, err := esl.Dial(freeswitchAddr, esl.Options{Password: freeswitchPassword})
	if err != nil {
		logger.WithError(err).Warn("Failed to connect to FreeSWITCH ESL, CDR capture disabled")
	} else {
		defer eslConn.Close()

		// Initialize database for CDR storage
		dbConfig := db.Config{
			Host:     config.DatabaseHost,
			Port:     fmt.Sprintf("%d", config.DatabasePort),
			User:     config.DatabaseUser,
			Password: config.DatabasePass,
			DBName:   config.DatabaseName,
		}
		dbConn, err := db.New(dbConfig)
		if err != nil {
			logger.WithError(err).Error("Failed to connect to database for CDR storage")
			eslConn.Close()
		} else {
			defer dbConn.Close()

			// Initialize CDR repository
			cdrRepo := cdr.NewRepository(dbConn)

			// Initialize CDR service
			cdrService := service.New(eslConn, cdrRepo)

			// Start CDR capture service in goroutine
			go func() {
				logger.Info("Starting CDR capture service")
				if err := cdrService.Start(); err != nil {
					logger.WithError(err).Error("CDR capture service failed")
				}
			}()
		}
	}

	// Start API server in a goroutine
	go func() {
		logger.Info("Starting API server")
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		logger.WithError(err).Error("Server forced to shutdown")
	}

	logger.Info("Server exited")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
