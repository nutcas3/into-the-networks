package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)

	config := loadConfig()

	// Initialize Kamailio RPC client
	kamailioClient := NewKamailioClient(config.KamailioHost, config.KamailioPort)

	// Initialize database
	db, err := NewDB(config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)
	if err != nil {
		logger.WithError(err).Fatal("Failed to connect to database")
	}
	defer db.Close()

	// Initialize router
	router := gin.Default()

	// Dispatcher routes
	dispatcherGroup := router.Group("/api/v1/dispatcher")
	{
		dispatcherGroup.POST("/add", addDestinationHandler(kamailioClient, db))
		dispatcherGroup.POST("/remove", removeDestinationHandler(kamailioClient, db))
		dispatcherGroup.GET("/list", listDestinationsHandler(db))
		dispatcherGroup.POST("/reload", reloadDispatcherHandler(kamailioClient))
	}

	// Carrier routes
	carrierGroup := router.Group("/api/v1/carrier")
	{
		carrierGroup.POST("/add", addCarrierHandler(db))
		carrierGroup.POST("/remove", removeCarrierHandler(db))
		carrierGroup.GET("/list", listCarriersHandler(db))
		carrierGroup.POST("/route", addRouteHandler(db))
		carrierGroup.GET("/route/:prefix", getRouteHandler(db))
	}

	// Health check routes
	healthGroup := router.Group("/api/v1/health")
	{
		healthGroup.GET("/status", healthStatusHandler(db))
		healthGroup.POST("/check", performHealthCheckHandler(kamailioClient, db))
	}

	// Start server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: router,
	}

	go func() {
		logger.WithField("port", config.Port).Info("Starting RPC server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("Server forced to shutdown")
	}

	logger.Info("Server exited")
}

type Config struct {
	KamailioHost string
	KamailioPort int
	DBHost       string
	DBPort       int
	DBUser       string
	DBPassword   string
	DBName       string
	Port         int
}

func loadConfig() Config {
	return Config{
		KamailioHost: getEnv("KAMAILIO_HOST", "kamailio"),
		KamailioPort: getEnvInt("KAMAILIO_PORT", 5060),
		DBHost:       getEnv("DB_HOST", "postgres"),
		DBPort:       getEnvInt("DB_PORT", 5432),
		DBUser:       getEnv("DB_USER", "kamailio"),
		DBPassword:   getEnv("DB_PASSWORD", "kamailio"),
		DBName:       getEnv("DB_NAME", "kamailio"),
		Port:         getEnvInt("PORT", 8080),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
