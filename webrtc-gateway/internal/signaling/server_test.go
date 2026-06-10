package signaling

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSIPHandler struct {
	inviteCalled  bool
	answerCalled  bool
	hangupCalled  bool
	lastSessionID string
	lastSDP       string
}

func (m *mockSIPHandler) Invite(caller, callee, sdp string) (string, error) {
	m.inviteCalled = true
	return "test-session", nil
}

func (m *mockSIPHandler) Answer(sessionID string, sdp string) error {
	m.answerCalled = true
	m.lastSessionID = sessionID
	m.lastSDP = sdp
	return nil
}

func (m *mockSIPHandler) Hangup(sessionID string) error {
	m.hangupCalled = true
	m.lastSessionID = sessionID
	return nil
}

func TestNewServer(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockSIP := &mockSIPHandler{}
	server := NewServer(logger, mockSIP)
	require.NotNil(t, server)
	assert.NotNil(t, server.clients)
	assert.NotNil(t, server.sessions)
	assert.NotNil(t, server.sipHandler)
}

func TestServerHandleRegister(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	server := NewServer(logger, nil)
	client := &Client{
		ID:   "client-1",
		Send: make(chan Message, 10),
	}

	msg := Message{
		Type: MessageTypeRegister,
		From: "alice",
	}

	server.handleRegister(client, msg)

	assert.True(t, client.Registered)
	assert.Equal(t, "alice", client.UserID)

	// Check response was sent
	select {
	case resp := <-client.Send:
		assert.Equal(t, MessageTypeRegister, resp.Type)
		assert.Equal(t, "server", resp.From)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected register response")
	}
}

func TestServerHandleCall(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	server := NewServer(logger, nil)

	// Register caller
	caller := &Client{
		ID:         "caller-1",
		UserID:     "alice",
		Registered: true,
		Send:       make(chan Message, 10),
	}
	server.clients["caller-1"] = caller

	// Register callee
	callee := &Client{
		ID:         "callee-1",
		UserID:     "bob",
		Registered: true,
		Send:       make(chan Message, 10),
	}
	server.clients["callee-1"] = callee

	msg := Message{
		Type: MessageTypeCall,
		From: "alice",
		To:   "bob",
	}

	server.handleCall(caller, msg)

	// Check callee received call notification
	select {
	case resp := <-callee.Send:
		assert.Equal(t, MessageTypeCall, resp.Type)
		assert.Equal(t, "alice", resp.From)
		assert.Equal(t, "bob", resp.To)
		assert.NotEmpty(t, resp.SessionID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected call notification")
	}

	// Verify session was created
	sessionID := ""
	select {
	case resp := <-callee.Send:
		sessionID = resp.SessionID
	case <-time.After(100 * time.Millisecond):
	}

	if sessionID != "" {
		session, exists := server.GetSession(sessionID)
		assert.True(t, exists)
		assert.Equal(t, "alice", session.Caller)
		assert.Equal(t, "bob", session.Callee)
		assert.Equal(t, "initiating", session.State)
	}
}

func TestServerHandleOffer(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	server := NewServer(logger, nil)

	// Create session
	sessionID := generateID()
	server.sessions[sessionID] = &Session{
		ID:     sessionID,
		Caller: "alice",
		Callee: "bob",
		State:  "initiating",
	}

	// Register callee
	callee := &Client{
		ID:         "callee-1",
		UserID:     "bob",
		Registered: true,
		Send:       make(chan Message, 10),
	}
	server.clients["callee-1"] = callee

	msg := Message{
		Type:      MessageTypeOffer,
		From:      "alice",
		To:        "bob",
		SessionID: sessionID,
		SDP:       "v=0\r\n",
	}

	server.handleOffer(nil, msg)

	// Verify session state updated
	session, exists := server.GetSession(sessionID)
	assert.True(t, exists)
	assert.Equal(t, "offered", session.State)
	assert.Equal(t, "v=0\r\n", session.OfferSDP)

	// Check callee received offer
	select {
	case resp := <-callee.Send:
		assert.Equal(t, MessageTypeOffer, resp.Type)
		assert.Equal(t, "v=0\r\n", resp.SDP)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected offer notification")
	}
}

func TestServerHandleAnswer(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockSIP := &mockSIPHandler{}
	server := NewServer(logger, mockSIP)

	// Create session
	sessionID := generateID()
	server.sessions[sessionID] = &Session{
		ID:     sessionID,
		Caller: "alice",
		Callee: "bob",
		State:  "offered",
	}

	// Register caller
	caller := &Client{
		ID:         "caller-1",
		UserID:     "alice",
		Registered: true,
		Send:       make(chan Message, 10),
	}
	server.clients["caller-1"] = caller

	msg := Message{
		Type:      MessageTypeAnswer,
		From:      "bob",
		To:        "alice",
		SessionID: sessionID,
		SDP:       "v=0\r\n",
	}

	server.handleAnswer(nil, msg)

	// Verify session state updated
	session, exists := server.GetSession(sessionID)
	assert.True(t, exists)
	assert.Equal(t, "answered", session.State)
	assert.Equal(t, "v=0\r\n", session.AnswerSDP)

	// Check caller received answer
	select {
	case resp := <-caller.Send:
		assert.Equal(t, MessageTypeAnswer, resp.Type)
		assert.Equal(t, "v=0\r\n", resp.SDP)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected answer notification")
	}

	// Verify SIP handler was called
	assert.True(t, mockSIP.answerCalled)
	assert.Equal(t, sessionID, mockSIP.lastSessionID)
}

func TestServerHandleHangup(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	mockSIP := &mockSIPHandler{}
	server := NewServer(logger, mockSIP)

	// Create session
	sessionID := generateID()
	server.sessions[sessionID] = &Session{
		ID:     sessionID,
		Caller: "alice",
		Callee: "bob",
		State:  "answered",
	}

	// Register caller and callee
	caller := &Client{
		ID:         "caller-1",
		UserID:     "alice",
		Registered: true,
		Send:       make(chan Message, 10),
	}
	server.clients["caller-1"] = caller

	callee := &Client{
		ID:         "callee-1",
		UserID:     "bob",
		Registered: true,
		Send:       make(chan Message, 10),
	}
	server.clients["callee-1"] = callee

	msg := Message{
		Type:      MessageTypeHangup,
		From:      "alice",
		SessionID: sessionID,
	}

	server.handleHangup(caller, msg)

	// Verify session was deleted
	_, exists := server.GetSession(sessionID)
	assert.False(t, exists)

	// Check both parties received hangup
	select {
	case resp := <-caller.Send:
		assert.Equal(t, MessageTypeHangup, resp.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected hangup notification to caller")
	}

	select {
	case resp := <-callee.Send:
		assert.Equal(t, MessageTypeHangup, resp.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected hangup notification to callee")
	}

	// Verify SIP handler was called
	assert.True(t, mockSIP.hangupCalled)
	assert.Equal(t, sessionID, mockSIP.lastSessionID)
}

func TestServerGetSession(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	server := NewServer(logger, nil)

	// Non-existent session
	_, exists := server.GetSession("non-existent")
	assert.False(t, exists)

	// Existing session
	sessionID := generateID()
	server.sessions[sessionID] = &Session{
		ID:     sessionID,
		Caller: "alice",
		Callee: "bob",
	}

	session, exists := server.GetSession(sessionID)
	assert.True(t, exists)
	assert.Equal(t, "alice", session.Caller)
	assert.Equal(t, "bob", session.Callee)
}

func TestServerGetClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	server := NewServer(logger, nil)

	// Non-existent client
	client := server.GetClient("nobody")
	assert.Nil(t, client)

	// Existing client
	registeredClient := &Client{
		ID:         "client-1",
		UserID:     "alice",
		Registered: true,
	}
	server.clients["client-1"] = registeredClient

	found := server.GetClient("alice")
	assert.NotNil(t, found)
	assert.Equal(t, "alice", found.UserID)
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()

	assert.NotEmpty(t, id1)
	assert.Greater(t, len(id1), 0)
}
