package esl

import (
	"testing"
	"time"
)

func TestNewBuffer(t *testing.T) {
	config := BufferConfig{
		MaxSize:       100,
		FlushInterval: 1 * time.Second,
		BatchSize:     10,
	}

	buffer := NewBuffer(config)

	if buffer.maxSize != 100 {
		t.Errorf("Expected max size 100, got %d", buffer.maxSize)
	}

	if buffer.flushTime != 1*time.Second {
		t.Errorf("Expected flush interval 1s, got %v", buffer.flushTime)
	}

	if cap(buffer.eventChan) != 100 {
		t.Errorf("Expected channel capacity 100, got %d", cap(buffer.eventChan))
	}
}

func TestBufferEnqueue(t *testing.T) {
	config := BufferConfig{MaxSize: 5, FlushInterval: 100 * time.Millisecond, BatchSize: 2}
	buffer := NewBuffer(config)
	defer buffer.Stop()

	event := &Event{
		Headers: map[string]string{"Event-Name": "TEST"},
		Body:    []byte("test body"),
	}

	err := buffer.Enqueue(event)
	if err != nil {
		t.Errorf("Unexpected error enqueuing event: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	if buffer.Size() != 1 {
		t.Errorf("Expected buffer size 1, got %d", buffer.Size())
	}
}

func TestBufferEnqueueOverflow(t *testing.T) {
	config := BufferConfig{MaxSize: 3, FlushInterval: 50 * time.Millisecond, BatchSize: 10}
	buffer := NewBuffer(config)
	defer buffer.Stop()

	events := make([]*Event, 5)
	for i := range 5 {
		events[i] = &Event{
			Headers: map[string]string{"Event-Name": "TEST", "Index": string(rune(i))},
			Body:    []byte("test body"),
		}
		buffer.Enqueue(events[i])
	}

	time.Sleep(200 * time.Millisecond)

	stats := buffer.GetStats()
	if stats["current_size"].(int) > 3 {
		t.Errorf("Buffer size should not exceed max size, got %d", stats["current_size"].(int))
	}

	if stats["dropped"].(int64) == 0 {
		t.Error("Expected some events to be dropped")
	}
}

func TestBufferDequeue(t *testing.T) {
	config := BufferConfig{MaxSize: 5, FlushInterval: 50 * time.Millisecond, BatchSize: 2}
	buffer := NewBuffer(config)
	defer buffer.Stop()

	event := &Event{
		Headers: map[string]string{"Event-Name": "TEST"},
		Body:    []byte("test body"),
	}

	buffer.Enqueue(event)
	time.Sleep(200 * time.Millisecond)

	dequeued, exists := buffer.Dequeue()
	if !exists {
		t.Error("Expected event to be dequeued")
	}

	if dequeued.Headers["Event-Name"] != "TEST" {
		t.Errorf("Expected event name TEST, got %s", dequeued.Headers["Event-Name"])
	}
}

func TestBufferFlush(t *testing.T) {
	config := BufferConfig{MaxSize: 5, FlushInterval: 50 * time.Millisecond, BatchSize: 2}
	buffer := NewBuffer(config)
	defer buffer.Stop()

	events := make([]*Event, 3)
	for i := range 3 {
		events[i] = &Event{
			Headers: map[string]string{"Event-Name": "TEST", "Index": string(rune(i))},
		}
		buffer.Enqueue(events[i])
	}

	time.Sleep(200 * time.Millisecond)

	flushed, err := buffer.Flush()
	if err != nil {
		t.Errorf("Unexpected error flushing: %v", err)
	}

	if len(flushed) != 3 {
		t.Errorf("Expected 3 flushed events, got %d", len(flushed))
	}

	if buffer.Size() != 0 {
		t.Errorf("Expected buffer to be empty after flush, got %d", buffer.Size())
	}
}

func TestBufferClear(t *testing.T) {
	config := BufferConfig{MaxSize: 5, FlushInterval: 50 * time.Millisecond, BatchSize: 2}
	buffer := NewBuffer(config)
	defer buffer.Stop()

	for range 3 {
		buffer.Enqueue(&Event{
			Headers: map[string]string{"Event-Name": "TEST"},
		})
	}

	time.Sleep(200 * time.Millisecond)

	err := buffer.Clear()
	if err != nil {
		t.Errorf("Unexpected error clearing: %v", err)
	}

	if buffer.Size() != 0 {
		t.Errorf("Expected buffer to be empty after clear, got %d", buffer.Size())
	}
}

func TestBufferGetStats(t *testing.T) {
	config := BufferConfig{MaxSize: 10, FlushInterval: 50 * time.Millisecond, BatchSize: 2}
	buffer := NewBuffer(config)
	defer buffer.Stop()

	for range 5 {
		buffer.Enqueue(&Event{
			Headers: map[string]string{"Event-Name": "TEST"},
		})
	}

	time.Sleep(200 * time.Millisecond)

	stats := buffer.GetStats()

	if stats["max_size"].(int) != 10 {
		t.Errorf("Expected max size 10, got %d", stats["max_size"].(int))
	}

	if stats["current_size"].(int) != 5 {
		t.Errorf("Expected current size 5, got %d", stats["current_size"].(int))
	}

	if stats["utilization"].(float64) != 0.5 {
		t.Errorf("Expected utilization 0.5, got %f", stats["utilization"].(float64))
	}
}

func TestBufferIsFull(t *testing.T) {
	config := BufferConfig{MaxSize: 3, FlushInterval: 50 * time.Millisecond, BatchSize: 10}
	buffer := NewBuffer(config)
	defer buffer.Stop()

	if buffer.IsFull() {
		t.Error("Buffer should not be full initially")
	}

	for range 3 {
		buffer.Enqueue(&Event{
			Headers: map[string]string{"Event-Name": "TEST"},
		})
	}

	time.Sleep(200 * time.Millisecond)

	if !buffer.IsFull() {
		t.Error("Buffer should be full after adding events")
	}
}

func TestBufferIsEmpty(t *testing.T) {
	config := BufferConfig{MaxSize: 5, FlushInterval: 50 * time.Millisecond, BatchSize: 2}
	buffer := NewBuffer(config)
	defer buffer.Stop()

	if !buffer.IsEmpty() {
		t.Error("Buffer should be empty initially")
	}

	buffer.Enqueue(&Event{
		Headers: map[string]string{"Event-Name": "TEST"},
	})

	time.Sleep(200 * time.Millisecond)

	if buffer.IsEmpty() {
		t.Error("Buffer should not be empty after adding event")
	}
}

func TestBufferGetDropRate(t *testing.T) {
	config := BufferConfig{MaxSize: 3, FlushInterval: 50 * time.Millisecond, BatchSize: 10}
	buffer := NewBuffer(config)
	defer buffer.Stop()

	if buffer.GetDropRate() != 0 {
		t.Error("Drop rate should be 0 initially")
	}

	for range 5 {
		buffer.Enqueue(&Event{
			Headers: map[string]string{"Event-Name": "TEST"},
		})
	}

	time.Sleep(200 * time.Millisecond)

	dropRate := buffer.GetDropRate()
	if dropRate == 0 {
		t.Error("Expected some drops when buffer overflows")
	}
}

func TestBufferGetEventsByType(t *testing.T) {
	config := BufferConfig{MaxSize: 10, FlushInterval: 50 * time.Millisecond, BatchSize: 2}
	buffer := NewBuffer(config)
	defer buffer.Stop()

	events := []string{"CHANNEL_CREATE", "CHANNEL_ANSWER", "CHANNEL_CREATE"}
	for _, eventType := range events {
		buffer.Enqueue(&Event{
			Headers: map[string]string{"Event-Name": eventType},
		})
	}

	time.Sleep(200 * time.Millisecond)

	createEvents := buffer.GetEventsByType("CHANNEL_CREATE")
	if len(createEvents) != 2 {
		t.Errorf("Expected 2 CHANNEL_CREATE events, got %d", len(createEvents))
	}

	answerEvents := buffer.GetEventsByType("CHANNEL_ANSWER")
	if len(answerEvents) != 1 {
		t.Errorf("Expected 1 CHANNEL_ANSWER event, got %d", len(answerEvents))
	}
}

func TestBufferGetEventsByTimeRange(t *testing.T) {
	config := BufferConfig{MaxSize: 10, FlushInterval: 50 * time.Millisecond, BatchSize: 2}
	buffer := NewBuffer(config)
	defer buffer.Stop()

	now := time.Now()

	event1 := &Event{
		Headers:    map[string]string{"Event-Name": "TEST1"},
		ReceivedAt: now.Add(-2 * time.Hour),
	}

	event2 := &Event{
		Headers:    map[string]string{"Event-Name": "TEST2"},
		ReceivedAt: now,
	}

	event3 := &Event{
		Headers:    map[string]string{"Event-Name": "TEST3"},
		ReceivedAt: now.Add(2 * time.Hour),
	}

	buffer.Enqueue(event1)
	buffer.Enqueue(event2)
	buffer.Enqueue(event3)

	time.Sleep(200 * time.Millisecond)

	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Hour)

	events := buffer.GetEventsByTimeRange(start, end)
	if len(events) != 1 {
		t.Errorf("Expected 1 event in time range, got %d", len(events))
	}

	if events[0].Headers["Event-Name"] != "TEST2" {
		t.Errorf("Expected TEST2 event, got %s", events[0].Headers["Event-Name"])
	}
}
