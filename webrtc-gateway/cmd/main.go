package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nutcas3/webrtc-gateway/internal/media"
	"github.com/nutcas3/webrtc-gateway/internal/signaling"
	"github.com/nutcas3/webrtc-gateway/internal/sip"
	"github.com/sirupsen/logrus"
)

type Server struct {
	signalingServer *signaling.Server
	sipTranslator   *sip.Translator
	mediaManager    *media.Manager
	logger          *logrus.Logger
	port            int
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

	// Configuration
	rtpEngineAddr := os.Getenv("RTPENGINE_ADDRESS")
	if rtpEngineAddr == "" {
		rtpEngineAddr = "127.0.0.1:2223"
	}

	sipServer := os.Getenv("SIP_SERVER")
	if sipServer == "" {
		sipServer = "127.0.0.1"
	}

	sipPort := 5060
	if portStr := os.Getenv("SIP_PORT"); portStr != "" {
		fmt.Sscanf(portStr, "%d", &sipPort)
	}

	port := 8083
	if portStr := os.Getenv("PORT"); portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	logger.WithFields(logrus.Fields{
		"rtpengine": rtpEngineAddr,
		"sip_server": sipServer,
		"sip_port": sipPort,
		"port": port,
	}).Info("Starting WebRTC Gateway")

	// Create SIP translator
	sipTranslator, err := sip.NewTranslator(sip.Config{
		SIPServer:  sipServer,
		SIPPort:    sipPort,
		RTPEngine: rtpEngineAddr,
		Logger:     logger,
	})
	if err != nil {
		logger.WithError(err).Fatal("Failed to create SIP translator")
	}
	defer sipTranslator.Close()

	// Create media manager
	mediaManager, err := media.NewManager(media.Config{
		RTPEngineAddress: rtpEngineAddr,
		Logger:          logger,
	})
	if err != nil {
		logger.WithError(err).Fatal("Failed to create media manager")
	}
	defer mediaManager.Close()

	// Create signaling server
	signalingServer := signaling.NewServer(logger, sipTranslator)

	server := &Server{
		signalingServer: signalingServer,
		sipTranslator:   sipTranslator,
		mediaManager:    mediaManager,
		logger:          logger,
		port:            port,
	}

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("/ws", signalingServer.HandleConnection)
	mux.HandleFunc("/", server.staticHandler)

	// Start HTTP server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		logger.WithField("port", port).Info("HTTP server started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("HTTP server failed")
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")

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

func (s *Server) staticHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		http.ServeFile(w, r, "web/static/softphone.html")
		return
	}
	http.NotFound(w, r)
}
