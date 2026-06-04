package esl

import (
	"context"
	"net"
	"sync"
	"time"
)

type Client struct {
	config       Config
	conn         net.Conn
	stateMachine StateMachine
	monitor      Monitor
	buffer       *Buffer

	// Resilience features
	reconnectAttempts int
	circuitBreaker    *CircuitBreaker
	healthChecker     *HealthChecker

	// Synchronization
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	reconnecting bool

	// Event handling
	eventHandlers map[string]EventHandler
	eventChan     chan *Event

	// Metrics
	metrics *Metrics
}

type Config struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	Password       string        `yaml:"password"`
	Timeout        time.Duration `yaml:"timeout"`
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`

	// Resilience settings
	MaxRetries        int           `yaml:"max_retries"`
	InitialBackoff    time.Duration `yaml:"initial_backoff"`
	MaxBackoff        time.Duration `yaml:"max_backoff"`
	BackoffMultiplier float64       `yaml:"backoff_multiplier"`

	// Circuit breaker
	CircuitBreakerThreshold int           `yaml:"circuit_breaker_threshold"`
	CircuitBreakerTimeout   time.Duration `yaml:"circuit_breaker_timeout"`

	// Health checking
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`
	HealthCheckTimeout  time.Duration `yaml:"health_check_timeout"`

	// Event buffering
	BufferSize      int           `yaml:"buffer_size"`
	BufferFlushTime time.Duration `yaml:"buffer_flush_time"`
}

type EventHandler func(*Event)

type StateMachine interface {
	HandleEvent(event *Event) error
	GetCallState(uuid string) (CallState, bool)
	ActiveCalls() int
}

type Monitor interface {
	RecordConnection(status bool)
	RecordEvent(eventType string)
	RecordReconnection(attempt int)
	RecordError(err error)
	IncrementCounter(name string, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
}

type EventBuffer interface {
	Enqueue(event *Event) error
	Dequeue() (*Event, bool)
	Size() int
	Flush() ([]*Event, error)
	Clear() error
}

type CallState string

const (
	CallStateInit        CallState = "CALL_INIT"
	CallStateEarly       CallState = "CALL_EARLY"
	CallStateConfirmed   CallState = "CALL_CONFIRMED"
	CallStateEstablished CallState = "CALL_ESTABLISHED"
	CallStateTerminated  CallState = "CALL_TERMINATED"
	CallStateFailed      CallState = "CALL_FAILED"
)

type Event struct {
	Headers    map[string]string
	Body       []byte
	ReceivedAt time.Time
}
