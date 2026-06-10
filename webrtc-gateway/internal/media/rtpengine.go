package media

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ngClient handles communication with RTPengine
type ngClient struct {
	addr        string
	conn        *net.UDPConn
	cookieStore map[string]chan Response
	mu          sync.RWMutex
	logger      *logrus.Logger
	closed      bool
}

// Response represents an ng protocol response
type Response struct {
	Result string          `json:"result"`
	Data   json.RawMessage `json:"data,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Request represents an ng protocol request
type Request struct {
	Command   string   `json:"command"`
	CallID    string   `json:"call-id"`
	FromTag   string   `json:"from-tag,omitempty"`
	ToTag     string   `json:"to-tag,omitempty"`
	SDPS      string   `json:"sdp,omitempty"`
	Flags     []string `json:"flags,omitempty"`
	Replace   []string `json:"replace,omitempty"`
	Transport string   `json:"transport-protocol,omitempty"`
	ICE       string   `json:"ICE,omitempty"`
	DTLS      string   `json:"DTLS,omitempty"`
}

// newNGClient creates a new ng protocol client
func newNGClient(addr string, logger *logrus.Logger) (*ngClient, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address %s: %w", addr, err)
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	client := &ngClient{
		addr:        addr,
		conn:        conn,
		cookieStore: make(map[string]chan Response),
		logger:      logger,
	}

	go client.readLoop()

	return client, nil
}

func (c *ngClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	closeAll := make([]chan Response, 0, len(c.cookieStore))
	for _, ch := range c.cookieStore {
		closeAll = append(closeAll, ch)
	}

	go func() {
		for _, ch := range closeAll {
			close(ch)
		}
	}()

	return c.conn.Close()
}

func (c *ngClient) Send(req Request) (Response, error) {
	cookie := generateCookie()
	respCh := make(chan Response, 1)

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return Response{}, fmt.Errorf("client is closed")
	}
	c.cookieStore[cookie] = respCh
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.cookieStore, cookie)
		c.mu.Unlock()
	}()

	msg := formatMessage(cookie, req)
	if _, err := c.conn.Write([]byte(msg)); err != nil {
		return Response{}, fmt.Errorf("failed to send request: %w", err)
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return Response{}, fmt.Errorf("client closed")
		}
		return resp, nil
	case <-time.After(5 * time.Second):
		return Response{}, fmt.Errorf("timeout waiting for response")
	}
}

func (c *ngClient) Ping() error {
	req := Request{Command: "ping"}
	resp, err := c.Send(req)
	if err != nil {
		return err
	}
	if resp.Result != "pong" {
		return fmt.Errorf("unexpected ping response: %s", resp.Result)
	}
	return nil
}

func (c *ngClient) Offer(callID, fromTag string, sdp string, options map[string]interface{}) (Response, error) {
	req := Request{
		Command: "offer",
		CallID:  callID,
		FromTag: fromTag,
		SDPS:    sdp,
	}

	if v, ok := options["flags"].([]string); ok {
		req.Flags = v
	}
	if v, ok := options["ICE"].(string); ok {
		req.ICE = v
	}
	if v, ok := options["DTLS"].(string); ok {
		req.DTLS = v
	}

	return c.Send(req)
}

func (c *ngClient) Answer(callID, fromTag, toTag string, sdp string, options map[string]interface{}) (Response, error) {
	req := Request{
		Command: "answer",
		CallID:  callID,
		FromTag: fromTag,
		ToTag:   toTag,
		SDPS:    sdp,
	}

	if v, ok := options["flags"].([]string); ok {
		req.Flags = v
	}

	return c.Send(req)
}

func (c *ngClient) Delete(callID, fromTag, toTag string) (Response, error) {
	req := Request{
		Command: "delete",
		CallID:  callID,
		FromTag: fromTag,
		ToTag:   toTag,
	}
	return c.Send(req)
}

func (c *ngClient) Query(callID, fromTag, toTag string) (Response, error) {
	req := Request{
		Command: "query",
		CallID:  callID,
		FromTag: fromTag,
		ToTag:   toTag,
	}
	return c.Send(req)
}

func (c *ngClient) readLoop() {
	buf := make([]byte, 65536)
	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			c.mu.RLock()
			closed := c.closed
			c.mu.RUnlock()
			if closed {
				return
			}
			c.logger.WithError(err).Error("Failed to read from UDP socket")
			continue
		}

		if err := c.handleResponse(buf[:n]); err != nil {
			c.logger.WithError(err).Warn("Failed to handle response")
		}
	}
}

func (c *ngClient) handleResponse(data []byte) error {
	parts := splitN(data, ' ', 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid message format")
	}

	cookie := string(parts[0])
	var resp Response
	if err := json.Unmarshal(parts[1], &resp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.mu.RLock()
	respCh, exists := c.cookieStore[cookie]
	c.mu.RUnlock()

	if exists {
		select {
		case respCh <- resp:
		default:
		}
	}

	return nil
}

func generateCookie() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func formatMessage(cookie string, req Request) string {
	jsonData, _ := json.Marshal(req)
	return fmt.Sprintf("%s %s", cookie, jsonData)
}

func splitN(data []byte, sep byte, n int) [][]byte {
	var parts [][]byte
	start := 0
	for i := 0; i < len(data) && len(parts) < n-1; i++ {
		if data[i] == sep {
			parts = append(parts, data[start:i])
			start = i + 1
		}
	}
	parts = append(parts, data[start:])
	return parts
}

// Manager handles media relay via RTPengine
type Manager struct {
	ngClient *ngClient
	sessions map[string]*MediaSession
	mu       sync.RWMutex
	logger   *logrus.Logger
	ctx      context.Context
	cancel   context.CancelFunc
}

// MediaSession represents a media session with RTPengine
type MediaSession struct {
	ID        string
	CallID    string
	FromTag   string
	ToTag     string
	LocalSDP  string
	RemoteSDP string
	State     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Config holds media manager configuration
type Config struct {
	RTPEngineAddress string
	Logger           *logrus.Logger
}

// NewManager creates a new media manager
func NewManager(cfg Config) (*Manager, error) {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}

	client, err := newNGClient(cfg.RTPEngineAddress, cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create ng client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		ngClient: client,
		sessions: make(map[string]*MediaSession),
		logger:   cfg.Logger,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Verify RTPengine connectivity
	if err := client.Ping(); err != nil {
		client.Close()
		return nil, fmt.Errorf("rtpengine not responding: %w", err)
	}

	m.logger.Info("Connected to RTPengine")

	return m, nil
}

// Close shuts down the media manager
func (m *Manager) Close() error {
	m.cancel()

	// Delete all active sessions
	m.mu.Lock()
	sessions := make([]*MediaSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()

	for _, s := range sessions {
		if err := m.Delete(s.CallID, s.FromTag, s.ToTag); err != nil {
			m.logger.WithError(err).WithField("callid", s.CallID).Warn("Failed to delete session during shutdown")
		}
	}

	return m.ngClient.Close()
}

// Offer sends an SDP offer to RTPengine
func (m *Manager) Offer(callID, fromTag, sdp string, options map[string]interface{}) (string, error) {
	resp, err := m.ngClient.Offer(callID, fromTag, sdp, options)
	if err != nil {
		return "", fmt.Errorf("offer failed: %w", err)
	}

	if resp.Result != "ok" {
		return "", fmt.Errorf("offer rejected: %s", resp.Error)
	}

	var resultData struct {
		SDP string `json:"sdp"`
	}
	if err := json.Unmarshal(resp.Data, &resultData); err != nil {
		m.logger.WithError(err).Warn("Failed to parse offer response")
		return sdp, nil // Return original SDP on parse error
	}

	session := &MediaSession{
		ID:        callID,
		CallID:    callID,
		FromTag:   fromTag,
		LocalSDP:  resultData.SDP,
		RemoteSDP: sdp,
		State:     "offered",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.mu.Lock()
	m.sessions[callID] = session
	m.mu.Unlock()

	m.logger.WithFields(logrus.Fields{
		"callid":  callID,
		"fromtag": fromTag,
	}).Info("Created media session offer")

	return resultData.SDP, nil
}

// Answer sends an SDP answer to RTPengine
func (m *Manager) Answer(callID, fromTag, toTag, sdp string, options map[string]interface{}) (string, error) {
	resp, err := m.ngClient.Answer(callID, fromTag, toTag, sdp, options)
	if err != nil {
		return "", fmt.Errorf("answer failed: %w", err)
	}

	if resp.Result != "ok" {
		return "", fmt.Errorf("answer rejected: %s", resp.Error)
	}

	var resultData struct {
		SDP string `json:"sdp"`
	}
	if err := json.Unmarshal(resp.Data, &resultData); err != nil {
		m.logger.WithError(err).Warn("Failed to parse answer response")
		return sdp, nil
	}

	m.mu.Lock()
	session, exists := m.sessions[callID]
	if exists {
		session.ToTag = toTag
		session.LocalSDP = resultData.SDP
		session.RemoteSDP = sdp
		session.State = "active"
		session.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	m.logger.WithFields(logrus.Fields{
		"callid": callID,
		"totag":  toTag,
	}).Info("Session established")

	return resultData.SDP, nil
}

// Delete removes a media session
func (m *Manager) Delete(callID, fromTag, toTag string) error {
	resp, err := m.ngClient.Delete(callID, fromTag, toTag)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	if resp.Result != "ok" && resp.Result != "deleted" {
		m.logger.WithFields(logrus.Fields{
			"callid": callID,
			"result": resp.Result,
		}).Warn("Unexpected delete response")
	}

	m.mu.Lock()
	delete(m.sessions, callID)
	m.mu.Unlock()

	m.logger.WithField("callid", callID).Info("Deleted media session")
	return nil
}

// Query queries the status of a media session
func (m *Manager) Query(callID, fromTag, toTag string) (map[string]interface{}, error) {
	resp, err := m.ngClient.Query(callID, fromTag, toTag)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if resp.Result != "ok" {
		return nil, fmt.Errorf("query rejected: %s", resp.Error)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse query response: %w", err)
	}

	return result, nil
}

// GetSession returns a media session by call ID
func (m *Manager) GetSession(callID string) (*MediaSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, exists := m.sessions[callID]
	return session, exists
}
