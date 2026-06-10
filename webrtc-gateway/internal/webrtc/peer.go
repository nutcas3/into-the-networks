package webrtc

import (
	"fmt"
	"sync"

	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
)

// PeerConnectionManager manages WebRTC peer connections
type PeerConnectionManager struct {
	connections map[string]*PeerConnection
	mu          sync.RWMutex
	logger      *logrus.Logger
	config      *webrtc.Configuration
}

// PeerConnection wraps a WebRTC peer connection
type PeerConnection struct {
	ID          string
	UserID      string
	SessionID   string
	PC          *webrtc.PeerConnection
	DataChannel *webrtc.DataChannel
	State       string
	mu          sync.RWMutex
}

// Config holds peer connection configuration
type Config struct {
	STUNServers []string
	TURNServers []TURNConfig
	Logger      *logrus.Logger
}

// TURNConfig holds TURN server configuration
type TURNConfig struct {
	URL      string
	Username string
	Password string
}

// NewPeerConnectionManager creates a new peer connection manager
func NewPeerConnectionManager(cfg Config) (*PeerConnectionManager, error) {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}

	// Create WebRTC configuration
	rtcConfig := &webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{},
	}

	// Add STUN servers
	for _, stun := range cfg.STUNServers {
		rtcConfig.ICEServers = append(rtcConfig.ICEServers, webrtc.ICEServer{
			URLs: []string{stun},
		})
	}

	// Add TURN servers
	for _, turn := range cfg.TURNServers {
		rtcConfig.ICEServers = append(rtcConfig.ICEServers, webrtc.ICEServer{
			URLs:       []string{turn.URL},
			Username:   turn.Username,
			Credential: turn.Password,
		})
	}

	return &PeerConnectionManager{
		connections: make(map[string]*PeerConnection),
		logger:      cfg.Logger,
		config:      rtcConfig,
	}, nil
}

// CreatePeerConnection creates a new WebRTC peer connection
func (m *PeerConnectionManager) CreatePeerConnection(userID, sessionID string) (*PeerConnection, error) {
	pc, err := webrtc.NewPeerConnection(*m.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	peerID := fmt.Sprintf("%s-%s", userID, sessionID)
	peer := &PeerConnection{
		ID:        peerID,
		UserID:    userID,
		SessionID: sessionID,
		PC:        pc,
		State:     "new",
	}

	// Set up ICE candidate handler
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			m.logger.WithField("peer_id", peerID).Info("ICE gathering complete")
			return
		}
		m.logger.WithFields(logrus.Fields{
			"peer_id":   peerID,
			"candidate": c.String(),
		}).Debug("ICE candidate received")
	})

	// Set up connection state handler
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		m.logger.WithFields(logrus.Fields{
			"peer_id": peerID,
			"state":   state.String(),
		}).Info("Connection state changed")

		peer.mu.Lock()
		peer.State = state.String()
		peer.mu.Unlock()
	})

	// Set up data channel handler
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		m.logger.WithField("peer_id", peerID).Info("Data channel opened")
		peer.mu.Lock()
		peer.DataChannel = dc
		peer.mu.Unlock()
	})

	m.mu.Lock()
	m.connections[peerID] = peer
	m.mu.Unlock()

	return peer, nil
}

// GetPeerConnection retrieves a peer connection by ID
func (m *PeerConnectionManager) GetPeerConnection(peerID string) (*PeerConnection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	peer, exists := m.connections[peerID]
	return peer, exists
}

// ClosePeerConnection closes a peer connection
func (m *PeerConnectionManager) ClosePeerConnection(peerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	peer, exists := m.connections[peerID]
	if !exists {
		return fmt.Errorf("peer connection not found: %s", peerID)
	}

	if err := peer.PC.Close(); err != nil {
		return fmt.Errorf("failed to close peer connection: %w", err)
	}

	delete(m.connections, peerID)
	m.logger.WithField("peer_id", peerID).Info("Peer connection closed")

	return nil
}

// CreateOffer creates a WebRTC offer
func (p *PeerConnection) CreateOffer() (string, error) {
	offer, err := p.PC.CreateOffer(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create offer: %w", err)
	}

	if err := p.PC.SetLocalDescription(offer); err != nil {
		return "", fmt.Errorf("failed to set local description: %w", err)
	}

	return offer.SDP, nil
}

// CreateAnswer creates a WebRTC answer
func (p *PeerConnection) CreateAnswer(offerSDP string) (string, error) {
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSDP,
	}

	if err := p.PC.SetRemoteDescription(offer); err != nil {
		return "", fmt.Errorf("failed to set remote description: %w", err)
	}

	answer, err := p.PC.CreateAnswer(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create answer: %w", err)
	}

	if err := p.PC.SetLocalDescription(answer); err != nil {
		return "", fmt.Errorf("failed to set local description: %w", err)
	}

	return answer.SDP, nil
}

// SetAnswer sets the remote answer
func (p *PeerConnection) SetAnswer(answerSDP string) error {
	answer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answerSDP,
	}

	if err := p.PC.SetRemoteDescription(answer); err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	return nil
}

// AddICECandidate adds an ICE candidate to the peer connection
func (p *PeerConnection) AddICECandidate(candidate string, sdpMid string, sdpMLineIndex uint16) error {
	iceCandidate := webrtc.ICECandidateInit{
		Candidate:     candidate,
		SDPMid:        &sdpMid,
		SDPMLineIndex: &sdpMLineIndex,
	}

	if err := p.PC.AddICECandidate(iceCandidate); err != nil {
		return fmt.Errorf("failed to add ICE candidate: %w", err)
	}

	return nil
}

// GetState returns the current connection state
func (p *PeerConnection) GetState() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State
}

// Close closes the peer connection
func (p *PeerConnection) Close() error {
	return p.PC.Close()
}
