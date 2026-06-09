package classifier

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	client := NewClient(Config{Endpoint: "http://test:5000", Logger: logger})
	require.NotNil(t, client)
	assert.Equal(t, "http://test:5000", client.endpoint)
}

func TestNewClientDefaults(t *testing.T) {
	client := NewClient(Config{})
	require.NotNil(t, client)
	assert.Equal(t, "http://localhost:5000", client.endpoint)
}

func TestHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		json.NewEncoder(w).Encode(HealthResponse{Status: "healthy", Model: "test", Version: "1.0.0"})
	}))
	defer server.Close()

	client := NewClient(Config{Endpoint: server.URL})
	health, err := client.Health()
	require.NoError(t, err)
	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, "test", health.Model)
}

func TestHealthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(Config{Endpoint: server.URL})
	_, err := client.Health()
	assert.Error(t, err)
}

func TestIsAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(HealthResponse{Status: "healthy"})
	}))
	defer server.Close()

	client := NewClient(Config{Endpoint: server.URL})
	assert.True(t, client.IsAvailable())
}

func TestIsAvailableNotAvailable(t *testing.T) {
	client := NewClient(Config{Endpoint: "http://invalid-url-12345"})
	assert.False(t, client.IsAvailable())
}

func TestClassify(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/classify", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req ClassificationRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "session-1", req.SessionID)

		json.NewEncoder(w).Encode(ClassificationResponse{
			Result:     "human",
			Confidence: 0.95,
			Latency:    12.5,
		})
	}))
	defer server.Close()

	client := NewClient(Config{Endpoint: server.URL})
	result, err := client.Classify("session-1", []byte{0, 1, 2, 3}, 8000)
	require.NoError(t, err)
	assert.Equal(t, "human", result.Result)
	assert.Equal(t, 0.95, result.Confidence)
	assert.Equal(t, 12.5, result.Latency)
}

func TestClassifyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(Config{Endpoint: server.URL})
	_, err := client.Classify("session-1", []byte{0}, 8000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
