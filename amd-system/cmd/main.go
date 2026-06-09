package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nutcas3/amd-system/internal/call"
	"github.com/nutcas3/amd-system/internal/classifier"
	"github.com/nutcas3/amd-system/internal/monitor"
	"github.com/nutcas3/amd-system/internal/router"
	"github.com/sirupsen/logrus"
)

// Server holds all services
type Server struct {
	callMgr    *call.Manager
	classifier *classifier.Client
	router     *router.Service
	collector  *monitor.Collector
	logger     *logrus.Logger
	port       int
}

// StartCallRequest initiates a new call session
type StartCallRequest struct {
	PhoneNumber string `json:"phone_number"`
	CampaignID  string `json:"campaign_id"`
}

// AudioChunkRequest sends audio for classification
type AudioChunkRequest struct {
	SessionID  string `json:"session_id"`
	AudioData  []byte `json:"audio_data"`
	SampleRate int    `json:"sample_rate"`
	IsFinal    bool   `json:"is_final,omitempty"`
}

// ClassificationResultResponse returns the classification outcome
type ClassificationResultResponse struct {
	SessionID  string      `json:"session_id"`
	Result     call.Result `json:"result"`
	Confidence float64     `json:"confidence"`
	RoutedTo   string      `json:"routed_to,omitempty"`
	Timestamp  int64       `json:"timestamp"`
}

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	level, err := logrus.ParseLevel(logLevel)
	if err == nil {
		logger.SetLevel(level)
	}

	port := 8086
	if portStr := os.Getenv("PORT"); portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	mlEndpoint := os.Getenv("ML_ENDPOINT")
	if mlEndpoint == "" {
		mlEndpoint = "http://localhost:5000"
	}

	logger.WithFields(logrus.Fields{
		"port":        port,
		"ml_endpoint": mlEndpoint,
	}).Info("Starting AMD System")

	// Initialize services
	callMgr := call.NewManager(call.Config{Logger: logger})

	cls := classifier.NewClient(classifier.Config{
		Endpoint: mlEndpoint,
		Logger:   logger,
	})

	routerSvc := router.NewService(router.Config{
		CallMgr:      callMgr,
		DefaultRoute: os.Getenv("AGENT_QUEUE"),
		AMRoute:      os.Getenv("VOICEMAIL_QUEUE"),
		FaxRoute:     os.Getenv("FAX_QUEUE"),
		UnknownRoute: os.Getenv("RETRY_QUEUE"),
		MaxRetries:   3,
		Logger:       logger,
	})

	collector := monitor.NewCollector(monitor.Config{
		CallMgr:    callMgr,
		Classifier: cls,
		Logger:     logger,
		Interval:   15 * time.Second,
	})

	server := &Server{
		callMgr:    callMgr,
		classifier: cls,
		router:     routerSvc,
		collector:  collector,
		logger:     logger,
		port:       port,
	}

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("/start", server.startCallHandler)
	mux.HandleFunc("/audio", server.audioChunkHandler)
	mux.HandleFunc("/result", server.getResultHandler)
	mux.HandleFunc("/stats", server.statsHandler)
	mux.HandleFunc("/metrics", server.metricsHandler)
	mux.HandleFunc("/classifier/health", server.classifierHealthHandler)
	mux.HandleFunc("/", server.dashboardHandler)

	// Start HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		logger.WithField("port", port).Info("AMD System started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("HTTP server failed")
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")

	collector.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("HTTP shutdown error")
	}

	logger.Info("Server exited")
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}
`, time.Now().Format(time.RFC3339))
}

func (s *Server) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.ServeFile(w, r, "web/static/index.html")
		return
	}
	http.NotFound(w, r)
}

func (s *Server) startCallHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req StartCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body")
		return
	}

	if req.PhoneNumber == "" {
		respondError(w, "Missing phone_number")
		return
	}

	session := s.callMgr.StartSession(req.PhoneNumber, req.CampaignID)

	respondJSON(w, map[string]interface{}{
		"success":    true,
		"session_id": session.ID,
		"started_at": session.StartedAt,
	})
}

func (s *Server) audioChunkHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AudioChunkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body")
		return
	}

	if req.SessionID == "" {
		respondError(w, "Missing session_id")
		return
	}

	// Record chunk
	if _, err := s.callMgr.RecordAudioChunk(req.SessionID); err != nil {
		respondError(w, err.Error())
		return
	}

	// Classify
	result, err := s.classifier.Classify(req.SessionID, req.AudioData, req.SampleRate)
	if err != nil {
		s.logger.WithError(err).WithField("session_id", req.SessionID).Error("Classification failed")
		respondError(w, err.Error())
		return
	}

	// Convert result
	clsResult := call.Result(result.Result)
	confidence := result.Confidence

	// Complete session if final or high confidence
	if req.IsFinal || confidence > 0.85 {
		s.callMgr.CompleteSession(req.SessionID, clsResult, confidence)
		s.router.Route(req.SessionID, clsResult, confidence)
	}

	respondJSON(w, ClassificationResultResponse{
		SessionID:  req.SessionID,
		Result:     clsResult,
		Confidence: confidence,
		Timestamp:  time.Now().Unix(),
	})
}

func (s *Server) getResultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		respondError(w, "Missing session_id")
		return
	}

	session, exists := s.callMgr.GetSession(sessionID)
	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	respondJSON(w, ClassificationResultResponse{
		SessionID:  session.ID,
		Result:     session.Result,
		Confidence: session.Confidence,
		RoutedTo:   session.RoutedTo,
		Timestamp:  time.Now().Unix(),
	})
}

func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := s.callMgr.GetStats()
	routing := s.router.GetRoutingStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"calls":   stats,
		"routing": routing,
	})
}

func (s *Server) classifierHealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health, err := s.classifier.Health()
	if err != nil {
		respondJSON(w, map[string]interface{}{
			"available": false,
			"error":     err.Error(),
		})
		return
	}

	respondJSON(w, map[string]interface{}{
		"available": true,
		"status":    health.Status,
		"model":     health.Model,
		"version":   health.Version,
	})
}

func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics := s.collector.GetMetrics()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, message string) {
	respondJSON(w, map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
