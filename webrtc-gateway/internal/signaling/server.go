package signaling

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// MessageType represents the type of signaling message
type MessageType string

const (
	MessageTypeOffer    MessageType = "offer"
	MessageTypeAnswer   MessageType = "answer"
	MessageTypeICE      MessageType = "ice"
	MessageTypeHangup   MessageType = "hangup"
	MessageTypeRegister MessageType = "register"
	MessageTypeCall     MessageType = "call"
	MessageTypeAccept   MessageType = "accept"
	MessageTypeReject   MessageType = "reject"
	MessageTypePing     MessageType = "ping"
	MessageTypePong     MessageType = "pong"
)

// Message represents a signaling message
type Message struct {
	Type      MessageType `json:"type"`
	From      string      `json:"from,omitempty"`
	To        string      `json:"to,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
	SDP       string      `json:"sdp,omitempty"`
	ICE       *ICEMessage `json:"ice,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// ICEMessage represents ICE candidate information
type ICEMessage struct {
	Candidate     string `json:"candidate"`
	SDPMLineIndex int    `json:"sdpMLineIndex"`
	SDPMid        string `json:"sdpMid"`
}

// Client represents a connected WebSocket client
type Client struct {
	ID         string
	UserID     string
	Conn       *websocket.Conn
	Send       chan Message
	Registered bool
	mu         sync.Mutex
}

// Server handles WebSocket signaling
type Server struct {
	clients    map[string]*Client
	sessions   map[string]*Session
	mu         sync.RWMutex
	logger     *logrus.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	upgrader   websocket.Upgrader
	sipHandler SIPHandler
}

// Session represents an active call session
type Session struct {
	ID        string
	Caller    string
	Callee    string
	OfferSDP  string
	AnswerSDP string
	State     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SIPHandler interface for SIP integration
type SIPHandler interface {
	Invite(caller, callee, sdp string) (string, error)
	Answer(sessionID string, sdp string) error
	Hangup(sessionID string) error
}

// NewServer creates a new signaling server
func NewServer(logger *logrus.Logger, sipHandler SIPHandler) *Server {
	if logger == nil {
		logger = logrus.New()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		clients:  make(map[string]*Client),
		sessions: make(map[string]*Session),
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
		sipHandler: sipHandler,
	}
}

// Close shuts down the server
func (s *Server) Close() {
	s.cancel()
	s.mu.Lock()
	for _, client := range s.clients {
		client.Conn.Close()
	}
	s.mu.Unlock()
}

// HandleConnection handles a new WebSocket connection
func (s *Server) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.WithError(err).Error("Failed to upgrade connection")
		return
	}

	clientID := generateID()
	client := &Client{
		ID:   clientID,
		Conn: conn,
		Send: make(chan Message, 256),
	}

	s.mu.Lock()
	s.clients[clientID] = client
	s.mu.Unlock()

	s.logger.WithField("client_id", clientID).Info("Client connected")

	// Start message handler
	go s.readMessages(client)
	go s.writeMessages(client)
}

// readMessages reads messages from the WebSocket
func (s *Server) readMessages(client *Client) {
	defer func() {
		client.Conn.Close()
		s.mu.Lock()
		delete(s.clients, client.ID)
		s.mu.Unlock()
		close(client.Send)
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			var msg Message
			if err := client.Conn.ReadJSON(&msg); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					s.logger.WithError(err).WithField("client_id", client.ID).Error("Read error")
				}
				return
			}

			msg.Timestamp = time.Now()
			s.handleMessage(client, msg)
		}
	}
}

// writeMessages writes messages to the WebSocket
func (s *Server) writeMessages(client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-client.Send:
			if !ok {
				return
			}
			if err := client.Conn.WriteJSON(msg); err != nil {
				s.logger.WithError(err).WithField("client_id", client.ID).Error("Write error")
				return
			}
		case <-ticker.C:
			// Send ping
			ping := Message{Type: MessageTypePing, Timestamp: time.Now()}
			if err := client.Conn.WriteJSON(ping); err != nil {
				return
			}
		case <-s.ctx.Done():
			return
		}
	}
}

// handleMessage processes incoming messages
func (s *Server) handleMessage(client *Client, msg Message) {
	switch msg.Type {
	case MessageTypeRegister:
		s.handleRegister(client, msg)
	case MessageTypeCall:
		s.handleCall(client, msg)
	case MessageTypeOffer:
		s.handleOffer(client, msg)
	case MessageTypeAnswer:
		s.handleAnswer(client, msg)
	case MessageTypeICE:
		s.handleICE(client, msg)
	case MessageTypeHangup:
		s.handleHangup(client, msg)
	case MessageTypePong:
		// Pong received, connection is alive
	default:
		s.logger.WithField("type", msg.Type).Warn("Unknown message type")
	}
}

// handleRegister handles user registration
func (s *Server) handleRegister(client *Client, msg Message) {
	client.mu.Lock()
	client.UserID = msg.From
	client.Registered = true
	client.mu.Unlock()

	s.logger.WithFields(logrus.Fields{
		"client_id": client.ID,
		"user_id":   msg.From,
	}).Info("User registered")

	// Send confirmation
	response := Message{
		Type:      MessageTypeRegister,
		From:      "server",
		To:        msg.From,
		Timestamp: time.Now(),
	}
	client.Send <- response
}

// handleCall handles outgoing call initiation
func (s *Server) handleCall(client *Client, msg Message) {
	if !client.Registered {
		s.logger.WithField("client_id", client.ID).Warn("Unregistered client attempted to call")
		return
	}

	sessionID := generateID()
	session := &Session{
		ID:        sessionID,
		Caller:    msg.From,
		Callee:    msg.To,
		State:     "initiating",
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	s.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"caller":     msg.From,
		"callee":     msg.To,
	}).Info("Call initiated")

	// Forward to callee
	s.mu.RLock()
	for _, c := range s.clients {
		if c.UserID == msg.To && c.Registered {
			callMsg := Message{
				Type:      MessageTypeCall,
				From:      msg.From,
				To:        msg.To,
				SessionID: sessionID,
				Timestamp: time.Now(),
			}
			c.Send <- callMsg
			break
		}
	}
	s.mu.RUnlock()
}

// handleOffer handles WebRTC offer
func (s *Server) handleOffer(client *Client, msg Message) {
	s.mu.Lock()
	session, exists := s.sessions[msg.SessionID]
	if !exists {
		s.mu.Unlock()
		s.logger.WithField("session_id", msg.SessionID).Warn("Session not found for offer")
		return
	}
	session.OfferSDP = msg.SDP
	session.State = "offered"
	session.UpdatedAt = time.Now()
	s.mu.Unlock()

	// Send offer to callee
	s.mu.RLock()
	for _, c := range s.clients {
		if c.UserID == session.Callee && c.Registered {
			offerMsg := Message{
				Type:      MessageTypeOffer,
				From:      session.Caller,
				To:        session.Callee,
				SessionID: msg.SessionID,
				SDP:       msg.SDP,
				Timestamp: time.Now(),
			}
			c.Send <- offerMsg
			break
		}
	}
	s.mu.RUnlock()
}

// handleAnswer handles WebRTC answer
func (s *Server) handleAnswer(client *Client, msg Message) {
	s.mu.Lock()
	session, exists := s.sessions[msg.SessionID]
	if !exists {
		s.mu.Unlock()
		s.logger.WithField("session_id", msg.SessionID).Warn("Session not found for answer")
		return
	}
	session.AnswerSDP = msg.SDP
	session.State = "answered"
	session.UpdatedAt = time.Now()
	s.mu.Unlock()

	// Send answer to caller
	s.mu.RLock()
	for _, c := range s.clients {
		if c.UserID == session.Caller && c.Registered {
			answerMsg := Message{
				Type:      MessageTypeAnswer,
				From:      session.Callee,
				To:        session.Caller,
				SessionID: msg.SessionID,
				SDP:       msg.SDP,
				Timestamp: time.Now(),
			}
			c.Send <- answerMsg
			break
		}
	}
	s.mu.RUnlock()

	// Send answer to SIP handler
	if s.sipHandler != nil {
		if err := s.sipHandler.Answer(msg.SessionID, msg.SDP); err != nil {
			s.logger.WithError(err).Error("Failed to send answer to SIP")
		}
	}
}

// handleICE handles ICE candidate exchange
func (s *Server) handleICE(client *Client, msg Message) {
	s.mu.RLock()
	session, exists := s.sessions[msg.SessionID]
	s.mu.RUnlock()

	if !exists {
		s.logger.WithField("session_id", msg.SessionID).Warn("Session not found for ICE")
		return
	}

	// Forward ICE candidate to the other party
	var targetUserID string
	if client.UserID == session.Caller {
		targetUserID = session.Callee
	} else {
		targetUserID = session.Caller
	}

	s.mu.RLock()
	for _, c := range s.clients {
		if c.UserID == targetUserID && c.Registered {
			iceMsg := Message{
				Type:      MessageTypeICE,
				From:      client.UserID,
				To:        targetUserID,
				SessionID: msg.SessionID,
				ICE:       msg.ICE,
				Timestamp: time.Now(),
			}
			c.Send <- iceMsg
			break
		}
	}
	s.mu.RUnlock()
}

// handleHangup handles call termination
func (s *Server) handleHangup(client *Client, msg Message) {
	s.mu.Lock()
	session, exists := s.sessions[msg.SessionID]
	if exists {
		session.State = "terminated"
		session.UpdatedAt = time.Now()
		delete(s.sessions, msg.SessionID)
	}
	s.mu.Unlock()

	if !exists {
		return
	}

	s.logger.WithField("session_id", msg.SessionID).Info("Call terminated")

	// Notify both parties
	s.mu.RLock()
	for _, c := range s.clients {
		if (c.UserID == session.Caller || c.UserID == session.Callee) && c.Registered {
			hangupMsg := Message{
				Type:      MessageTypeHangup,
				From:      client.UserID,
				To:        c.UserID,
				SessionID: msg.SessionID,
				Timestamp: time.Now(),
			}
			c.Send <- hangupMsg
		}
	}
	s.mu.RUnlock()

	// Notify SIP handler
	if s.sipHandler != nil {
		if err := s.sipHandler.Hangup(msg.SessionID); err != nil {
			s.logger.WithError(err).Error("Failed to send hangup to SIP")
		}
	}
}

// GetSession returns a session by ID
func (s *Server) GetSession(sessionID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, exists := s.sessions[sessionID]
	return session, exists
}

// GetClient returns a client by user ID
func (s *Server) GetClient(userID string) *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, client := range s.clients {
		if client.UserID == userID && client.Registered {
			return client
		}
	}
	return nil
}

// generateID generates a unique ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
