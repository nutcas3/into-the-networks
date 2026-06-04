package internal

import (
	"context"
	"testing"
	"time"

	"github.com/nutcas3/esl-resilience/internal/esl"
	"github.com/nutcas3/esl-resilience/internal/monitor"
	"github.com/nutcas3/esl-resilience/internal/state"
)

func TestServerIntegration(t *testing.T) {
	config := DefaultConfig()
	config.FreeSWITCH.Host = "localhost"
	config.FreeSWITCH.Port = 8021
	config.FreeSWITCH.Password = "ClueCon"

	server := NewServer(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Start(ctx)
	if err != nil {
		t.Skipf("Skipping integration test - cannot connect to FreeSWITCH: %v", err)
	}

	defer server.Stop()

	time.Sleep(1 * time.Second)

	stats := server.GetStats()
	if stats == nil {
		t.Error("Expected stats to be returned")
	}

	clientStats := stats["client"].(map[string]any)
	if clientStats["connected"].(bool) != true {
		t.Error("Expected client to be connected")
	}
}

func TestServerComponentsIntegration(t *testing.T) {
	config := DefaultConfig()
	config.ESL.BufferSize = 100
	config.Monitor.Port = 9091

	server := NewServer(config)

	if server.client == nil {
		t.Error("Expected client to be initialized")
	}

	if server.stateMachine == nil {
		t.Error("Expected state machine to be initialized")
	}

	if server.buffer == nil {
		t.Error("Expected buffer to be initialized")
	}

	if server.monitor == nil {
		t.Error("Expected monitor to be initialized")
	}

	if server.logger == nil {
		t.Error("Expected logger to be initialized")
	}
}

func TestEventHandlingIntegration(t *testing.T) {
	machine := state.NewMachine()

	bufferConfig := esl.DefaultBufferConfig()
	bufferConfig.MaxSize = 10
	buffer := esl.NewBuffer(bufferConfig)
	defer buffer.Stop()

	event := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":                 "test-integration-uuid",
			"Event-Name":                "CHANNEL_CREATE",
			"Caller-Username":           "test-caller",
			"Caller-Destination-Number": "1234567890",
			"Call-Direction":            "inbound",
		},
		ReceivedAt: time.Now(),
	}

	err := machine.HandleEvent(event)
	if err != nil {
		t.Errorf("Error handling event: %v", err)
	}

	err = buffer.Enqueue(event)
	if err != nil {
		t.Errorf("Error enqueuing event: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if buffer.Size() != 1 {
		t.Errorf("Expected buffer size 1, got %d", buffer.Size())
	}

	state, exists := machine.GetCallState("test-integration-uuid")
	if !exists {
		t.Error("Expected call to exist")
	}

	if state != esl.CallStateInit {
		t.Errorf("Expected CallStateInit, got %s", state)
	}
}

func TestMonitorIntegration(t *testing.T) {
	monitor := monitor.NewPrometheusMonitor()

	go func() {
		monitor.Start()
	}()

	time.Sleep(200 * time.Millisecond)

	monitor.RecordConnection(true)
	monitor.RecordEvent("CHANNEL_CREATE")
	monitor.IncrementCounter("esl_events_processed_total", nil)
	monitor.SetGauge("sip_active_calls", 5.0, nil)

	if !monitor.IsRunning() {
		t.Error("Expected monitor to be running")
	}

	err := monitor.Stop()
	if err != nil {
		t.Errorf("Error stopping monitor: %v", err)
	}
}

func TestCircuitBreakerIntegration(t *testing.T) {
	cb := esl.NewCircuitBreaker(3, 100*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()

	if cb.IsOpen() {
		t.Error("Circuit should not be open yet")
	}

	cb.RecordFailure()

	if !cb.IsOpen() {
		t.Error("Circuit should be open now")
	}

	time.Sleep(150 * time.Millisecond)

	if cb.IsOpen() {
		t.Error("Circuit should be half-open after timeout")
	}

	cb.RecordSuccess()

	if cb.IsOpen() {
		t.Error("Circuit should be closed after success")
	}
}

func TestBufferIntegration(t *testing.T) {
	config := esl.DefaultBufferConfig()
	config.MaxSize = 5
	config.FlushInterval = 50 * time.Millisecond
	config.BatchSize = 2

	buffer := esl.NewBuffer(config)
	defer buffer.Stop()

	events := make([]*esl.Event, 7)
	for i := range 7 {
		events[i] = &esl.Event{
			Headers: map[string]string{
				"Event-Name": "TEST",
				"Index":      string(rune('A' + i)),
			},
			ReceivedAt: time.Now(),
		}

		err := buffer.Enqueue(events[i])
		if err != nil {
			t.Errorf("Error enqueueing event %d: %v", i, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	stats := buffer.GetStats()
	if stats["current_size"].(int) > 5 {
		t.Errorf("Buffer size should not exceed max, got %d", stats["current_size"].(int))
	}

	if stats["dropped"].(int64) == 0 {
		t.Error("Expected some events to be dropped")
	}

	flushed, err := buffer.Flush()
	if err != nil {
		t.Errorf("Error flushing buffer: %v", err)
	}

	if len(flushed) == 0 {
		t.Error("Expected some events to be flushed")
	}
}

func TestStateMachineIntegration(t *testing.T) {
	machine := state.NewMachine()

	uuid := "integration-test-uuid"

	lifecycle := []struct {
		event      string
		headers    map[string]string
		finalState esl.CallState
	}{
		{
			event: "CHANNEL_CREATE",
			headers: map[string]string{
				"Unique-ID":                 uuid,
				"Event-Name":                "CHANNEL_CREATE",
				"Caller-Username":           "test-caller",
				"Caller-Destination-Number": "1234567890",
				"Call-Direction":            "inbound",
			},
			finalState: esl.CallStateInit,
		},
		{
			event: "CHANNEL_PROGRESS",
			headers: map[string]string{
				"Unique-ID":  uuid,
				"Event-Name": "CHANNEL_PROGRESS",
			},
			finalState: esl.CallStateEarly,
		},
		{
			event: "CHANNEL_ANSWER",
			headers: map[string]string{
				"Unique-ID":  uuid,
				"Event-Name": "CHANNEL_ANSWER",
			},
			finalState: esl.CallStateConfirmed,
		},
		{
			event: "CHANNEL_PARK",
			headers: map[string]string{
				"Unique-ID":  uuid,
				"Event-Name": "CHANNEL_PARK",
			},
			finalState: esl.CallStateEstablished,
		},
		{
			event: "CHANNEL_HANGUP_COMPLETE",
			headers: map[string]string{
				"Unique-ID":    uuid,
				"Event-Name":   "CHANNEL_HANGUP_COMPLETE",
				"Hangup-Cause": "NORMAL_CLEARING",
			},
			finalState: esl.CallStateTerminated,
		},
	}

	for _, step := range lifecycle {
		event := &esl.Event{
			Headers:    step.headers,
			ReceivedAt: time.Now(),
		}

		err := machine.HandleEvent(event)
		if err != nil {
			t.Errorf("Error handling %s: %v", step.event, err)
		}

		state, exists := machine.GetCallState(uuid)
		if !exists {
			t.Errorf("Call should exist after %s", step.event)
		}

		if state != step.finalState {
			t.Errorf("Expected state %s after %s, got %s", step.finalState, step.event, state)
		}
	}

	stats := machine.GetStats()
	if stats["total_calls"].(int) != 1 {
		t.Errorf("Expected 1 total call, got %d", stats["total_calls"].(int))
	}

	if stats["active_calls"].(int) != 0 {
		t.Errorf("Expected 0 active calls after hangup, got %d", stats["active_calls"].(int))
	}
}

func TestFullWorkflowIntegration(t *testing.T) {
	config := DefaultConfig()
	config.ESL.BufferSize = 100
	config.ESL.MaxRetries = 3

	server := NewServer(config)

	event := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":                 "workflow-test-uuid",
			"Event-Name":                "CHANNEL_CREATE",
			"Caller-Username":           "test-caller",
			"Caller-Destination-Number": "1234567890",
			"Call-Direction":            "inbound",
		},
		ReceivedAt: time.Now(),
	}

	err := server.stateMachine.HandleEvent(event)
	if err != nil {
		t.Errorf("Error handling event: %v", err)
	}

	err = server.buffer.Enqueue(event)
	if err != nil {
		t.Errorf("Error enqueuing event: %v", err)
	}

	server.monitor.RecordEvent("CHANNEL_CREATE")
	server.monitor.SetGauge("sip_active_calls", 1.0, nil)

	time.Sleep(200 * time.Millisecond)

	stats := server.GetStats()

	stateMachineStats := stats["state_machine"].(map[string]any)
	if stateMachineStats["total_calls"].(int) != 1 {
		t.Errorf("Expected 1 total call, got %d", stateMachineStats["total_calls"].(int))
	}

	bufferStats := stats["buffer"].(map[string]any)
	if bufferStats["current_size"].(int) != 1 {
		t.Errorf("Expected buffer size 1, got %d", bufferStats["current_size"].(int))
	}

	if server.stateMachine.ActiveCalls() != 1 {
		t.Errorf("Expected 1 active call, got %d", server.stateMachine.ActiveCalls())
	}
}
