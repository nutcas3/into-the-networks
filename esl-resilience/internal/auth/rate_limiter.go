package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
	AllowTenant(ctx context.Context, tenantID uuid.UUID, limit int, window time.Duration) (bool, error)
	AllowUser(ctx context.Context, userID string, limit int, window time.Duration) (bool, error)
	GetStats(ctx context.Context, key string) (*RateLimitStats, error)
	Reset(ctx context.Context, key string) error
}

type RateLimitStats struct {
	CurrentCount int64     `json:"current_count"`
	Limit        int       `json:"limit"`
	Window       string    `json:"window"`
	ResetTime    time.Time `json:"reset_time"`
	Remaining    int64     `json:"remaining"`
}

type RedisRateLimiter struct {
	client *redis.Client
	logger *logrus.Logger
	prefix string
}

func NewRedisRateLimiter(redisAddr, password, prefix string) (*RedisRateLimiter, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: password,
		DB:       0,
	})

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	if prefix == "" {
		prefix = "rate_limit"
	}

	logger.WithField("redis_addr", redisAddr).Info("Redis rate limiter initialized")

	return &RedisRateLimiter{
		client: client,
		logger: logger,
		prefix: prefix,
	}, nil
}

func (r *RedisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 || window <= 0 {
		return true, nil // No rate limiting
	}

	redisKey := fmt.Sprintf("%s:%s", r.prefix, key)

	// Use Redis INCR with EXPIRE for sliding window
	pipe := r.client.Pipeline()
	incrCmd := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, window)

	if _, err := pipe.Exec(ctx); err != nil {
		r.logger.WithError(err).WithField("key", key).Error("Rate limit check failed")
		return false, fmt.Errorf("rate limit check failed: %w", err)
	}

	current := incrCmd.Val()
	allowed := current <= int64(limit)

	r.logger.WithFields(logrus.Fields{
		"key":     key,
		"current": current,
		"limit":   limit,
		"allowed": allowed,
	}).Debug("Rate limit check")

	return allowed, nil
}

func (r *RedisRateLimiter) AllowTenant(ctx context.Context, tenantID uuid.UUID, limit int, window time.Duration) (bool, error) {
	key := fmt.Sprintf("tenant:%s", tenantID.String())
	return r.Allow(ctx, key, limit, window)
}

func (r *RedisRateLimiter) AllowUser(ctx context.Context, userID string, limit int, window time.Duration) (bool, error) {
	key := fmt.Sprintf("user:%s", userID)
	return r.Allow(ctx, key, limit, window)
}

func (r *RedisRateLimiter) GetStats(ctx context.Context, key string) (*RateLimitStats, error) {
	redisKey := fmt.Sprintf("%s:%s", r.prefix, key)

	pipe := r.client.Pipeline()
	countCmd := pipe.Get(ctx, redisKey)
	ttlCmd := pipe.TTL(ctx, redisKey)

	if _, err := pipe.Exec(ctx); err != nil {
		// Key might not exist, that's okay
		if err.Error() != "redis: nil" {
			return nil, fmt.Errorf("failed to get rate limit stats: %w", err)
		}
	}

	var count int64 = 0
	var ttl time.Duration = 0

	if countCmd.Err() == nil {
		count, _ = countCmd.Int64()
	}

	if ttlCmd.Err() == nil {
		ttl = ttlCmd.Val()
	}

	stats := &RateLimitStats{
		CurrentCount: count,
		Window:       ttl.String(),
		ResetTime:    time.Now().Add(ttl),
		Remaining:    int64(0),
	}

	// Calculate remaining (this is approximate since we don't know the exact limit)
	if ttl > 0 {
		stats.Remaining = 0 // Will be set by caller based on their limit
	}

	return stats, nil
}

// Reset resets the rate limit counter for a key
func (r *RedisRateLimiter) Reset(ctx context.Context, key string) error {
	redisKey := fmt.Sprintf("%s:%s", r.prefix, key)

	if err := r.client.Del(ctx, redisKey).Err(); err != nil {
		r.logger.WithError(err).WithField("key", key).Error("Failed to reset rate limit")
		return fmt.Errorf("failed to reset rate limit: %w", err)
	}

	r.logger.WithField("key", key).Info("Rate limit reset")
	return nil
}

// Close closes the Redis connection
func (r *RedisRateLimiter) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// InMemoryRateLimiter implements RateLimiter using in-memory storage (for fallback/testing)
type InMemoryRateLimiter struct {
	counters map[string]*Counter
	mu       sync.RWMutex
	logger   *logrus.Logger
}

// Counter tracks rate limit state
type Counter struct {
	Count     int64
	ResetTime time.Time
	Window    time.Duration
	Limit     int
}

func NewInMemoryRateLimiter() *InMemoryRateLimiter {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &InMemoryRateLimiter{
		counters: make(map[string]*Counter),
		logger:   logger,
	}
}

func (m *InMemoryRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 || window <= 0 {
		return true, nil // No rate limiting
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	counter, exists := m.counters[key]

	if !exists || now.After(counter.ResetTime) {
		// Reset or create counter
		counter = &Counter{
			Count:     0,
			ResetTime: now.Add(window),
			Window:    window,
			Limit:     limit,
		}
		m.counters[key] = counter
	}

	counter.Count++
	allowed := counter.Count <= int64(limit)

	m.logger.WithFields(logrus.Fields{
		"key":     key,
		"current": counter.Count,
		"limit":   limit,
		"allowed": allowed,
	}).Debug("In-memory rate limit check")

	return allowed, nil
}

func (m *InMemoryRateLimiter) AllowTenant(ctx context.Context, tenantID uuid.UUID, limit int, window time.Duration) (bool, error) {
	key := fmt.Sprintf("tenant:%s", tenantID.String())
	return m.Allow(ctx, key, limit, window)
}

func (m *InMemoryRateLimiter) AllowUser(ctx context.Context, userID string, limit int, window time.Duration) (bool, error) {
	key := fmt.Sprintf("user:%s", userID)
	return m.Allow(ctx, key, limit, window)
}

func (m *InMemoryRateLimiter) GetStats(ctx context.Context, key string) (*RateLimitStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counter, exists := m.counters[key]
	if !exists {
		return &RateLimitStats{
			CurrentCount: 0,
			Window:       "0s",
			ResetTime:    time.Now(),
			Remaining:    0,
		}, nil
	}

	remaining := int64(counter.Limit) - counter.Count
	if remaining < 0 {
		remaining = 0
	}

	return &RateLimitStats{
		CurrentCount: counter.Count,
		Limit:        counter.Limit,
		Window:       counter.Window.String(),
		ResetTime:    counter.ResetTime,
		Remaining:    remaining,
	}, nil
}

func (m *InMemoryRateLimiter) Reset(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.counters, key)
	m.logger.WithField("key", key).Info("In-memory rate limit reset")
	return nil
}

func (m *InMemoryRateLimiter) Close() error {
	return nil
}
