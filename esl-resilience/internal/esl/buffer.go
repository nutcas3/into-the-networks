package esl

import (
	"context"
	"sync"
	"time"
)

type Buffer struct {
	events    []*Event
	maxSize   int
	mu        sync.RWMutex
	dropped   int64
	processed int64
	flushTime time.Duration

	// Channel for non-blocking operations
	eventChan chan *Event
	ctx       context.Context
	cancel    context.CancelFunc
}

type BufferConfig struct {
	MaxSize       int           `yaml:"max_size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
	BatchSize     int           `yaml:"batch_size"`
}

func DefaultBufferConfig() BufferConfig {
	return BufferConfig{
		MaxSize:       10000,
		FlushInterval: 1 * time.Second,
		BatchSize:     100,
	}
}

func NewBuffer(config BufferConfig) *Buffer {
	ctx, cancel := context.WithCancel(context.Background())

	b := &Buffer{
		events:    make([]*Event, 0, config.MaxSize),
		maxSize:   config.MaxSize,
		flushTime: config.FlushInterval,
		eventChan: make(chan *Event, config.MaxSize),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start background flush goroutine
	go b.backgroundFlush()

	return b
}

func (b *Buffer) Enqueue(event *Event) error {
	select {
	case b.eventChan <- event:
		return nil
	default:
		// Channel is full, drop oldest event
		b.mu.Lock()
		defer b.mu.Unlock()

		if len(b.events) > 0 {
			// Remove oldest event
			b.events = b.events[1:]
			b.dropped++
		}

		// Add new event
		b.events = append(b.events, event)
		return nil
	}
}

func (b *Buffer) Dequeue() (*Event, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.events) == 0 {
		return nil, false
	}

	event := b.events[0]
	b.events = b.events[1:]
	b.processed++

	return event, true
}

func (b *Buffer) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.events)
}

func (b *Buffer) Flush() ([]*Event, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	events := make([]*Event, len(b.events))
	copy(events, b.events)

	b.events = b.events[:0] // Clear slice but keep capacity

	return events, nil
}

func (b *Buffer) Clear() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.events = b.events[:0] // Clear slice but keep capacity
	return nil
}

func (b *Buffer) backgroundFlush() {
	ticker := time.NewTicker(b.flushTime)
	defer ticker.Stop()

	batch := make([]*Event, 0, 100) // Default batch size

	for {
		select {
		case <-b.ctx.Done():
			return

		case event := <-b.eventChan:
			batch = append(batch, event)

			// Process batch when it's full or on timer
			if len(batch) >= cap(batch) {
				b.processBatch(batch)
				batch = batch[:0] // Reset batch
			}

		case <-ticker.C:
			// Flush any pending events
			if len(batch) > 0 {
				b.processBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (b *Buffer) processBatch(batch []*Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, event := range batch {
		if len(b.events) >= b.maxSize {
			// Remove oldest events to make room
			overflow := len(b.events) + len(batch) - b.maxSize
			if overflow > 0 {
				b.events = b.events[overflow:]
				b.dropped += int64(overflow)
			}
		}
		b.events = append(b.events, event)
	}
}

func (b *Buffer) Stop() error {
	b.cancel()
	close(b.eventChan)
	return nil
}

func (b *Buffer) GetStats() map[string]any {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return map[string]any{
		"current_size":   len(b.events),
		"max_size":       b.maxSize,
		"dropped":        b.dropped,
		"processed":      b.processed,
		"utilization":    float64(len(b.events)) / float64(b.maxSize),
		"channel_size":   len(b.eventChan),
		"flush_interval": b.flushTime.String(),
	}
}

func (b *Buffer) IsFull() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.events) >= b.maxSize
}

func (b *Buffer) IsEmpty() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.events) == 0
}

func (b *Buffer) GetDropRate() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	total := b.dropped + b.processed
	if total == 0 {
		return 0
	}

	return float64(b.dropped) / float64(total) * 100
}

func (b *Buffer) Peek() (*Event, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.events) == 0 {
		return nil, false
	}

	return b.events[0], true
}

func (b *Buffer) GetEventsByTimeRange(start, end time.Time) []*Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var events []*Event
	for _, event := range b.events {
		if event.ReceivedAt.After(start) && event.ReceivedAt.Before(end) {
			events = append(events, event)
		}
	}

	return events
}

func (b *Buffer) GetEventsByType(eventType string) []*Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var events []*Event
	for _, event := range b.events {
		if event.Headers["Event-Name"] == eventType {
			events = append(events, event)
		}
	}

	return events
}
