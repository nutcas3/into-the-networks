package internal

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/nutcas3/esl-resilience/internal/cdr"
	"github.com/nutcas3/esl-resilience/internal/db"
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
	database     *db.Database
	cdrRepo      *cdr.Repository
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
	Database struct {
		Host     string `env:"DB_HOST" default:"postgres"`
		Port     int    `env:"DB_PORT" default:"5432"`
		Username string `env:"DB_USERNAME" default:"freeswitch"`
		Password string `env:"DB_PASSWORD" default:"freeswitch_pass"`
		Database string `env:"DB_DATABASE" default:"freeswitch_cdr"`
		SSLMode  string `env:"DB_SSL_MODE" default:"disable"`
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
		Database: struct {
			Host     string `env:"DB_HOST" default:"postgres"`
			Port     int    `env:"DB_PORT" default:"5432"`
			Username string `env:"DB_USERNAME" default:"freeswitch"`
			Password string `env:"DB_PASSWORD" default:"freeswitch_pass"`
			Database string `env:"DB_DATABASE" default:"freeswitch_cdr"`
			SSLMode  string `env:"DB_SSL_MODE" default:"disable"`
		}{
			Host:     "postgres",
			Port:     5432,
			Username: "freeswitch",
			Password: "freeswitch_pass",
			Database: "freeswitch_cdr",
			SSLMode:  "disable",
		},
	}
}

func NewServer(config Config) *Server {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Initialize database
	dbConfig := db.Config{
		Host:     config.Database.Host,
		Port:     config.Database.Port,
		Username: config.Database.Username,
		Password: config.Database.Password,
		Database: config.Database.Database,
		SSLMode:  config.Database.SSLMode,
	}

	database, err := db.NewDatabase(dbConfig)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize database")
	}

	// Initialize CDR repository
	cdrRepo := cdr.NewRepository(database.GetDB())

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

	registerEventHandlers(client, logger, cdrRepo)

	server := &Server{
		client:       client,
		stateMachine: stateMachine,
		buffer:       buffer,
		monitor:      monitor,
		database:     database,
		cdrRepo:      cdrRepo,
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

	if err := s.database.Close(); err != nil {
		errors = append(errors, fmt.Errorf("failed to close database: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	s.logger.Info("Server stopped successfully")
	return nil
}

func (s *Server) GetStats() map[string]any {
	stats := make(map[string]any)

	stats["client"] = map[string]any{
		"connected": s.client.IsConnected(),
		"metrics":   s.client.GetMetrics(),
	}

	stats["state_machine"] = s.stateMachine.GetStats()

	stats["buffer"] = s.buffer.GetStats()

	stats["monitor"] = map[string]any{
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

func registerEventHandlers(client *esl.Client, logger *logrus.Logger, cdrRepo *cdr.Repository) {
	client.OnEvent("CHANNEL_CREATE", func(event *esl.Event) {
		logger.WithFields(logrus.Fields{
			"uuid":   event.Headers["Unique-ID"],
			"caller": event.Headers["Caller-Username"],
			"callee": event.Headers["Caller-Destination-Number"],
		}).Info("Channel created")

		// Create CDR record
		channelUUID, err := uuid.Parse(event.Headers["Unique-ID"])
		if err != nil {
			logger.WithError(err).Error("Failed to parse channel UUID")
			return
		}

		cdr := &cdr.CallDetailRecord{
			ID:                channelUUID,
			AccountCode:       event.Headers["Account-Code"],
			CallerIDName:      event.Headers["Caller-ID-Name"],
			CallerIDNumber:    event.Headers["Caller-ID-Number"],
			DestinationNumber: event.Headers["Caller-Destination-Number"],
			StartTimestamp:    time.Now(),
			ChannelUUID:       channelUUID,
			Context:           event.Headers["Context"],
			CreatedAt:         time.Now(),
		}

		if err := cdrRepo.CreateCDR(context.Background(), cdr); err != nil {
			logger.WithError(err).Error("Failed to create CDR")
		}
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

		// Update CDR with answer timestamp
		channelUUID, err := uuid.Parse(event.Headers["Unique-ID"])
		if err != nil {
			logger.WithError(err).Error("Failed to parse channel UUID")
			return
		}

		updates := map[string]any{
			"answer_timestamp": time.Now(),
		}

		if err := cdrRepo.UpdateCDR(context.Background(), channelUUID, updates); err != nil {
			logger.WithError(err).Error("Failed to update CDR answer timestamp")
		}
	})

	client.OnEvent("CHANNEL_HANGUP_COMPLETE", func(event *esl.Event) {
		logger.WithFields(logrus.Fields{
			"uuid":         event.Headers["Unique-ID"],
			"hangup_cause": event.Headers["Hangup-Cause"],
			"duration":     event.Headers["variable_billsec"],
		}).Info("Call completed")

		// Update CDR with end timestamp and final details
		channelUUID, err := uuid.Parse(event.Headers["Unique-ID"])
		if err != nil {
			logger.WithError(err).Error("Failed to parse channel UUID")
			return
		}

		updates := map[string]any{
			"end_timestamp": time.Now(),
			"hangup_cause":  event.Headers["Hangup-Cause"],
		}

		// Parse duration if available
		if billsec := event.Headers["variable_billsec"]; billsec != "" {
			if duration, err := time.ParseDuration(billsec + "s"); err == nil {
				updates["billsec_seconds"] = int64(duration.Seconds())
				updates["duration_seconds"] = int64(duration.Seconds())
			}
		}

		// Parse destination channel UUID if available
		if destUUID := event.Headers["Bridge-B-Unique-ID"]; destUUID != "" {
			if parsedUUID, err := uuid.Parse(destUUID); err == nil {
				updates["destination_channel_uuid"] = parsedUUID.String()
			}
		}

		if err := cdrRepo.UpdateCDR(context.Background(), channelUUID, updates); err != nil {
			logger.WithError(err).Error("Failed to update CDR completion details")
		}
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
