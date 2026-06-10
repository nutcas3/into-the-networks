package sip

import (
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTranslator(t *testing.T) {
	logger := logrus.New()
	translator, err := NewTranslator(Config{
		SIPServer: "127.0.0.1",
		SIPPort:   5060,
		Logger:    logger,
	})
	require.NoError(t, err)
	require.NotNil(t, translator)
	defer translator.Close()
}

func TestTranslatorInvite(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	translator, err := NewTranslator(Config{
		SIPServer: "127.0.0.1",
		SIPPort:   5060,
		Logger:    logger,
	})
	require.NoError(t, err)
	defer translator.Close()

	sessionID, err := translator.Invite("alice", "bob", "v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\na=rtpmap:111 opus/48000/2\r\n")
	require.NoError(t, err)
	assert.NotEmpty(t, sessionID)
	assert.True(t, strings.HasPrefix(sessionID, "sip-"))

	// Wait a moment for goroutine to start
	time.Sleep(100 * time.Millisecond)

	// Verify session was created
	session, exists := translator.GetSession(sessionID)
	assert.True(t, exists)
	assert.Equal(t, "alice", session.Caller)
	assert.Equal(t, "bob", session.Callee)
	assert.Equal(t, "inviting", session.State)
}

func TestTranslatorAnswer(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	translator, err := NewTranslator(Config{
		SIPServer: "127.0.0.1",
		SIPPort:   5060,
		Logger:    logger,
	})
	require.NoError(t, err)
	defer translator.Close()

	sessionID, err := translator.Invite("alice", "bob", "v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\ns=-\r\n")
	require.NoError(t, err)

	answerSDP := "v=0\r\no=- 1 1 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n"
	err = translator.Answer(sessionID, answerSDP)
	require.NoError(t, err)

	session, exists := translator.GetSession(sessionID)
	assert.True(t, exists)
	assert.Equal(t, "answered", session.State)
	assert.True(t, session.Answered)
	assert.Equal(t, answerSDP, session.WebRTCSDP)
}

func TestTranslatorAnswerSessionNotFound(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	translator, err := NewTranslator(Config{
		SIPServer: "127.0.0.1",
		SIPPort:   5060,
		Logger:    logger,
	})
	require.NoError(t, err)
	defer translator.Close()

	err = translator.Answer("non-existent", "v=0\r\n")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestTranslatorHangup(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	translator, err := NewTranslator(Config{
		SIPServer: "127.0.0.1",
		SIPPort:   5060,
		Logger:    logger,
	})
	require.NoError(t, err)
	defer translator.Close()

	sessionID, err := translator.Invite("alice", "bob", "v=0\r\n")
	require.NoError(t, err)

	err = translator.Hangup(sessionID)
	require.NoError(t, err)

	// Verify session was deleted
	_, exists := translator.GetSession(sessionID)
	assert.False(t, exists)
}

func TestTranslatorHangupSessionNotFound(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	translator, err := NewTranslator(Config{
		SIPServer: "127.0.0.1",
		SIPPort:   5060,
		Logger:    logger,
	})
	require.NoError(t, err)
	defer translator.Close()

	err = translator.Hangup("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestTranslatorRegister(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	translator, err := NewTranslator(Config{
		SIPServer: "127.0.0.1",
		SIPPort:   5060,
		Logger:    logger,
	})
	require.NoError(t, err)
	defer translator.Close()

	err = translator.Register("user1", "alice", "secret123")
	require.NoError(t, err)

	// Give goroutine time to execute
	time.Sleep(100 * time.Millisecond)
}

func TestParseSIPAddress(t *testing.T) {
	// Test with port
	user, host, port, err := ParseSIPAddress("sip:alice@example.com:5060")
	require.NoError(t, err)
	assert.Equal(t, "alice", user)
	assert.Equal(t, "example.com", host)
	assert.Equal(t, 5060, port)

	// Test without port
	user, host, port, err = ParseSIPAddress("sip:bob@example.com")
	require.NoError(t, err)
	assert.Equal(t, "bob", user)
	assert.Equal(t, "example.com", host)
	assert.Equal(t, 5060, port)

	// Test invalid format
	_, _, _, err = ParseSIPAddress("invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid SIP address format")
}

func TestConvertWebRTCSDPToSIP(t *testing.T) {
	webrtcSDP := "v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\na=rtpmap:111 opus/48000/2\r\na=candidate:1 1 UDP 2130706431 192.168.1.1 12345 typ host\r\na=fingerprint:sha-256 AB:CD:EF:12:34:56\r\na=setup:actpass\r\na=ice-ufrag:abc123\r\na=ice-pwd:def456\r\n"

	sipSDP := ConvertWebRTCSDPToSIP(webrtcSDP)

	// Should not contain WebRTC-specific attributes
	assert.NotContains(t, sipSDP, "a=candidate:")
	assert.NotContains(t, sipSDP, "a=fingerprint:")
	assert.NotContains(t, sipSDP, "a=setup:")
	assert.NotContains(t, sipSDP, "a=ice-ufrag:")
	assert.NotContains(t, sipSDP, "a=ice-pwd:")

	// Should use RTP/AVP instead of UDP/TLS/RTP/SAVPF
	assert.Contains(t, sipSDP, "RTP/AVP")
	assert.NotContains(t, sipSDP, "UDP/TLS/RTP/SAVPF")

	// Should still have rtpmap
	assert.Contains(t, sipSDP, "a=rtpmap:111 opus/48000/2")
}

func TestConvertSIPSDPToWebRTC(t *testing.T) {
	sipSDP := "v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=audio 9 RTP/AVP 111\r\na=rtpmap:111 opus/48000/2\r\na=rtcp:9 IN IP4 0.0.0.0\r\n"

	webrtcSDP := ConvertSIPSDPToWebRTC(sipSDP)

	// Should use UDP/TLS/RTP/SAVPF
	assert.Contains(t, webrtcSDP, "UDP/TLS/RTP/SAVPF")
	assert.NotContains(t, webrtcSDP, "RTP/AVP")

	// Should add DTLS fingerprint
	assert.Contains(t, webrtcSDP, "a=setup:actpass")
	assert.Contains(t, webrtcSDP, "a=fingerprint:sha-256")
}

func TestGenerateCallID(t *testing.T) {
	id1 := GenerateCallID()

	assert.NotEmpty(t, id1)
	assert.Contains(t, id1, "@webrtc-gateway")
	assert.Greater(t, len(id1), len("@webrtc-gateway"))
}

func TestGenerateTag(t *testing.T) {
	tag1 := GenerateTag()

	assert.NotEmpty(t, tag1)
	assert.Greater(t, len(tag1), 0)
}

func TestGenerateSessionID(t *testing.T) {
	id := generateSessionID()
	assert.True(t, strings.HasPrefix(id, "sip-"))
	assert.NotEmpty(t, id)
}
