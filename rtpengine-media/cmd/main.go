package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nutcas3/rtpengine-media/internal/metrics"
	"github.com/nutcas3/rtpengine-media/internal/session"
	"github.com/sirupsen/logrus"
)

type Server struct {
	manager   *session.Manager
	collector *metrics.Collector
	logger    *logrus.Logger
	port      int
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

	ngAddress := os.Getenv("RTPEngine_ADDRESS")
	if ngAddress == "" {
		ngAddress = "127.0.0.1:2223"
	}

	port := 8082
	if portStr := os.Getenv("PORT"); portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	logger.WithFields(logrus.Fields{
		"ng_address": ngAddress,
		"port":       port,
	}).Info("Starting RTPengine media server")

	// Create session manager
	manager, err := session.NewManager(session.Config{
		NGAddress:       ngAddress,
		SessionTimeout:  1 * time.Hour,
		CleanupInterval: 5 * time.Minute,
		Logger:          logger,
	})
	if err != nil {
		logger.WithError(err).Fatal("Failed to create session manager")
	}
	defer manager.Close()

	// Create metrics collector
	collector := metrics.NewCollector()

	server := &Server{
		manager:   manager,
		collector: collector,
		logger:    logger,
		port:      port,
	}

	// Start HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("/api/v1/sessions", server.sessionsHandler)
	mux.HandleFunc("/api/v1/sessions/", server.sessionHandler)
	mux.HandleFunc("/api/v1/metrics", server.metricsHandler)
	mux.HandleFunc("/api/v1/metrics/session/", server.sessionMetricsHandler)
	mux.HandleFunc("/api/v1/offer", server.offerHandler)
	mux.HandleFunc("/api/v1/answer", server.answerHandler)
	mux.HandleFunc("/api/v1/delete", server.deleteHandler)
	mux.HandleFunc("/api/v1/record/start", server.startRecordingHandler)
	mux.HandleFunc("/api/v1/record/stop", server.stopRecordingHandler)

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

func (s *Server) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sessions := s.manager.List()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "[%d sessions]\n", len(sessions))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) sessionHandler(w http.ResponseWriter, r *http.Request) {
	callID := r.URL.Path[len("/api/v1/sessions/"):]
	if callID == "" {
		http.Error(w, "Missing call ID", http.StatusBadRequest)
		return
	}

	session, exists := s.manager.Get(callID)
	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "callid=%s status=%s\n", session.CallID, session.Status)
}

func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	global := s.collector.GetGlobal()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
  "active_sessions": %d,
  "total_sessions": %d,
  "total_calls": %d,
  "failed_calls": %d,
  "avg_mos": %.2f,
  "avg_jitter_ms": %.2f,
  "avg_packet_loss_percent": %.2f
}
`, global.ActiveSessions, global.TotalSessions, global.TotalCalls, global.FailedCalls, global.AvgMOS, global.AvgJitter, global.AvgPacketLoss)
}

func (s *Server) sessionMetricsHandler(w http.ResponseWriter, r *http.Request) {
	callID := r.URL.Path[len("/api/v1/metrics/session/"):]
	if callID == "" {
		http.Error(w, "Missing call ID", http.StatusBadRequest)
		return
	}

	m, exists := s.collector.GetSession(callID)
	if !exists {
		http.Error(w, "Session metrics not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{
  "call_id": "%s",
  "codec": "%s",
  "packets_received": %d,
  "packets_sent": %d,
  "packets_lost": %d,
  "jitter_ms": %.2f,
  "rtt_ms": %.2f,
  "mos_score": %.2f,
  "packet_loss_rate": %.2f
}
`, m.CallID, m.Codec, m.PacketsReceived, m.PacketsSent, m.PacketsLost, m.Jitter, m.RTT, m.MOS, m.GetPacketLossRate())
}

func (s *Server) offerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	callID := r.FormValue("callid")
	fromTag := r.FormValue("fromtag")
	sdp := r.FormValue("sdp")

	if callID == "" || fromTag == "" || sdp == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	options := make(map[string]interface{})
	if dir := r.FormValue("direction"); dir != "" {
		options["direction"] = []string{dir}
	}
	if flags := r.FormValue("flags"); flags != "" {
		options["flags"] = []string{flags}
	}
	if ice := r.FormValue("ICE"); ice != "" {
		options["ICE"] = ice
	}

	_, localSDP, err := s.manager.Offer(callID, fromTag, sdp, options)
	if err != nil {
		s.logger.WithError(err).Error("Offer failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.collector.AddSession(callID, "unknown")
	s.collector.IncrementCalls()

	w.Header().Set("Content-Type", "application/sdp")
	w.Write([]byte(localSDP))
}

func (s *Server) answerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	callID := r.FormValue("callid")
	fromTag := r.FormValue("fromtag")
	toTag := r.FormValue("totag")
	sdp := r.FormValue("sdp")

	if callID == "" || fromTag == "" || toTag == "" || sdp == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	options := make(map[string]interface{})
	_, localSDP, err := s.manager.Answer(callID, fromTag, toTag, sdp, options)
	if err != nil {
		s.logger.WithError(err).Error("Answer failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/sdp")
	w.Write([]byte(localSDP))
}

func (s *Server) deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	callID := r.FormValue("callid")
	fromTag := r.FormValue("fromtag")
	toTag := r.FormValue("totag")

	if callID == "" {
		http.Error(w, "Missing callid", http.StatusBadRequest)
		return
	}

	if err := s.manager.Delete(callID, fromTag, toTag); err != nil {
		s.logger.WithError(err).Error("Delete failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.collector.RemoveSession(callID)

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"deleted"}`)
}

func (s *Server) startRecordingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	callID := r.FormValue("callid")
	fromTag := r.FormValue("fromtag")
	toTag := r.FormValue("totag")

	if callID == "" {
		http.Error(w, "Missing callid", http.StatusBadRequest)
		return
	}

	if err := s.manager.StartRecording(callID, fromTag, toTag); err != nil {
		s.logger.WithError(err).Error("Start recording failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.collector.IncrementRecordings()

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"recording_started"}`)
}

func (s *Server) stopRecordingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	callID := r.FormValue("callid")
	fromTag := r.FormValue("fromtag")
	toTag := r.FormValue("totag")

	if callID == "" {
		http.Error(w, "Missing callid", http.StatusBadRequest)
		return
	}

	if err := s.manager.StopRecording(callID, fromTag, toTag); err != nil {
		s.logger.WithError(err).Error("Stop recording failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"recording_stopped"}`)
}
