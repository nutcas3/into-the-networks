package internal

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nutcas3/esl-resilience/internal/esl"
	"github.com/nutcas3/esl-resilience/internal/monitor"
	"github.com/nutcas3/esl-resilience/internal/state"
	"github.com/sirupsen/logrus"
)

type Server struct {
	client       *esl.Client
	stateMachine *state.Machine
	buffer       *esl.Buffer
	monitor      *monitor.PrometheusMonitor
	logger       *logrus.Logger
}

type Config struct {
	FreeSWITCH struct {
		Host     string `env:"FREESWITCH_HOST" default:"localhost"`
		Port     int    `env:"FREESWITCH_PORT" default:"8021"`
		Password string `env:"FREESWITCH_PASSWORD" default:"ClueCon"`
	}
	ESL struct {
		MaxRetries     int           `env:"ESL_MAX_RETRIES" default:"10"`
		InitialBackoff time.Duration `env:"ESL_INITIAL_BACKOFF" default:"1s"`
		MaxBackoff     time.Duration `env:"ESL_MAX_BACKOFF" default:"60s"`
		BufferSize     int           `env:"ESL_BUFFER_SIZE" default:"10000"`
	}
	Monitor struct {
		Port int `env:"MONITOR_PORT" default:"9090"`
	}
}

func DefaultConfig() Config {
	return Config{
		FreeSWITCH: struct {
			Host     string `env:"FREESWITCH_HOST" default:"localhost"`
			Port     int    `env:"FREESWITCH_PORT" default:"8021"`
			Password string `env:"FREESWITCH_PASSWORD" default:"ClueCon"`
		}{
			Host:     "localhost",
			Port:     8021,
			Password: "ClueCon",
		},
		ESL: struct {
			MaxRetries     int           `env:"ESL_MAX_RETRIES" default:"10"`
			InitialBackoff time.Duration `env:"ESL_INITIAL_BACKOFF" default:"1s"`
			MaxBackoff     time.Duration `env:"ESL_MAX_BACKOFF" default:"60s"`
			BufferSize     int           `env:"ESL_BUFFER_SIZE" default:"10000"`
		}{
			MaxRetries:     10,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     60 * time.Second,
			BufferSize:     10000,
		},
		Monitor: struct {
			Port int `env:"MONITOR_PORT" default:"9090"`
		}{
			Port: 9090,
		},
	}
}

func NewServer(config Config) *Server {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})

	eslConfig := esl.DefaultConfig()
	eslConfig.Host = config.FreeSWITCH.Host
	eslConfig.Port = config.FreeSWITCH.Port
	eslConfig.Password = config.FreeSWITCH.Password
	eslConfig.MaxRetries = config.ESL.MaxRetries
	eslConfig.InitialBackoff = config.ESL.InitialBackoff
	eslConfig.MaxBackoff = config.ESL.MaxBackoff

	client := esl.NewClient(eslConfig)
	stateMachine := state.NewMachine()

	bufferConfig := esl.DefaultBufferConfig()
	bufferConfig.MaxSize = config.ESL.BufferSize
	buffer := esl.NewBuffer(bufferConfig)

	monitor := monitor.NewPrometheusMonitor()

	client.SetStateMachine(stateMachine)
	client.SetEventBuffer(buffer)
	client.SetMonitor(monitor)

	registerEventHandlers(client, logger)

	server := &Server{
		client:       client,
		stateMachine: stateMachine,
		buffer:       buffer,
		monitor:      monitor,
		logger:       logger,
	}

	return server
}

func (s *Server) Start(ctx context.Context) error {
	s.logger.WithFields(logrus.Fields{
		"freeswitch_host": s.client.GetConfig().Host,
		"freeswitch_port": s.client.GetConfig().Port,
		"max_retries":     s.client.GetConfig().MaxRetries,
		"buffer_size":     s.client.GetConfig().BufferSize,
	}).Info("Starting ESL Resilience Server")

	go func() {
		if err := s.monitor.Start(); err != nil {
			s.logger.WithError(err).Error("Monitor failed to start")
		}
	}()

	return s.client.Start(ctx)
}

func (s *Server) Stop() error {
	s.logger.Info("Shutting down ESL Resilience Server")

	var errors []error

	if err := s.client.Stop(); err != nil {
		errors = append(errors, fmt.Errorf("failed to stop client: %w", err))
	}

	if err := s.buffer.Stop(); err != nil {
		errors = append(errors, fmt.Errorf("failed to stop buffer: %w", err))
	}

	if err := s.monitor.Stop(); err != nil {
		errors = append(errors, fmt.Errorf("failed to stop monitor: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	s.logger.Info("Server stopped successfully")
	return nil
}

func (s *Server) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	stats["client"] = map[string]interface{}{
		"connected": s.client.IsConnected(),
		"metrics":   s.client.GetMetrics(),
	}

	stats["state_machine"] = s.stateMachine.GetStats()

	stats["buffer"] = s.buffer.GetStats()

	stats["monitor"] = map[string]interface{}{
		"running": s.monitor.IsRunning(),
	}

	return stats
}

func (s *Server) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		if err := s.Start(ctx); err != nil {
			errChan <- fmt.Errorf("server failed: %w", err)
		}
	}()

	select {
	case <-sigChan:
		s.logger.Info("Shutdown signal received")
		return s.Stop()

	case err := <-errChan:
		return err

	case <-ctx.Done():
		s.logger.Info("Context cancelled, shutting down")
		return s.Stop()
	}
}

func registerEventHandlers(client *esl.Client, logger *logrus.Logger) {
	client.OnEvent("CHANNEL_CREATE", func(event *esl.Event) {
		logger.WithFields(logrus.Fields{
			"uuid":   event.Headers["Unique-ID"],
			"caller": event.Headers["Caller-Username"],
			"callee": event.Headers["Caller-Destination-Number"],
		}).Info("Channel created")
	})

	client.OnEvent("CHANNEL_PROGRESS", func(event *esl.Event) {
		logger.WithFields(logrus.Fields{
			"uuid":   event.Headers["Unique-ID"],
			"status": "ringing",
		}).Info("Call progress")
	})

	client.OnEvent("CHANNEL_ANSWER", func(event *esl.Event) {
		logger.WithFields(logrus.Fields{
			"uuid":   event.Headers["Unique-ID"],
			"status": "answered",
		}).Info("Call answered")
	})

	client.OnEvent("CHANNEL_HANGUP_COMPLETE", func(event *esl.Event) {
		logger.WithFields(logrus.Fields{
			"uuid":         event.Headers["Unique-ID"],
			"hangup_cause": event.Headers["Hangup-Cause"],
			"duration":     event.Headers["variable_billsec"],
		}).Info("Call completed")
	})

	client.OnEvent("CHANNEL_EXECUTE_COMPLETE", func(event *esl.Event) {
		logger.WithFields(logrus.Fields{
			"uuid":         event.Headers["Unique-ID"],
			"application":  event.Headers["Application"],
			"hangup_cause": event.Headers["Hangup-Cause"],
		}).Warn("Call execution failed")
	})

	client.OnEvent("HEARTBEAT", func(event *esl.Event) {
		logger.Debug("Heartbeat received")
	})

	client.OnEvent("CUSTOM", func(event *esl.Event) {
		logger.WithFields(logrus.Fields{
			"uuid":           event.Headers["Unique-ID"],
			"event_subclass": event.Headers["Event-Subclass"],
		}).Debug("Custom event")
	})
}
