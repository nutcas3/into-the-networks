package esl

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

func DefaultConfig() Config {
	return Config{
		Host:                    "localhost",
		Port:                    8021,
		Password:                "ClueCon",
		Timeout:                 5 * time.Second,
		ConnectTimeout:          10 * time.Second,
		ReadTimeout:             30 * time.Second,
		WriteTimeout:            10 * time.Second,
		MaxRetries:              10,
		InitialBackoff:          1 * time.Second,
		MaxBackoff:              60 * time.Second,
		BackoffMultiplier:       2.0,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   30 * time.Second,
		HealthCheckInterval:     30 * time.Second,
		HealthCheckTimeout:      5 * time.Second,
		BufferSize:              10000,
		BufferFlushTime:         1 * time.Second,
	}
}

func NewClient(config Config) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		config:        config,
		ctx:           ctx,
		cancel:        cancel,
		eventHandlers: make(map[string]EventHandler),
		eventChan:     make(chan *Event, 1000),
		metrics:       NewMetrics(),
	}

	// Initialize resilience components
	client.circuitBreaker = NewCircuitBreaker(config.CircuitBreakerThreshold, config.CircuitBreakerTimeout)
	client.healthChecker = NewHealthChecker(config.HealthCheckInterval, config.HealthCheckTimeout)

	return client
}

func (c *Client) SetStateMachine(sm StateMachine) {
	c.stateMachine = sm
}

func (c *Client) SetMonitor(monitor Monitor) {
	c.monitor = monitor
}

func (c *Client) SetEventBuffer(buffer *Buffer) {
	c.buffer = buffer
}

func (c *Client) OnEvent(eventType string, handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eventHandlers[eventType] = handler
}

func (c *Client) Start(ctx context.Context) error {
	logrus.WithFields(logrus.Fields{
		"host": c.config.Host,
		"port": c.config.Port,
	}).Info("Starting ESL client")

	go c.connectionManager()
	go c.eventProcessor()
	go c.healthChecker.Start(ctx, c)

	return nil
}

func (c *Client) Stop() error {
	logrus.Info("Stopping ESL client")

	c.cancel()

	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	return nil
}

func (c *Client) connectionManager() {
	backoff := c.config.InitialBackoff

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if c.circuitBreaker.IsOpen() {
				logrus.Warn("Circuit breaker is open, waiting...")
				time.Sleep(c.config.CircuitBreakerTimeout)
				continue
			}

			if err := c.connect(); err != nil {
				c.metrics.IncrementCounter("esl_connection_failures_total", nil)
				c.recordError(err)

				if c.monitor != nil {
					c.monitor.RecordReconnection(c.reconnectAttempts)
				}

				c.reconnectAttempts++
				if c.reconnectAttempts >= c.config.MaxRetries {
					logrus.WithField("attempts", c.reconnectAttempts).Error("Max reconnection attempts reached")
					return
				}

				time.Sleep(backoff)
				backoff = time.Duration(float64(backoff) * c.config.BackoffMultiplier)
				if backoff > c.config.MaxBackoff {
					backoff = c.config.MaxBackoff
				}

				continue
			}

			c.reconnectAttempts = 0
			backoff = c.config.InitialBackoff
			c.circuitBreaker.RecordSuccess()

			if c.monitor != nil {
				c.monitor.RecordConnection(true)
			}

			if err := c.processEvents(); err != nil {
				logrus.WithError(err).Error("Event processing failed")
				c.circuitBreaker.RecordFailure()
			}

			if c.monitor != nil {
				c.monitor.RecordConnection(false)
			}
		}
	}
}

func (c *Client) connect() error {
	if c.reconnecting {
		return fmt.Errorf("reconnection already in progress")
	}

	c.mu.Lock()
	c.reconnecting = true
	defer func() {
		c.reconnecting = false
		c.mu.Unlock()
	}()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	address := net.JoinHostPort(c.config.Host, fmt.Sprintf("%d", c.config.Port))

	dialer := &net.Dialer{
		Timeout: c.config.ConnectTimeout,
	}

	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	c.conn = conn

	if err := c.authenticate(); err != nil {
		conn.Close()
		return fmt.Errorf("authentication failed: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"address": address,
		"attempt": c.reconnectAttempts + 1,
	}).Info("ESL connection established")

	if c.buffer != nil {
		if err := c.replayBufferedEvents(); err != nil {
			logrus.WithError(err).Warn("Failed to replay buffered events")
		}
	}

	return nil
}

func (c *Client) authenticate() error {
	authCmd := fmt.Sprintf("auth %s\n\n", c.config.Password)

	if err := c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout)); err != nil {
		return err
	}

	if _, err := c.conn.Write([]byte(authCmd)); err != nil {
		return err
	}

	if err := c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout)); err != nil {
		return err
	}

	response := make([]byte, 1024)
	n, err := c.conn.Read(response)
	if err != nil {
		return err
	}

	respStr := string(response[:n])
	if !containsSuccess(respStr) {
		return fmt.Errorf("authentication failed: %s", respStr)
	}

	eventCmd := "event plain CHANNEL_CREATE\n\n"
	if _, err := c.conn.Write([]byte(eventCmd)); err != nil {
		return err
	}

	return nil
}

func (c *Client) processEvents() error {
	reader := NewEventReader(c.conn)

	for {
		select {
		case <-c.ctx.Done():
			return nil
		default:
			event, err := reader.ReadEvent()
			if err != nil {
				return err
			}

			select {
			case c.eventChan <- event:
			default:
				if c.buffer != nil {
					if err := c.buffer.Enqueue(event); err != nil {
						logrus.WithError(err).Error("Failed to buffer event")
					}
				}
			}
		}
	}
}

func (c *Client) eventProcessor() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case event := <-c.eventChan:
			c.handleEvent(event)
		}
	}
}

func (c *Client) handleEvent(event *Event) {
	eventType := event.Headers["Event-Name"]

	if c.monitor != nil {
		c.monitor.RecordEvent(eventType)
	}

	c.metrics.IncrementCounter("esl_events_processed_total", map[string]string{
		"event_type": eventType,
	})

	if c.stateMachine != nil {
		if err := c.stateMachine.HandleEvent(event); err != nil {
			logrus.WithFields(logrus.Fields{
				"event_type": eventType,
				"error":      err,
			}).Error("State machine failed to handle event")
			c.recordError(err)
		}
	}

	c.mu.RLock()
	handler, exists := c.eventHandlers[eventType]
	c.mu.RUnlock()

	if exists {
		handler(event)
	}
}

func (c *Client) replayBufferedEvents() error {
	if c.buffer == nil {
		return nil
	}

	events, err := c.buffer.Flush()
	if err != nil {
		return err
	}

	for _, event := range events {
		select {
		case c.eventChan <- event:
		default:
			c.buffer.Enqueue(event)
		}
	}

	logrus.WithField("count", len(events)).Info("Replayed buffered events")
	return nil
}

func (c *Client) recordError(err error) {
	if c.monitor != nil {
		c.monitor.RecordError(err)
	}
	c.metrics.IncrementCounter("esl_errors_total", map[string]string{
		"error_type": fmt.Sprintf("%T", err),
	})
}

func (c *Client) SendCommand(cmd string) (string, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return "", fmt.Errorf("not connected")
	}

	if err := conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout)); err != nil {
		return "", err
	}

	if _, err := conn.Write([]byte(cmd + "\n\n")); err != nil {
		return "", err
	}

	if err := conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout)); err != nil {
		return "", err
	}

	response := make([]byte, 4096)
	n, err := conn.Read(response)
	if err != nil {
		return "", err
	}

	return string(response[:n]), nil
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil
}

func (c *Client) GetConfig() Config {
	return c.config
}

func (c *Client) GetMetrics() map[string]interface{} {
	return c.metrics.GetAll()
}

func containsSuccess(response string) bool {
	return len(response) > 0 && response[0] == '+'
}
