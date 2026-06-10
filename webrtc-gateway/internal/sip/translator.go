package sip

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Translator handles SIP to WebRTC translation
type Translator struct {
	clients   map[string]*SIPClient
	sessions  map[string]*SIPSession
	mu        sync.RWMutex
	logger    *logrus.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	rtpEngine string
	sipServer string
	sipPort   int
}

// SIPClient represents a SIP client connection
type SIPClient struct {
	UserID     string
	Conn       net.Conn
	Auth       *AuthInfo
	Registered bool
	mu         sync.RWMutex
}

// AuthInfo contains SIP authentication details
type AuthInfo struct {
	Username string
	Password string
	Realm    string
}

// SIPSession represents a SIP call session
type SIPSession struct {
	ID         string
	Caller     string
	Callee     string
	WebRTCSDP  string
	SIPSDP     string
	State      string
	InviteSent bool
	Answered   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Config holds translator configuration
type Config struct {
	SIPServer string
	SIPPort   int
	RTPEngine string
	Logger    *logrus.Logger
}

// NewTranslator creates a new SIP translator
func NewTranslator(cfg Config) (*Translator, error) {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}

	ctx, cancel := context.WithCancel(context.Background())

	t := &Translator{
		clients:   make(map[string]*SIPClient),
		sessions:  make(map[string]*SIPSession),
		logger:    cfg.Logger,
		ctx:       ctx,
		cancel:    cancel,
		rtpEngine: cfg.RTPEngine,
		sipServer: cfg.SIPServer,
		sipPort:   cfg.SIPPort,
	}

	return t, nil
}

// Close shuts down the translator
func (t *Translator) Close() {
	t.cancel()
	t.mu.Lock()
	for _, client := range t.clients {
		if client.Conn != nil {
			client.Conn.Close()
		}
	}
	t.mu.Unlock()
}

// Invite sends a SIP INVITE to initiate a call
func (t *Translator) Invite(caller, callee, sdp string) (string, error) {
	sessionID := generateSessionID()

	session := &SIPSession{
		ID:         sessionID,
		Caller:     caller,
		Callee:     callee,
		WebRTCSDP:  sdp,
		State:      "inviting",
		InviteSent: true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	t.mu.Lock()
	t.sessions[sessionID] = session
	t.mu.Unlock()

	t.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"caller":     caller,
		"callee":     callee,
	}).Info("SIP INVITE sent")

	// Convert WebRTC SDP to SIP SDP
	sipSDP := ConvertWebRTCSDPToSIP(sdp)
	session.SIPSDP = sipSDP

	// Send SIP INVITE via UDP
	go t.sendSIPInvite(session, sipSDP)

	return sessionID, nil
}

// sendSIPInvite sends a SIP INVITE message via UDP
func (t *Translator) sendSIPInvite(session *SIPSession, sipSDP string) {
	callID := GenerateCallID()
	fromTag := GenerateTag()

	// Build SIP INVITE message
	invite := fmt.Sprintf(
		"INVITE sip:%s@%s:%d SIP/2.0\r\n"+
			"Via: SIP/2.0/UDP %s:%d;branch=z9hG4bK%s;rport\r\n"+
			"From: <sip:%s@%s>;tag=%s\r\n"+
			"To: <sip:%s@%s>\r\n"+
			"Call-ID: %s\r\n"+
			"CSeq: 1 INVITE\r\n"+
			"Contact: <sip:%s@%s:%d>\r\n"+
			"Content-Type: application/sdp\r\n"+
			"Allow: INVITE, ACK, CANCEL, BYE, OPTIONS, MESSAGE\r\n"+
			"Supported: replaces\r\n"+
			"Content-Length: %d\r\n"+
			"\r\n%s",
		session.Callee, t.sipServer, t.sipPort,
		t.sipServer, t.sipPort, GenerateTag(),
		session.Caller, t.sipServer, fromTag,
		session.Callee, t.sipServer,
		callID,
		session.Caller, t.sipServer, t.sipPort,
		len(sipSDP), sipSDP,
	)

	t.logger.WithFields(logrus.Fields{
		"session_id": session.ID,
		"call_id":    callID,
	}).Info("Sending SIP INVITE")

	// Send via UDP
	addr := fmt.Sprintf("%s:%d", t.sipServer, t.sipPort)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.logger.WithError(err).Error("Failed to resolve SIP address")
		return
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.logger.WithError(err).Error("Failed to connect to SIP server")
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(invite)); err != nil {
		t.logger.WithError(err).Error("Failed to send SIP INVITE")
		return
	}

	// Wait for responses
	buf := make([]byte, 65536)
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	for {
		n, err := conn.Read(buf)
		if err != nil {
			t.logger.WithError(err).Error("Failed to read SIP response")
			return
		}

		response := string(buf[:n])
		if strings.Contains(response, "SIP/2.0 200") {
			t.mu.Lock()
			if s, exists := t.sessions[session.ID]; exists {
				s.State = "answered"
				s.Answered = true
				s.UpdatedAt = time.Now()
				// Extract SDP from response
				if parts := strings.Split(response, "\r\n\r\n"); len(parts) > 1 {
					s.SIPSDP = parts[1]
				}
			}
			t.mu.Unlock()

			t.logger.WithField("session_id", session.ID).Info("SIP call answered (200 OK)")

			// Send ACK
			ack := fmt.Sprintf(
				"ACK sip:%s@%s:%d SIP/2.0\r\n"+
					"Via: SIP/2.0/UDP %s:%d;branch=z9hG4bK%s;rport\r\n"+
					"From: <sip:%s@%s>;tag=%s\r\n"+
					"To: <sip:%s@%s>\r\n"+
					"Call-ID: %s\r\n"+
					"CSeq: 1 ACK\r\n"+
					"Content-Length: 0\r\n"+
					"\r\n",
				session.Callee, t.sipServer, t.sipPort,
				t.sipServer, t.sipPort, GenerateTag(),
				session.Caller, t.sipServer, fromTag,
				session.Callee, t.sipServer,
				callID,
			)
			conn.Write([]byte(ack))
			return
		} else if strings.Contains(response, "SIP/2.0 486") {
			t.logger.WithField("session_id", session.ID).Warn("SIP call busy (486)")
			return
		} else if strings.Contains(response, "SIP/2.0 480") {
			t.logger.WithField("session_id", session.ID).Warn("SIP call unavailable (480)")
			return
		}
	}
}

// Answer sends a SIP 200 OK with SDP
func (t *Translator) Answer(sessionID, sdp string) error {
	t.mu.Lock()
	session, exists := t.sessions[sessionID]
	if !exists {
		t.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}
	session.WebRTCSDP = sdp
	session.State = "answered"
	session.Answered = true
	session.UpdatedAt = time.Now()
	t.mu.Unlock()

	t.logger.WithField("session_id", sessionID).Info("SIP 200 OK sent")

	// Send SIP 200 OK response with SDP
	go t.sendSIPAnswer(session, sdp)

	return nil
}

// Hangup sends a SIP BYE to terminate the call
func (t *Translator) Hangup(sessionID string) error {
	t.mu.Lock()
	session, exists := t.sessions[sessionID]
	if !exists {
		t.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}
	session.State = "terminated"
	session.UpdatedAt = time.Now()
	delete(t.sessions, sessionID)
	t.mu.Unlock()

	t.logger.WithField("session_id", sessionID).Info("SIP BYE sent")

	// Send SIP BYE to terminate the call
	go t.sendSIPBye(session)

	return nil
}

// Register registers a SIP client
func (t *Translator) Register(userID, username, password string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	client, exists := t.clients[userID]
	if !exists {
		client = &SIPClient{
			UserID: userID,
			Auth: &AuthInfo{
				Username: username,
				Password: password,
				Realm:    "sip.example.com",
			},
		}
		t.clients[userID] = client
	}

	client.Registered = true

	t.logger.WithField("user_id", userID).Info("SIP client registered")

	// Send SIP REGISTER to the server
	go t.sendSIPRegister(userID, username, password)

	return nil
}

// GetSession returns a session by ID
func (t *Translator) GetSession(sessionID string) (*SIPSession, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	session, exists := t.sessions[sessionID]
	return session, exists
}

// sendSIPAnswer sends a SIP 200 OK response with SDP
func (t *Translator) sendSIPAnswer(session *SIPSession, sdp string) {
	// Build SIP 200 OK message
	ok := fmt.Sprintf(
		"SIP/2.0 200 OK\r\n"+
			"Via: SIP/2.0/UDP %s:%d;branch=z9hG4bK%s;rport\r\n"+
			"From: <sip:%s@%s>;tag=%s\r\n"+
			"To: <sip:%s@%s>;tag=%s\r\n"+
			"Call-ID: %s\r\n"+
			"CSeq: 1 INVITE\r\n"+
			"Contact: <sip:%s@%s:%d>\r\n"+
			"Content-Type: application/sdp\r\n"+
			"Content-Length: %d\r\n"+
			"\r\n%s",
		t.sipServer, t.sipPort, GenerateTag(),
		session.Callee, t.sipServer, GenerateTag(),
		session.Caller, t.sipServer, GenerateTag(),
		GenerateCallID(),
		session.Callee, t.sipServer, t.sipPort,
		len(sdp), sdp,
	)

	addr := fmt.Sprintf("%s:%d", t.sipServer, t.sipPort)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.logger.WithError(err).Error("Failed to resolve SIP address for 200 OK")
		return
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.logger.WithError(err).Error("Failed to connect to SIP server for 200 OK")
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(ok)); err != nil {
		t.logger.WithError(err).Error("Failed to send SIP 200 OK")
		return
	}

	// Wait for ACK
	buf := make([]byte, 65536)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if n, err := conn.Read(buf); err == nil {
		ack := string(buf[:n])
		if strings.Contains(ack, "ACK") {
			t.logger.WithField("session_id", session.ID).Info("SIP ACK received")
		}
	}
}

// sendSIPBye sends a SIP BYE message to terminate a call
func (t *Translator) sendSIPBye(session *SIPSession) {
	bye := fmt.Sprintf(
		"BYE sip:%s@%s:%d SIP/2.0\r\n"+
			"Via: SIP/2.0/UDP %s:%d;branch=z9hG4bK%s;rport\r\n"+
			"From: <sip:%s@%s>;tag=%s\r\n"+
			"To: <sip:%s@%s>;tag=%s\r\n"+
			"Call-ID: %s\r\n"+
			"CSeq: 2 BYE\r\n"+
			"Content-Length: 0\r\n"+
			"\r\n",
		session.Callee, t.sipServer, t.sipPort,
		t.sipServer, t.sipPort, GenerateTag(),
		session.Caller, t.sipServer, GenerateTag(),
		session.Callee, t.sipServer, GenerateTag(),
		GenerateCallID(),
	)

	addr := fmt.Sprintf("%s:%d", t.sipServer, t.sipPort)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.logger.WithError(err).Error("Failed to resolve SIP address for BYE")
		return
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.logger.WithError(err).Error("Failed to connect to SIP server for BYE")
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(bye)); err != nil {
		t.logger.WithError(err).Error("Failed to send SIP BYE")
		return
	}

	// Wait for 200 OK
	buf := make([]byte, 65536)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if n, err := conn.Read(buf); err == nil {
		resp := string(buf[:n])
		if strings.Contains(resp, "SIP/2.0 200") {
			t.logger.WithField("session_id", session.ID).Info("SIP BYE acknowledged")
		}
	}
}

// sendSIPRegister sends a SIP REGISTER message to the server
func (t *Translator) sendSIPRegister(userID, username, password string) {
	register := fmt.Sprintf(
		"REGISTER sip:%s SIP/2.0\r\n"+
			"Via: SIP/2.0/UDP %s:%d;branch=z9hG4bK%s;rport\r\n"+
			"From: <sip:%s@%s>;tag=%s\r\n"+
			"To: <sip:%s@%s>\r\n"+
			"Call-ID: %s\r\n"+
			"CSeq: 1 REGISTER\r\n"+
			"Contact: <sip:%s@%s:%d>\r\n"+
			"Expires: 3600\r\n"+
			"Content-Length: 0\r\n"+
			"\r\n",
		t.sipServer,
		t.sipServer, t.sipPort, GenerateTag(),
		username, t.sipServer, GenerateTag(),
		username, t.sipServer,
		GenerateCallID(),
		username, t.sipServer, t.sipPort,
	)

	addr := fmt.Sprintf("%s:%d", t.sipServer, t.sipPort)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.logger.WithError(err).Error("Failed to resolve SIP address for REGISTER")
		return
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		t.logger.WithError(err).Error("Failed to connect to SIP server for REGISTER")
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(register)); err != nil {
		t.logger.WithError(err).Error("Failed to send SIP REGISTER")
		return
	}

	// Wait for 200 OK or 401 Unauthorized
	buf := make([]byte, 65536)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if n, err := conn.Read(buf); err == nil {
		resp := string(buf[:n])
		if strings.Contains(resp, "SIP/2.0 200") {
			t.logger.WithField("user_id", userID).Info("SIP REGISTER successful")
		} else if strings.Contains(resp, "SIP/2.0 401") {
			t.logger.WithField("user_id", userID).Info("SIP REGISTER requires authentication")
		}
	}
}

// ConvertWebRTCSDPToSIP converts WebRTC SDP to SIP SDP format
func ConvertWebRTCSDPToSIP(webrtcSDP string) string {
	// Strip WebRTC-specific attributes for SIP compatibility
	lines := strings.Split(webrtcSDP, "\r\n")
	var result []string
	for _, line := range lines {
		// Skip ICE candidates, DTLS fingerprint, and SRTP crypto
		if strings.HasPrefix(line, "a=candidate:") {
			continue
		}
		if strings.HasPrefix(line, "a=fingerprint:") {
			continue
		}
		if strings.HasPrefix(line, "a=crypto:") {
			continue
		}
		if strings.HasPrefix(line, "a=setup:") {
			continue
		}
		if strings.HasPrefix(line, "a=ice-") {
			continue
		}
		// Keep only RTP/AVP profile for SIP
		if strings.HasPrefix(line, "m=") {
			line = strings.Replace(line, "UDP/TLS/RTP/SAVPF", "RTP/AVP", 1)
			line = strings.Replace(line, "UDP/TLS/RTP/SAVP", "RTP/AVP", 1)
		}
		result = append(result, line)
	}
	return strings.Join(result, "\r\n")
}

// ConvertSIPSDPToWebRTC converts SIP SDP to WebRTC SDP format
func ConvertSIPSDPToWebRTC(sipSDP string) string {
	// Add WebRTC required attributes to SIP SDP
	lines := strings.Split(sipSDP, "\r\n")
	var result []string
	var mediaSection bool
	for _, line := range lines {
		if strings.HasPrefix(line, "m=") {
			mediaSection = true
			// Upgrade to DTLS/SRTP for WebRTC
			line = strings.Replace(line, "RTP/AVP", "UDP/TLS/RTP/SAVPF", 1)
		}
		result = append(result, line)
		if mediaSection && strings.HasPrefix(line, "a=rtcp:") {
			// Add DTLS setup after rtcp line
			result = append(result, "a=setup:actpass")
			result = append(result, "a=fingerprint:sha-256 00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00:00")
			mediaSection = false
		}
	}
	return strings.Join(result, "\r\n")
}

// ParseSIPAddress parses a SIP address string
func ParseSIPAddress(addr string) (username, host string, port int, err error) {
	// Remove "sip:" prefix
	addr = strings.TrimPrefix(addr, "sip:")

	// Split at @
	parts := strings.Split(addr, "@")
	if len(parts) != 2 {
		return "", "", 0, fmt.Errorf("invalid SIP address format")
	}

	username = parts[0]

	// Split host and port
	hostPort := strings.Split(parts[1], ":")
	host = hostPort[0]

	if len(hostPort) > 1 {
		port, err = strconv.Atoi(hostPort[1])
		if err != nil {
			return "", "", 0, fmt.Errorf("invalid port: %w", err)
		}
	} else {
		port = 5060 // Default SIP port
	}

	return username, host, port, nil
}

// GenerateCallID generates a unique SIP Call-ID
func GenerateCallID() string {
	return fmt.Sprintf("%x@%s", time.Now().UnixNano(), "webrtc-gateway")
}

// GenerateTag generates a unique SIP tag
func GenerateTag() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("sip-%d", time.Now().UnixNano())
}
