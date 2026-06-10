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

	"github.com/nutcas3/wireguard-voip/internal/monitor"
	"github.com/nutcas3/wireguard-voip/internal/peer"
	"github.com/nutcas3/wireguard-voip/internal/tunnel"
	"github.com/nutcas3/wireguard-voip/internal/wg"
	"github.com/nutcas3/wireguard-voip/internal/zero"
	"github.com/sirupsen/logrus"
)

// Server holds all services
type Server struct {
	wgService     *wg.Service
	peerMgr       *peer.Manager
	tunnelSvc     *tunnel.Service
	zeroRater     *zero.Rater
	collector     *monitor.Collector
	logger        *logrus.Logger
	port          int
}

// ProvisionRequest represents a peer provisioning request
type ProvisionRequest struct {
	UserID     string `json:"user_id"`
	DeviceInfo string `json:"device_info,omitempty"`
}

// ProvisionResponse represents peer configuration for the client
type ProvisionResponse struct {
	Success    bool     `json:"success"`
	PrivateKey string   `json:"private_key,omitempty"`
	PublicKey  string   `json:"public_key,omitempty"`
	ServerIP   string   `json:"server_ip,omitempty"`
	ClientIP   string   `json:"client_ip,omitempty"`
	ServerPort int      `json:"server_port,omitempty"`
	AllowedIPs []string `json:"allowed_ips,omitempty"`
	Error      string   `json:"error,omitempty"`
	Timestamp  int64    `json:"timestamp"`
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

	port := 8085
	if portStr := os.Getenv("PORT"); portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	serverIP := os.Getenv("SERVER_IP")
	if serverIP == "" {
		serverIP = "auto-detect"
	}

	serverPort := 51820
	if portStr := os.Getenv("WG_PORT"); portStr != "" {
		fmt.Sscanf(portStr, "%d", &serverPort)
	}

	subnet := os.Getenv("WG_SUBNET")
	if subnet == "" {
		subnet = "10.200.0.0/24"
	}

	logger.WithFields(logrus.Fields{
		"port":      port,
		"server_ip": serverIP,
		"wg_port":   serverPort,
		"subnet":    subnet,
	}).Info("Starting WireGuard VoIP service")

	// Initialize WireGuard service
	wgService := wg.NewService(wg.Config{
		Name:   "wg0",
		Logger: logger,
	})

	// Initialize peer manager
	peerMgr, err := peer.NewManager(peer.Config{
		Subnet:    subnet,
		Logger:    logger,
		WGService: wgService,
	})
	if err != nil {
		logger.WithError(err).Fatal("Failed to create peer manager")
	}

	// Initialize tunnel service
	tunnelSvc := tunnel.NewService(tunnel.Config{
		Name:        "wg0",
		LocalAddr:   fmt.Sprintf("%s:%d", serverIP, serverPort),
		MTU:         1420,
		VoIPSubnets: tunnel.DefaultVoIPSubnets(),
		Logger:      logger,
	})

	// Initialize zero-rater
	zeroRater := zero.NewRater(zero.Config{
		CarrierAPI: os.Getenv("CARRIER_API"),
		Enabled:    os.Getenv("ZERO_RATING_ENABLED") != "false",
		Logger:     logger,
	})

	// Configure WireGuard interface
	privKey, pubKey, err := wg.GenerateKeyPair()
	if err != nil {
		logger.WithError(err).Fatal("Failed to generate server keys")
	}

	logger.WithField("public_key", pubKey[:16]+"...").Info("Generated server key pair")

	wgConfig := &wg.InterfaceConfig{
		PrivateKey: privKey,
		ListenPort: serverPort,
		Address:    subnet,
		MTU:        1420,
		Peers:      []wg.PeerConfig{},
	}
	wgService.Configure(wgConfig)

	// Initialize metrics collector
	collector := monitor.NewCollector(monitor.Config{
		WGService: wgService,
		PeerMgr:   peerMgr,
		TunnelSvc: tunnelSvc,
		Interval:  30 * time.Second,
		Logger:    logger,
	})

	server := &Server{
		wgService: wgService,
		peerMgr:   peerMgr,
		tunnelSvc: tunnelSvc,
		zeroRater: zeroRater,
		collector: collector,
		logger:    logger,
		port:      port,
	}

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("/provision", server.provisionHandler)
	mux.HandleFunc("/revoke", server.revokeHandler)
	mux.HandleFunc("/peers", server.peersHandler)
	mux.HandleFunc("/metrics", server.metricsHandler)
	mux.HandleFunc("/config", server.configHandler)
	mux.HandleFunc("/", server.dashboardHandler)

	// Start HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		logger.WithField("port", port).Info("WireGuard VoIP service started")
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
	tunnelSvc.Down()
	wgService.Down()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.WithError(err).Error("HTTP shutdown error")
	}

	logger.Info("Server exited")
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	health := s.collector.CheckHealth()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Server) dashboardHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.ServeFile(w, r, "web/static/index.html")
		return
	}
	http.NotFound(w, r)
}

func (s *Server) provisionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProvisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body")
		return
	}

	if req.UserID == "" {
		respondError(w, "Missing user_id")
		return
	}

	p, err := s.peerMgr.Provision(req.UserID, req.DeviceInfo)
	if err != nil {
		s.logger.WithError(err).Error("Failed to provision peer")
		respondJSON(w, ProvisionResponse{
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now().Unix(),
		})
		return
	}

	wgConfig := s.wgService.GetConfig()

	respondJSON(w, ProvisionResponse{
		Success:    true,
		PrivateKey: p.PrivateKey,
		PublicKey:  p.PublicKey,
		ServerIP:   "", // Client should use configured endpoint
		ClientIP:   p.IP,
		ServerPort: wgConfig.ListenPort,
		AllowedIPs: []string{"0.0.0.0/0"},
		Timestamp:  time.Now().Unix(),
	})
}

func (s *Server) revokeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, "Invalid request body")
		return
	}

	if req.PublicKey == "" {
		respondError(w, "Missing public_key")
		return
	}

	if err := s.peerMgr.Revoke(req.PublicKey); err != nil {
		s.logger.WithError(err).Error("Failed to revoke peer")
		respondError(w, err.Error())
		return
	}

	respondJSON(w, map[string]interface{}{
		"success":   true,
		"message":   "Peer revoked",
		"timestamp": time.Now().Unix(),
	})
}

func (s *Server) peersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	peers := s.peerMgr.GetAll()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peers":     peers,
		"count":     len(peers),
		"timestamp": time.Now().Unix(),
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

func (s *Server) configHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	wgConfig := s.wgService.GetConfig()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"interface": wgConfig,
		"tunnel":    s.tunnelSvc.GetStats(),
		"zero_rated": s.zeroRater.IsEnabled(),
		"policies":  s.zeroRater.GetPolicies(),
		"timestamp": time.Now().Unix(),
	})
}

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, message string) {
	respondJSON(w, ProvisionResponse{
		Success:   false,
		Error:     message,
		Timestamp: time.Now().Unix(),
	})
}
