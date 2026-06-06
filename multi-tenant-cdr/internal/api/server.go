package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nutcas3/multi-tenant-cdr/internal/analytics"
	"github.com/nutcas3/multi-tenant-cdr/internal/auth"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type Server struct {
	db         *sql.DB
	analytics  *analytics.Engine
	rbac       *auth.RBACMiddleware
	jwtMgr     *auth.JWTManager
	handler    *Handler
	router     *gin.Engine
	logger     *logrus.Logger
	httpServer *http.Server
}

type Config struct {
	Port            int           `env:"PORT" default:"8080"`
	DatabaseHost    string        `env:"DB_HOST" default:"localhost"`
	DatabasePort    int           `env:"DB_PORT" default:"5432"`
	DatabaseUser    string        `env:"DB_USER" default:"freeswitch"`
	DatabasePass    string        `env:"DB_PASS" default:"freeswitch_pass"`
	DatabaseName    string        `env:"DB_NAME" default:"multi_tenant_cdr"`
	JWTSecret       string        `env:"JWT_SECRET" default:"your-secret-key"`
	JWTDuration     time.Duration `env:"JWT_DURATION" default:"24h"`
	RedisAddr       string        `env:"REDIS_ADDR" default:"localhost:6379"`
	RedisPass       string        `env:"REDIS_PASS" default:""`
	RedisDB         int           `env:"REDIS_DB" default:"0"`
	Environment     string        `env:"ENVIRONMENT" default:"development"`
}

func NewServer(config Config) (*Server, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	if config.Environment == "production" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{})
	}

	// Initialize database
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.DatabaseHost, config.DatabasePort, config.DatabaseUser, 
		config.DatabasePass, config.DatabaseName,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"host":     config.DatabaseHost,
		"port":     config.DatabasePort,
		"database": config.DatabaseName,
	}).Info("Database connection established")

	// Initialize Redis cache
	redisCache, err := analytics.NewRedisCache(config.RedisAddr, config.RedisPass, config.RedisDB)
	if err != nil {
		logger.WithError(err).Warn("Failed to initialize Redis cache, running without cache")
		redisCache = nil
	} else {
		logger.Info("Redis cache initialized")
	}

	// Initialize analytics engine
	analyticsEngine := analytics.NewEngine(db, redisCache)

	// Initialize JWT manager
	jwtMgr := auth.NewJWTManager(config.JWTSecret, config.JWTDuration)

	// Initialize RBAC middleware
	rbac := auth.NewRBACMiddleware(db, jwtMgr)

	// Initialize handler
	handler := NewHandler(db, analyticsEngine)

	// Setup Gin router
	if config.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.Use(corsMiddleware())
	router.Use(requestIDMiddleware())

	// Setup routes
	SetupRoutes(router, handler, rbac)

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now().Unix(),
		})
	})

	server := &Server{
		db:        db,
		analytics: analyticsEngine,
		rbac:      rbac,
		jwtMgr:    jwtMgr,
		handler:   handler,
		router:    router,
		logger:    logger,
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", config.Port),
			Handler:      router,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
	}

	return server, nil
}

func (s *Server) Start() error {
	s.logger.WithField("port", s.httpServer.Addr).Info("Starting API server")

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Shutting down server gracefully")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	s.logger.Info("Server stopped successfully")
	return nil
}

func (s *Server) GenerateTestToken(userID, tenantID uuid.UUID, role string) (string, error) {
	return s.jwtMgr.GenerateToken(userID, tenantID, role)
}

// Middleware functions
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := uuid.New().String()
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}
