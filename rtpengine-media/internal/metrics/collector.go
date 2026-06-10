package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// SessionMetrics tracks per-session media statistics
type SessionMetrics struct {
	CallID          string    `json:"call_id"`
	PacketsReceived uint64    `json:"packets_received"`
	PacketsSent     uint64    `json:"packets_sent"`
	BytesReceived   uint64    `json:"bytes_received"`
	BytesSent       uint64    `json:"bytes_sent"`
	PacketsLost     uint64    `json:"packets_lost"`
	Jitter          float64   `json:"jitter_ms"`
	RTT             float64   `json:"rtt_ms"`
	MOS             float64   `json:"mos_score"`
	Codec           string    `json:"codec"`
	LastUpdated     time.Time `json:"last_updated"`
	mu              sync.RWMutex
}

// Update updates metrics with new values from RTPengine
func (m *SessionMetrics) Update(packetsReceived, packetsSent, bytesReceived, bytesSent, packetsLost uint64, jitter, rtt float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PacketsReceived = packetsReceived
	m.PacketsSent = packetsSent
	m.BytesReceived = bytesReceived
	m.BytesSent = bytesSent
	m.PacketsLost = packetsLost
	m.Jitter = jitter
	m.RTT = rtt
	m.LastUpdated = time.Now()

	// Calculate MOS score (simplified ITU-T P.862 approximation)
	m.MOS = calculateMOS(jitter, rtt, packetsLost, packetsReceived)
}

// GetMOS returns the current MOS score
func (m *SessionMetrics) GetMOS() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.MOS
}

// GetPacketLossRate returns packet loss percentage
func (m *SessionMetrics) GetPacketLossRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.PacketsReceived + m.PacketsLost
	if total == 0 {
		return 0
	}
	return float64(m.PacketsLost) / float64(total) * 100
}

// Collector manages metrics for all active sessions
type Collector struct {
	sessions map[string]*SessionMetrics
	mu       sync.RWMutex
	total    GlobalMetrics
}

// GlobalMetrics tracks overall system metrics
type GlobalMetrics struct {
	ActiveSessions    int64     `json:"active_sessions"`
	TotalSessions     uint64    `json:"total_sessions"`
	TotalCalls        uint64    `json:"total_calls"`
	TotalRecordings   uint64    `json:"total_recordings"`
	FailedCalls       uint64    `json:"failed_calls"`
	TranscodedCalls   uint64    `json:"transcoded_calls"`
	AvgMOS            float64   `json:"avg_mos"`
	AvgJitter         float64   `json:"avg_jitter_ms"`
	AvgPacketLoss     float64   `json:"avg_packet_loss_percent"`
	Uptime            time.Time `json:"started_at"`
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		sessions: make(map[string]*SessionMetrics),
		total: GlobalMetrics{
			Uptime: time.Now(),
		},
	}
}

// AddSession creates metrics tracking for a new session
func (c *Collector) AddSession(callID, codec string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sessions[callID] = &SessionMetrics{
		CallID:      callID,
		Codec:       codec,
		LastUpdated: time.Now(),
	}

	atomic.AddInt64(&c.total.ActiveSessions, 1)
	atomic.AddUint64(&c.total.TotalSessions, 1)
}

// RemoveSession removes metrics tracking for a session
func (c *Collector) RemoveSession(callID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.sessions, callID)
	atomic.AddInt64(&c.total.ActiveSessions, -1)
}

// UpdateSession updates metrics for a specific session
func (c *Collector) UpdateSession(callID string, packetsReceived, packetsSent, bytesReceived, bytesSent, packetsLost uint64, jitter, rtt float64) {
	c.mu.RLock()
	metrics, exists := c.sessions[callID]
	c.mu.RUnlock()

	if !exists {
		return
	}

	metrics.Update(packetsReceived, packetsSent, bytesReceived, bytesSent, packetsLost, jitter, rtt)
}

// GetSession returns metrics for a specific session
func (c *Collector) GetSession(callID string) (*SessionMetrics, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, exists := c.sessions[callID]
	return m, exists
}

// GetAllSessions returns metrics for all active sessions
func (c *Collector) GetAllSessions() []*SessionMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sessions := make([]*SessionMetrics, 0, len(c.sessions))
	for _, m := range c.sessions {
		sessions = append(sessions, m)
	}
	return sessions
}

// GetGlobal returns global system metrics
func (c *Collector) GetGlobal() GlobalMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Calculate averages
	var totalMOS, totalJitter, totalPacketLoss float64
	var count int

	for _, m := range c.sessions {
		if m.LastUpdated.After(time.Now().Add(-5 * time.Minute)) {
			totalMOS += m.MOS
			totalJitter += m.Jitter
			totalPacketLoss += m.GetPacketLossRate()
			count++
		}
	}

	global := c.total
	if count > 0 {
		global.AvgMOS = totalMOS / float64(count)
		global.AvgJitter = totalJitter / float64(count)
		global.AvgPacketLoss = totalPacketLoss / float64(count)
	}

	return global
}

// IncrementCalls increments total call counter
func (c *Collector) IncrementCalls() {
	atomic.AddUint64(&c.total.TotalCalls, 1)
}

// IncrementFailedCalls increments failed call counter
func (c *Collector) IncrementFailedCalls() {
	atomic.AddUint64(&c.total.FailedCalls, 1)
}

// IncrementRecordings increments recording counter
func (c *Collector) IncrementRecordings() {
	atomic.AddUint64(&c.total.TotalRecordings, 1)
}

// IncrementTranscodedCalls increments transcoded call counter
func (c *Collector) IncrementTranscodedCalls() {
	atomic.AddUint64(&c.total.TranscodedCalls, 1)
}

// calculateMOS calculates a simplified MOS score
// Based on ITU-T G.107 E-model approximation
func calculateMOS(jitter, rtt float64, packetsLost, packetsReceived uint64) float64 {
	var packetLossRate float64
	if packetsReceived+packetsLost > 0 {
		packetLossRate = float64(packetsLost) / float64(packetsReceived+packetsLost) * 100
	}

	// Equipment impairment factor (simplified for G.711)
	Ie := 0.0

	// Packet loss robustness factor
	Bpl := 25.1

	// Expected equipment impairment factor
	IeEff := Ie + (95 - Ie) * (packetLossRate / (packetLossRate + Bpl))

	// Delay impairment
	delay := rtt + jitter
	var Id float64
	if delay < 100 {
		Id = 0.0
	} else if delay < 400 {
		Id = 0.024 * delay
	} else {
		Id = 0.024*400 + 0.11*(delay-400)
	}

	// R-factor
	R := 93.2 - IeEff - Id

	// Clamp R-factor
	if R < 0 {
		R = 0
	}
	if R > 100 {
		R = 100
	}

	// Convert to MOS
	var mos float64
	if R < 0 {
		mos = 1.0
	} else if R < 100 {
		mos = 1 + 0.035*R + R*(R-60)*(100-R)*7*0.000001
	} else {
		mos = 4.5
	}

	if mos < 1.0 {
		mos = 1.0
	}
	if mos > 4.5 {
		mos = 4.5
	}

	return mos
}
