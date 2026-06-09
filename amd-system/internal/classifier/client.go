package classifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Client communicates with the Python ML classification service
type Client struct {
	endpoint   string
	httpClient *http.Client
	logger     *logrus.Logger
}

// Config holds classifier client configuration
type Config struct {
	Endpoint string
	Logger   *logrus.Logger
}

// ClassificationRequest represents an audio classification request
type ClassificationRequest struct {
	SessionID   string  `json:"session_id"`
	AudioData   []byte  `json:"audio_data"`
	SampleRate  int     `json:"sample_rate"`
	Channels    int     `json:"channels"`
	Format      string  `json:"format"`
}

// ClassificationResponse represents the ML service response
type ClassificationResponse struct {
	Result     string  `json:"result"`
	Confidence float64 `json:"confidence"`
	Features   map[string]interface{} `json:"features,omitempty"`
	Latency    float64 `json:"latency_ms"`
}

// HealthResponse represents the ML service health check
type HealthResponse struct {
	Status    string `json:"status"`
	Model     string `json:"model"`
	Version   string `json:"version"`
	Timestamp int64  `json:"timestamp"`
}

// NewClient creates a new classifier client
func NewClient(cfg Config) *Client {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://localhost:5000"
	}

	return &Client{
		endpoint:   cfg.Endpoint,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     cfg.Logger,
	}
}

// Classify sends audio data to the ML service for classification
func (c *Client) Classify(sessionID string, audioData []byte, sampleRate int) (*ClassificationResponse, error) {
	reqBody := ClassificationRequest{
		SessionID:  sessionID,
		AudioData:  audioData,
		SampleRate: sampleRate,
		Channels:   1,
		Format:     "pcm_16bit",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(
		fmt.Sprintf("%s/classify", c.endpoint),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to call classifier: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("classifier returned status %d: %s", resp.StatusCode, string(body))
	}

	var result ClassificationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"session_id": sessionID,
		"result":     result.Result,
		"confidence": result.Confidence,
		"latency_ms": result.Latency,
	}).Debug("Classification completed")

	return &result, nil
}

// ClassifyRealtime is optimized for streaming audio chunks
func (c *Client) ClassifyRealtime(sessionID string, audioData []byte, sampleRate int) (*ClassificationResponse, error) {
	// For realtime, use a shorter timeout
	client := &http.Client{Timeout: 2 * time.Second}

	reqBody := ClassificationRequest{
		SessionID:  sessionID,
		AudioData:  audioData,
		SampleRate: sampleRate,
		Channels:   1,
		Format:     "pcm_16bit",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := client.Post(
		fmt.Sprintf("%s/classify/realtime", c.endpoint),
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return nil, fmt.Errorf("realtime classification failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("classifier returned status %d", resp.StatusCode)
	}

	var result ClassificationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Health checks if the ML service is available
func (c *Client) Health() (*HealthResponse, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/health", c.endpoint))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, err
	}

	return &health, nil
}

// IsAvailable checks if the classifier service is reachable
func (c *Client) IsAvailable() bool {
	_, err := c.Health()
	return err == nil
}
