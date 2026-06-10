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

	"github.com/nutcas3/voip-push/internal/apns"
	"github.com/nutcas3/voip-push/internal/device"
	"github.com/nutcas3/voip-push/internal/fcm"
	"github.com/nutcas3/voip-push/internal/push"
	"github.com/sirupsen/logrus"
)

// Server holds all services
type Server struct {
	orchestrator *push.Orchestrator
	deviceMgr    *device.Manager
	logger       *logrus.Logger
	port         int
}

// DeviceRegistration represents a device registration request
type DeviceRegistration struct {
	UserID      string `json:"user_id"`
	DeviceID    string `json:"device_id"`
	Platform    string `json:"platform"`
	PushToken   string `json:"push_token"`
	VoIPToken   string `json:"voip_token,omitempty"`
	AppVersion  string `json:"app_version,omitempty"`
	OSVersion   string `json:"os_version,omitempty"`
	DeviceModel string `json:"device_model,omitempty"`
}

// PushRequest represents an incoming call push request
type PushRequest struct {
	UserID     string `json:"user_id"`
	CallerID   string `json:"caller_id"`
	CallerName string `json:"caller_name"`
	SessionID  string `json:"session_id"`
}

// PushResponse represents the response to a push request
type PushResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp int64  `json:"timestamp"`
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

	port := 8084
	if portStr := os.Getenv("PORT"); portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	// Initialize device manager
	deviceMgr := device.NewManager(logger)

	// Initialize FCM service (optional)
	var fcmService *fcm.Service
	fcmKey := os.Getenv("FCM_SERVER_KEY")
	if fcmKey != "" {
		fcmService = fcm.NewService(fcm.Config{
			ServerKey: fcmKey,
			ProjectID: os.Getenv("FCM_PROJECT_ID"),
			Logger:    logger,
		})
		logger.Info("FCM service initialized")
	}

	// Initialize APNS service (optional)
	var apnsService *apns.Service
	apnsBundleID := os.Getenv("APNS_BUNDLE_ID")
	if apnsBundleID != "" {
		apnsCfg := apns.Config{
			BundleID: apnsBundleID,
			Logger:   logger,
		}

		if os.Getenv("APNS_USE_JWT") == "true" {
			apnsCfg.UseJWT = true
			apnsCfg.KeyID = os.Getenv("APNS_KEY_ID")
			apnsCfg.TeamID = os.Getenv("APNS_TEAM_ID")
			apnsCfg.AuthKeyPath = os.Getenv("APNS_AUTH_KEY_PATH")
		} else {
			apnsCfg.CertPath = os.Getenv("APNS_CERT_PATH")
			apnsCfg.CertPassword = os.Getenv("APNS_CERT_PASSWORD")
		}

		apnsService, err = apns.NewService(apnsCfg)
		if err != nil {
			logger.WithError(err).Fatal("Failed to initialize APNS service")
		}
		logger.Info("APNS service initialized")
	}

	// Initialize push orchestrator
	orchestrator := push.NewOrchestrator(push.Config{
		FCMService:  fcmService,
		APNSService: apnsService,
		DeviceMgr:   deviceMgr,
		RetryCount:  3,
		RetryDelay:  2 * time.Second,
		PushTimeout: 10 * time.Second,
		Logger:      logger,
	})

	server := &Server{
		orchestrator: orchestrator,
		deviceMgr:    deviceMgr,
		logger:       logger,
		port:         port,
	}

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("/register", server.registerHandler)
	mux.HandleFunc("/unregister", server.unregisterHandler)
	mux.HandleFunc("/push", server.pushHandler)
	mux.HandleFunc("/stats", server.statsHandler)
	mux.HandleFunc("/", server.dashboardHandler)

	// Start HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		logger.WithField("port", port).Info("VoIP Push service started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("HTTP server failed")
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")

	orchestrator.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("HTTP shutdown error")
	}

	logger.Info("Server exited")
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}\n`, time.Now().Format(time.RFC3339))
}

func (s *Server) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.ServeFile(w, r, "web/static/index.html")
		return
	}
	http.NotFound(w, r)
}

func (s *Server) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeviceRegistration
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, "Invalid request body")
		return
	}

	if req.UserID == "" || req.PushToken == "" || req.Platform == "" {
		s.respondError(w, "Missing required fields")
		return
	}

	platform := device.Platform(req.Platform)
	if platform != device.PlatformIOS && platform != device.PlatformAndroid {
		s.respondError(w, "Invalid platform")
		return
	}

	dev := &device.Device{
		ID:          req.DeviceID,
		UserID:      req.UserID,
		Platform:    platform,
		PushToken:   req.PushToken,
		VoIPToken:   req.VoIPToken,
		AppVersion:  req.AppVersion,
		OSVersion:   req.OSVersion,
		DeviceModel: req.DeviceModel,
	}

	if err := s.deviceMgr.Register(dev); err != nil {
		s.logger.WithError(err).Error("Failed to register device")
		s.respondError(w, err.Error())
		return
	}

	s.respondJSON(w, PushResponse{
		Success:   true,
		Message:   "Device registered successfully",
		Timestamp: time.Now().Unix(),
	})
}

func (s *Server) unregisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, "Invalid request body")
		return
	}

	if req.UserID == "" {
		s.respondError(w, "Missing user_id")
		return
	}

	if err := s.deviceMgr.Unregister(req.UserID); err != nil {
		s.logger.WithError(err).Error("Failed to unregister device")
		s.respondError(w, err.Error())
		return
	}

	s.respondJSON(w, PushResponse{
		Success:   true,
		Message:   "Device unregistered successfully",
		Timestamp: time.Now().Unix(),
	})
}

func (s *Server) pushHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, "Invalid request body")
		return
	}

	if req.UserID == "" || req.CallerID == "" || req.SessionID == "" {
		s.respondError(w, "Missing required fields")
		return
	}

	if req.CallerName == "" {
		req.CallerName = req.CallerID
	}

	result, err := s.orchestrator.SendIncomingCallPush(req.UserID, req.CallerID, req.CallerName, req.SessionID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to send push")
		s.respondJSON(w, PushResponse{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now().Unix(),
		})
		return
	}

	if !result.Success {
		s.respondJSON(w, PushResponse{
			Success:   false,
			Error:     result.Error.Error(),
			Timestamp: time.Now().Unix(),
		})
		return
	}

	s.respondJSON(w, PushResponse{
		Success:   true,
		Message:   fmt.Sprintf("Push sent to %s device", result.Platform),
		Timestamp: time.Now().Unix(),
	})
}

func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := s.orchestrator.GetStats()
	stats["timestamp"] = time.Now().Unix()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) respondJSON(w http.ResponseWriter, resp PushResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) respondError(w http.ResponseWriter, message string) {
	s.respondJSON(w, PushResponse{
		Success:   false,
		Error:     message,
		Timestamp: time.Now().Unix(),
	})
}
