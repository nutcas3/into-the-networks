package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type Database struct {
	db     *sql.DB
	logger *logrus.Logger
}

type Config struct {
	Host     string `env:"DB_HOST" default:"localhost"`
	Port     int    `env:"DB_PORT" default:"5432"`
	Username string `env:"DB_USERNAME" default:"freeswitch"`
	Password string `env:"DB_PASSWORD" default:"freeswitch_pass"`
	Database string `env:"DB_DATABASE" default:"freeswitch_cdr"`
	SSLMode  string `env:"DB_SSL_MODE" default:"disable"`
}

func DefaultConfig() Config {
	return Config{
		Host:     "localhost",
		Port:     5432,
		Username: "freeswitch",
		Password: "freeswitch_pass",
		Database: "freeswitch_cdr",
		SSLMode:  "disable",
	}
}

func NewDatabase(config Config) (*Database, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.Username, config.Password, config.Database, config.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"host":     config.Host,
		"port":     config.Port,
		"database": config.Database,
	}).Info("Database connection established")

	return &Database{
		db:     db,
		logger: logger,
	}, nil
}

func (d *Database) Close() error {
	if err := d.db.Close(); err != nil {
		d.logger.WithError(err).Error("Failed to close database connection")
		return err
	}
	d.logger.Info("Database connection closed")
	return nil
}

func (d *Database) GetDB() *sql.DB {
	return d.db
}

func (d *Database) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := d.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	return nil
}

func (d *Database) Stats() map[string]interface{} {
	stats := d.db.Stats()
	return map[string]interface{}{
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration.String(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	}
}
