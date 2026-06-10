package tunnel

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Logger: logger})
	require.NotNil(t, svc)
	assert.Equal(t, "wg0", svc.name)
	assert.Equal(t, 1420, svc.mtu)
	assert.False(t, svc.IsUp())
}

func TestNewServiceCustom(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{
		Name:      "wg1",
		LocalAddr: "10.0.0.1:51820",
		MTU:       1380,
		Logger:    logger,
	})
	require.NotNil(t, svc)
	assert.Equal(t, "wg1", svc.name)
	assert.Equal(t, 1380, svc.mtu)
}

func TestUpDown(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{
		VoIPSubnets: []string{"10.0.0.0/8"},
		Logger:      logger,
	})

	assert.False(t, svc.IsUp())
	require.NoError(t, svc.Up())
	assert.True(t, svc.IsUp())

	routes := svc.GetRoutes()
	assert.Len(t, routes, 1)

	require.NoError(t, svc.Down())
	assert.False(t, svc.IsUp())
	assert.Len(t, svc.GetRoutes(), 0)
}

func TestUpAlreadyUp(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Logger: logger})
	require.NoError(t, svc.Up())
	// Should be idempotent
	require.NoError(t, svc.Up())
	assert.True(t, svc.IsUp())
}

func TestDownAlreadyDown(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Logger: logger})
	require.NoError(t, svc.Down())
	assert.False(t, svc.IsUp())
}

func TestAddRoute(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Logger: logger})
	err := svc.AddRoute("192.168.1.0/24")
	require.NoError(t, err)

	routes := svc.GetRoutes()
	assert.Len(t, routes, 1)
}

func TestAddRouteInvalid(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{Logger: logger})
	err := svc.AddRoute("invalid")
	assert.Error(t, err)
}

func TestGetStats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	svc := NewService(Config{
		Name:      "wg0",
		LocalAddr: "10.0.0.1:51820",
		Logger:    logger,
	})

	stats := svc.GetStats()
	assert.Equal(t, "wg0", stats["name"])
	assert.Equal(t, "10.0.0.1:51820", stats["local_addr"])
	assert.Equal(t, 1420, stats["mtu"])
	assert.Equal(t, false, stats["up"])
	assert.NotZero(t, stats["timestamp"])
}

func TestDefaultVoIPSubnets(t *testing.T) {
	subnets := DefaultVoIPSubnets()
	assert.Len(t, subnets, 1)
	assert.Equal(t, "0.0.0.0/0", subnets[0])
}
