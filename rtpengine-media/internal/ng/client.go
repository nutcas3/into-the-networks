package ng

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Client handles communication with RTPengine using the ng protocol
type Client struct {
	addr        string
	conn        *net.UDPConn
	cookieStore map[string]chan Response
	mu          sync.RWMutex
	logger      *logrus.Logger
	closed      bool
}

// Response represents an ng protocol response from RTPengine
type Response struct {
	Result string          `json:"result"`
	Data   json.RawMessage `json:"data,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// Request represents an ng protocol request to RTPengine
type Request struct {
	Command   string            `json:"command"`
	CallID    string            `json:"call-id"`
	FromTag   string            `json:"from-tag,omitempty"`
	ToTag     string            `json:"to-tag,omitempty"`
	ViaBranch string            `json:"via-branch,omitempty"`
	Flags     []string          `json:"flags,omitempty"`
	Replace   []string          `json:"replace,omitempty"`
	Transport string            `json:"transport-protocol,omitempty"`
	MediaAddr string            `json:"media-address,omitempty"`
	Address   string            `json:"address,omitempty"`
	Port      int               `json:"port,omitempty"`
	ICE       string            `json:"ICE,omitempty"`
	DTLS      string            `json:"DTLS,omitempty"`
	SDPS      string            `json:"sdp,omitempty"`
	Directions []string         `json:"direction,omitempty"`
	Codec     map[string]string `json:"codec,omitempty"`
	Codecs    []string          `json:"codecs,omitempty"`
	Ptime     int               `json:"ptime,omitempty"`
	ReceivedFrom string         `json:"received-from,omitempty"`
}

// NewClient creates a new ng protocol client
func NewClient(addr string, logger *logrus.Logger) (*Client, error) {
	if logger == nil {
		logger = logrus.New()
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address %s: %w", addr, err)
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	client := &Client{
		addr:        addr,
		conn:        conn,
		cookieStore: make(map[string]chan Response),
		logger:      logger,
	}

	go client.readLoop()

	return client, nil
}

// Close closes the client connection
func (c *Client) Close() error {
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

// Send sends a request to RTPengine and waits for response
func (c *Client) Send(req Request) (Response, error) {
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

// Ping sends a ping request to check RTPengine availability
func (c *Client) Ping() error {
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

// Offer sends an offer to RTPengine (creates or updates a session)
func (c *Client) Offer(callID, fromTag string, sdp string, options map[string]interface{}) (Response, error) {
	req := Request{
		Command: "offer",
		CallID:  callID,
		FromTag: fromTag,
		SDPS:    sdp,
	}

	// Apply options
	if v, ok := options["direction"].([]string); ok {
		req.Directions = v
	}
	if v, ok := options["flags"].([]string); ok {
		req.Flags = v
	}
	if v, ok := options["replace"].([]string); ok {
		req.Replace = v
	}
	if v, ok := options["transport"].(string); ok {
		req.Transport = v
	}
	if v, ok := options["ICE"].(string); ok {
		req.ICE = v
	}
	if v, ok := options["DTLS"].(string); ok {
		req.DTLS = v
	}
	if v, ok := options["received-from"].(string); ok {
		req.ReceivedFrom = v
	}

	return c.Send(req)
}

// Answer sends an answer to RTPengine
func (c *Client) Answer(callID, fromTag, toTag string, sdp string, options map[string]interface{}) (Response, error) {
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
	if v, ok := options["replace"].([]string); ok {
		req.Replace = v
	}

	return c.Send(req)
}

// Delete deletes a media session
func (c *Client) Delete(callID, fromTag, toTag string) (Response, error) {
	req := Request{
		Command: "delete",
		CallID:  callID,
		FromTag: fromTag,
		ToTag:   toTag,
	}
	return c.Send(req)
}

// Query queries the status of a media session
func (c *Client) Query(callID, fromTag, toTag string) (Response, error) {
	req := Request{
		Command: "query",
		CallID:  callID,
		FromTag: fromTag,
		ToTag:   toTag,
	}
	return c.Send(req)
}

// List lists active sessions (with optional callid filter)
func (c *Client) List(callID string) (Response, error) {
	req := Request{
		Command: "list",
	}
	if callID != "" {
		req.CallID = callID
	}
	return c.Send(req)
}

// StartRecording starts recording a media session
func (c *Client) StartRecording(callID, fromTag, toTag string) (Response, error) {
	req := Request{
		Command: "start recording",
		CallID:  callID,
		FromTag: fromTag,
		ToTag:   toTag,
	}
	return c.Send(req)
}

// StopRecording stops recording a media session
func (c *Client) StopRecording(callID, fromTag, toTag string) (Response, error) {
	req := Request{
		Command: "stop recording",
		CallID:  callID,
		FromTag: fromTag,
		ToTag:   toTag,
	}
	return c.Send(req)
}

// readLoop continuously reads responses from RTPengine
func (c *Client) readLoop() {
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

// handleResponse parses and routes responses to waiting requests
func (c *Client) handleResponse(data []byte) error {
	// ng protocol format: cookie + space + json
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

// generateCookie creates a random cookie for request/response matching
func generateCookie() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// formatMessage formats a request into the ng protocol wire format
func formatMessage(cookie string, req Request) string {
	jsonData, _ := json.Marshal(req)
	return fmt.Sprintf("%s %s", cookie, jsonData)
}

// splitN splits byte slice by separator, max n parts
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
