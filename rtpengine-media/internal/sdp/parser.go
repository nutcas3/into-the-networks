package sdp

import (
	"fmt"
	"strconv"
	"strings"
)

// SessionDescription represents a parsed SDP session
type SessionDescription struct {
	Version       int            `json:"version"`
	Origin        Origin         `json:"origin"`
	SessionName   string         `json:"session_name"`
	Connection    *Connection    `json:"connection,omitempty"`
	Bandwidth     []Bandwidth    `json:"bandwidth,omitempty"`
	Timing        []Timing       `json:"timing,omitempty"`
	MediaSections []MediaSection `json:"media_sections"`
	Attributes    []Attribute    `json:"attributes,omitempty"`
	Raw           string         `json:"raw,omitempty"`
}

// Origin represents the origin field
type Origin struct {
	Username       string `json:"username"`
	SessionID      string `json:"session_id"`
	SessionVersion string `json:"session_version"`
	NetworkType    string `json:"network_type"`
	AddressType    string `json:"address_type"`
	Address        string `json:"address"`
}

// Connection represents the connection field
type Connection struct {
	NetworkType string `json:"network_type"`
	AddressType string `json:"address_type"`
	Address     string `json:"address"`
	TTL         int    `json:"ttl,omitempty"`
	Count       int    `json:"count,omitempty"`
}

// Bandwidth represents bandwidth information
type Bandwidth struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Timing represents timing information
type Timing struct {
	Start string `json:"start"`
	Stop  string `json:"stop"`
}

// MediaSection represents a media section
type MediaSection struct {
	Type       string      `json:"type"`
	Port       int         `json:"port"`
	Protocol   string      `json:"protocol"`
	Formats    []string    `json:"formats"`
	Title      string      `json:"title,omitempty"`
	Connection *Connection `json:"connection,omitempty"`
	Bandwidth  []Bandwidth `json:"bandwidth,omitempty"`
	Attributes []Attribute `json:"attributes"`
	Raw        string      `json:"raw,omitempty"`
}

// Attribute represents an SDP attribute
type Attribute struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// Codec represents a parsed codec
type Codec struct {
	PayloadType int    `json:"payload_type"`
	Name        string `json:"name"`
	ClockRate   int    `json:"clock_rate"`
	Channels    int    `json:"channels,omitempty"`
}

// Parse parses an SDP string into a structured description
func Parse(sdp string) (*SessionDescription, error) {
	session := &SessionDescription{
		Attributes:    make([]Attribute, 0),
		MediaSections: make([]MediaSection, 0),
		Bandwidth:     make([]Bandwidth, 0),
		Timing:        make([]Timing, 0),
		Raw:           sdp,
	}

	lines := strings.Split(sdp, "\r\n")
	if len(lines) == 1 {
		lines = strings.Split(sdp, "\n")
	}

	var currentMedia *MediaSection

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if len(line) < 2 || line[1] != '=' {
			continue
		}

		field := line[0]
		value := line[2:]

		switch field {
		case 'v':
			v, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid version: %s", value)
			}
			session.Version = v
		case 'o':
			origin, err := parseOrigin(value)
			if err != nil {
				return nil, fmt.Errorf("invalid origin: %w", err)
			}
			session.Origin = origin
		case 's':
			session.SessionName = value
		case 'c':
			conn, err := parseConnection(value)
			if err != nil {
				return nil, fmt.Errorf("invalid connection: %w", err)
			}
			if currentMedia != nil {
				currentMedia.Connection = conn
			} else {
				session.Connection = conn
			}
		case 'b':
			bw := parseBandwidth(value)
			if currentMedia != nil {
				currentMedia.Bandwidth = append(currentMedia.Bandwidth, bw)
			} else {
				session.Bandwidth = append(session.Bandwidth, bw)
			}
		case 't':
			timing := parseTiming(value)
			session.Timing = append(session.Timing, timing)
		case 'm':
			if currentMedia != nil {
				session.MediaSections = append(session.MediaSections, *currentMedia)
			}
			media, err := parseMedia(value)
			if err != nil {
				return nil, fmt.Errorf("invalid media: %w", err)
			}
			currentMedia = &media
		case 'a':
			attr := parseAttribute(value)
			if currentMedia != nil {
				currentMedia.Attributes = append(currentMedia.Attributes, attr)
			} else {
				session.Attributes = append(session.Attributes, attr)
			}
		}
	}

	if currentMedia != nil {
		session.MediaSections = append(session.MediaSections, *currentMedia)
	}

	return session, nil
}

// String converts the session description back to SDP format
func (s *SessionDescription) String() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("v=%d", s.Version))
	parts = append(parts, fmt.Sprintf("o=%s %s %s %s %s %s",
		s.Origin.Username,
		s.Origin.SessionID,
		s.Origin.SessionVersion,
		s.Origin.NetworkType,
		s.Origin.AddressType,
		s.Origin.Address))
	parts = append(parts, fmt.Sprintf("s=%s", s.SessionName))

	if s.Connection != nil {
		parts = append(parts, s.Connection.String())
	}

	for _, bw := range s.Bandwidth {
		parts = append(parts, fmt.Sprintf("b=%s:%s", bw.Type, bw.Value))
	}

	for _, t := range s.Timing {
		parts = append(parts, fmt.Sprintf("t=%s %s", t.Start, t.Stop))
	}

	for _, attr := range s.Attributes {
		if attr.Value != "" {
			parts = append(parts, fmt.Sprintf("a=%s:%s", attr.Name, attr.Value))
		} else {
			parts = append(parts, fmt.Sprintf("a=%s", attr.Name))
		}
	}

	for _, media := range s.MediaSections {
		parts = append(parts, media.String())
	}

	return strings.Join(parts, "\r\n") + "\r\n"
}

// GetCodecs extracts codecs from a media section
func (m *MediaSection) GetCodecs() []Codec {
	var codecs []Codec

	for _, attr := range m.Attributes {
		if attr.Name == "rtpmap" {
			codec := parseCodec(attr.Value)
			if codec != nil {
				codecs = append(codecs, *codec)
			}
		}
	}

	return codecs
}

// SetMediaAddress sets the media connection address
func (s *SessionDescription) SetMediaAddress(addr string) {
	for i := range s.MediaSections {
		s.MediaSections[i].Connection = &Connection{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     addr,
		}
	}
}

// ReplaceCodecs replaces the codec list in all media sections
func (s *SessionDescription) ReplaceCodecs(codecs []Codec) {
	for i := range s.MediaSections {
		m := &s.MediaSections[i]
		var formats []string
		var attrs []Attribute

		for _, codec := range codecs {
			pt := strconv.Itoa(codec.PayloadType)
			formats = append(formats, pt)
			attrs = append(attrs, Attribute{
				Name:  "rtpmap",
				Value: fmt.Sprintf("%s %s/%d", pt, codec.Name, codec.ClockRate),
			})
		}

		m.Formats = formats
		// Keep non-rtpmap attributes
		for _, attr := range m.Attributes {
			if attr.Name != "rtpmap" {
				attrs = append(attrs, attr)
			}
		}
		m.Attributes = attrs
	}
}

func parseOrigin(value string) (Origin, error) {
	parts := strings.Fields(value)
	if len(parts) < 6 {
		return Origin{}, fmt.Errorf("invalid origin format")
	}
	return Origin{
		Username:       parts[0],
		SessionID:      parts[1],
		SessionVersion: parts[2],
		NetworkType:    parts[3],
		AddressType:    parts[4],
		Address:        parts[5],
	}, nil
}

func parseConnection(value string) (*Connection, error) {
	parts := strings.Fields(value)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid connection format")
	}
	conn := &Connection{
		NetworkType: parts[0],
		AddressType: parts[1],
		Address:     parts[2],
	}
	if len(parts) > 3 {
		if ttl, err := strconv.Atoi(parts[3]); err == nil {
			conn.TTL = ttl
		}
	}
	if len(parts) > 4 {
		if count, err := strconv.Atoi(parts[4]); err == nil {
			conn.Count = count
		}
	}
	return conn, nil
}

func (c *Connection) String() string {
	if c.TTL > 0 && c.Count > 0 {
		return fmt.Sprintf("c=%s %s %s/%d/%d", c.NetworkType, c.AddressType, c.Address, c.TTL, c.Count)
	}
	if c.TTL > 0 {
		return fmt.Sprintf("c=%s %s %s/%d", c.NetworkType, c.AddressType, c.Address, c.TTL)
	}
	return fmt.Sprintf("c=%s %s %s", c.NetworkType, c.AddressType, c.Address)
}

func parseBandwidth(value string) Bandwidth {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) == 2 {
		return Bandwidth{Type: parts[0], Value: parts[1]}
	}
	return Bandwidth{Type: value}
}

func parseTiming(value string) Timing {
	parts := strings.Fields(value)
	if len(parts) >= 2 {
		return Timing{Start: parts[0], Stop: parts[1]}
	}
	return Timing{Start: value}
}

func parseMedia(value string) (MediaSection, error) {
	parts := strings.Fields(value)
	if len(parts) < 4 {
		return MediaSection{}, fmt.Errorf("invalid media format")
	}

	port, err := strconv.Atoi(strings.Split(parts[1], "/")[0])
	if err != nil {
		return MediaSection{}, fmt.Errorf("invalid port: %w", err)
	}

	return MediaSection{
		Type:       parts[0],
		Port:       port,
		Protocol:   parts[2],
		Formats:    parts[3:],
		Attributes: make([]Attribute, 0),
		Bandwidth:  make([]Bandwidth, 0),
	}, nil
}

func (m *MediaSection) String() string {
	parts := []string{
		fmt.Sprintf("m=%s %d %s %s", m.Type, m.Port, m.Protocol, strings.Join(m.Formats, " ")),
	}

	if m.Title != "" {
		parts = append(parts, fmt.Sprintf("i=%s", m.Title))
	}

	if m.Connection != nil {
		parts = append(parts, m.Connection.String())
	}

	for _, bw := range m.Bandwidth {
		parts = append(parts, fmt.Sprintf("b=%s:%s", bw.Type, bw.Value))
	}

	for _, attr := range m.Attributes {
		if attr.Value != "" {
			parts = append(parts, fmt.Sprintf("a=%s:%s", attr.Name, attr.Value))
		} else {
			parts = append(parts, fmt.Sprintf("a=%s", attr.Name))
		}
	}

	return strings.Join(parts, "\r\n")
}

func parseAttribute(value string) Attribute {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) == 2 {
		return Attribute{Name: parts[0], Value: parts[1]}
	}
	return Attribute{Name: value}
}

func parseCodec(value string) *Codec {
	parts := strings.SplitN(value, " ", 2)
	if len(parts) < 2 {
		return nil
	}

	pt, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil
	}

	codecParts := strings.Split(parts[1], "/")
	if len(codecParts) < 2 {
		return nil
	}

	clockRate, err := strconv.Atoi(codecParts[1])
	if err != nil {
		return nil
	}

	channels := 1
	if len(codecParts) > 2 {
		if ch, err := strconv.Atoi(codecParts[2]); err == nil {
			channels = ch
		}
	}

	return &Codec{
		PayloadType: pt,
		Name:        codecParts[0],
		ClockRate:   clockRate,
		Channels:    channels,
	}
}

// IsRTP determines if the protocol is RTP-based
func (m *MediaSection) IsRTP() bool {
	return strings.Contains(m.Protocol, "RTP")
}

// IsWebRTC determines if this is a WebRTC media section
func (m *MediaSection) IsWebRTC() bool {
	for _, attr := range m.Attributes {
		if attr.Name == "setup" || attr.Name == "fingerprint" {
			return true
		}
	}
	return false
}

// GetAttribute returns an attribute by name
func (m *MediaSection) GetAttribute(name string) (string, bool) {
	for _, attr := range m.Attributes {
		if attr.Name == name {
			return attr.Value, true
		}
	}
	return "", false
}

// AddAttribute adds an attribute to the media section
func (m *MediaSection) AddAttribute(name, value string) {
	m.Attributes = append(m.Attributes, Attribute{Name: name, Value: value})
}

// RemoveAttribute removes an attribute by name
func (m *MediaSection) RemoveAttribute(name string) {
	var attrs []Attribute
	for _, attr := range m.Attributes {
		if attr.Name != name {
			attrs = append(attrs, attr)
		}
	}
	m.Attributes = attrs
}

// GetICEInfo extracts ICE candidate information
func (s *SessionDescription) GetICEInfo() (ufrag, pwd string, candidates []string) {
	for _, attr := range s.Attributes {
		switch attr.Name {
		case "ice-ufrag":
			ufrag = attr.Value
		case "ice-pwd":
			pwd = attr.Value
		}
	}
	return
}
