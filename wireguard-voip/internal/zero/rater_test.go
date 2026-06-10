package zero

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRater(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	r := NewRater(Config{Enabled: true, Logger: logger})
	require.NotNil(t, r)
	assert.True(t, r.IsEnabled())
	assert.Len(t, r.GetPolicies(), 1)
}

func TestNewRaterDisabled(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	r := NewRater(Config{Enabled: false, Logger: logger})
	assert.False(t, r.IsEnabled())
}

func TestAddPolicy(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	r := NewRater(Config{Enabled: true, Logger: logger})

	policy := &Policy{
		Name:        "test-policy",
		Description: "Test policy",
		Subnets:     []string{"192.168.1.0/24"},
		Ports:       []int{8080},
		Protocols:   []string{"tcp"},
		Enabled:     true,
	}

	err := r.AddPolicy(policy)
	require.NoError(t, err)
	assert.Len(t, r.GetPolicies(), 2)
}

func TestAddPolicyNoName(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	r := NewRater(Config{Logger: logger})
	err := r.AddPolicy(&Policy{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestRemovePolicy(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	r := NewRater(Config{Enabled: true, Logger: logger})

	err := r.RemovePolicy("voip-default")
	require.NoError(t, err)
	assert.Len(t, r.GetPolicies(), 0)
}

func TestRemovePolicyNotFound(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	r := NewRater(Config{Enabled: true, Logger: logger})
	err := r.RemovePolicy("nonexistent")
	assert.Error(t, err)
}

func TestIsZeroRated(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	r := NewRater(Config{Enabled: true, Logger: logger})

	// Should match default policy
	assert.True(t, r.IsZeroRated("8.8.8.8", 5060, "udp"))

	// Port not in policy
	assert.False(t, r.IsZeroRated("8.8.8.8", 80, "udp"))
}

func TestIsZeroRatedDisabled(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	r := NewRater(Config{Enabled: false, Logger: logger})
	assert.False(t, r.IsZeroRated("8.8.8.8", 5060, "udp"))
}

func TestIsZeroRatedCustomPolicy(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	r := NewRater(Config{Enabled: true, Logger: logger})
	r.RemovePolicy("voip-default")

	// Add specific subnet policy
	r.AddPolicy(&Policy{
		Name:      "custom",
		Subnets:   []string{"10.0.0.0/8"},
		Ports:     []int{8080},
		Protocols: []string{"tcp"},
		Enabled:   true,
	})

	assert.True(t, r.IsZeroRated("10.1.2.3", 8080, "tcp"))
	assert.False(t, r.IsZeroRated("192.168.1.1", 8080, "tcp"))
}

func TestEnableDisable(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	r := NewRater(Config{Enabled: true, Logger: logger})
	assert.True(t, r.IsEnabled())

	r.Disable()
	assert.False(t, r.IsEnabled())

	r.Enable()
	assert.True(t, r.IsEnabled())
}

func TestNotifyCarrier(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	r := NewRater(Config{Enabled: true, Logger: logger})
	// No carrier API configured - should return nil
	err := r.NotifyCarrier("alice", "session-1", true)
	assert.NoError(t, err)
}
