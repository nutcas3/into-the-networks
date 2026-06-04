package state

import (
	"fmt"
	"testing"
	"time"

	"github.com/nutcas3/esl-resilience/internal/esl"
)

func TestNewMachine(t *testing.T) {
	machine := NewMachine()

	if machine.ActiveCalls() != 0 {
		t.Errorf("Expected 0 active calls, got %d", machine.ActiveCalls())
	}

	stats := machine.GetStats()
	if stats["total_calls"].(int) != 0 {
		t.Errorf("Expected 0 total calls, got %d", stats["total_calls"].(int))
	}
}

func TestStateMachineValidTransitions(t *testing.T) {
	machine := NewMachine()

	uuid := "test-uuid-123"
	event := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":                 uuid,
			"Event-Name":                "CHANNEL_CREATE",
			"Caller-Username":           "test-caller",
			"Caller-Destination-Number": "1234567890",
			"Call-Direction":            "inbound",
		},
		ReceivedAt: time.Now(),
	}

	err := machine.HandleEvent(event)
	if err != nil {
		t.Errorf("Unexpected error handling CHANNEL_CREATE: %v", err)
	}

	state, exists := machine.GetCallState(uuid)
	if !exists {
		t.Error("Expected call to exist")
	}

	if state != esl.CallStateInit {
		t.Errorf("Expected CallStateInit, got %s", state)
	}

	if machine.ActiveCalls() != 1 {
		t.Errorf("Expected 1 active call, got %d", machine.ActiveCalls())
	}
}

func TestStateMachineInvalidTransition(t *testing.T) {
	machine := NewMachine()

	uuid := "test-uuid-456"

	progressEvent := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":  uuid,
			"Event-Name": "CHANNEL_PROGRESS",
		},
		ReceivedAt: time.Now(),
	}

	err := machine.HandleEvent(progressEvent)
	if err == nil {
		t.Error("Expected error for invalid transition")
	}

	_, exists := machine.GetCallState(uuid)
	if exists {
		t.Error("Call should not exist for invalid transition")
	}
}

func TestStateMachineCallLifecycle(t *testing.T) {
	machine := NewMachine()

	uuid := "test-uuid-lifecycle"

	createEvent := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":                 uuid,
			"Event-Name":                "CHANNEL_CREATE",
			"Caller-Username":           "test-caller",
			"Caller-Destination-Number": "1234567890",
			"Call-Direction":            "inbound",
		},
		ReceivedAt: time.Now(),
	}

	err := machine.HandleEvent(createEvent)
	if err != nil {
		t.Errorf("Error creating call: %v", err)
	}

	progressEvent := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":  uuid,
			"Event-Name": "CHANNEL_PROGRESS",
		},
		ReceivedAt: time.Now(),
	}

	err = machine.HandleEvent(progressEvent)
	if err != nil {
		t.Errorf("Error handling progress: %v", err)
	}

	state, _ := machine.GetCallState(uuid)
	if state != esl.CallStateEarly {
		t.Errorf("Expected CallStateEarly, got %s", state)
	}

	answerEvent := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":  uuid,
			"Event-Name": "CHANNEL_ANSWER",
		},
		ReceivedAt: time.Now(),
	}

	err = machine.HandleEvent(answerEvent)
	if err != nil {
		t.Errorf("Error handling answer: %v", err)
	}

	state, _ = machine.GetCallState(uuid)
	if state != esl.CallStateConfirmed {
		t.Errorf("Expected CallStateConfirmed, got %s", state)
	}

	parkEvent := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":  uuid,
			"Event-Name": "CHANNEL_PARK",
		},
		ReceivedAt: time.Now(),
	}

	err = machine.HandleEvent(parkEvent)
	if err != nil {
		t.Errorf("Error handling park: %v", err)
	}

	state, _ = machine.GetCallState(uuid)
	if state != esl.CallStateEstablished {
		t.Errorf("Expected CallStateEstablished, got %s", state)
	}

	hangupEvent := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":    uuid,
			"Event-Name":   "CHANNEL_HANGUP_COMPLETE",
			"Hangup-Cause": "NORMAL_CLEARING",
		},
		ReceivedAt: time.Now(),
	}

	err = machine.HandleEvent(hangupEvent)
	if err != nil {
		t.Errorf("Error handling hangup: %v", err)
	}

	state, _ = machine.GetCallState(uuid)
	if state != esl.CallStateTerminated {
		t.Errorf("Expected CallStateTerminated, got %s", state)
	}

	if machine.ActiveCalls() != 0 {
		t.Errorf("Expected 0 active calls after hangup, got %d", machine.ActiveCalls())
	}
}

func TestStateMachineFailedCall(t *testing.T) {
	machine := NewMachine()

	uuid := "test-uuid-failed"

	createEvent := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":                 uuid,
			"Event-Name":                "CHANNEL_CREATE",
			"Caller-Username":           "test-caller",
			"Caller-Destination-Number": "1234567890",
			"Call-Direction":            "inbound",
		},
		ReceivedAt: time.Now(),
	}

	err := machine.HandleEvent(createEvent)
	if err != nil {
		t.Errorf("Error creating call: %v", err)
	}

	failEvent := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":    uuid,
			"Event-Name":   "CHANNEL_EXECUTE_COMPLETE",
			"Hangup-Cause": "USER_BUSY",
		},
		ReceivedAt: time.Now(),
	}

	err = machine.HandleEvent(failEvent)
	if err != nil {
		t.Errorf("Error handling failure: %v", err)
	}

	state, _ := machine.GetCallState(uuid)
	if state != esl.CallStateFailed {
		t.Errorf("Expected CallStateFailed, got %s", state)
	}

	if machine.ActiveCalls() != 0 {
		t.Errorf("Expected 0 active calls after failure, got %d", machine.ActiveCalls())
	}
}

func TestStateMachineGetCallInfo(t *testing.T) {
	machine := NewMachine()

	uuid := "test-uuid-info"

	event := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":                 uuid,
			"Event-Name":                "CHANNEL_CREATE",
			"Caller-Username":           "test-caller",
			"Caller-Destination-Number": "1234567890",
			"Call-Direction":            "inbound",
			"Channel-Name":              "test-channel",
		},
		ReceivedAt: time.Now(),
	}

	err := machine.HandleEvent(event)
	if err != nil {
		t.Errorf("Error creating call: %v", err)
	}

	callInfo, exists := machine.GetCallInfo(uuid)
	if !exists {
		t.Error("Expected call info to exist")
	}

	if callInfo.UUID != uuid {
		t.Errorf("Expected UUID %s, got %s", uuid, callInfo.UUID)
	}

	if callInfo.Caller != "test-caller" {
		t.Errorf("Expected caller test-caller, got %s", callInfo.Caller)
	}

	if callInfo.Callee != "1234567890" {
		t.Errorf("Expected callee 1234567890, got %s", callInfo.Callee)
	}

	if callInfo.Direction != "inbound" {
		t.Errorf("Expected direction inbound, got %s", callInfo.Direction)
	}

	if callInfo.CurrentState != esl.CallStateInit {
		t.Errorf("Expected CallStateInit, got %s", callInfo.CurrentState)
	}

	if callInfo.ChannelData["Channel-Name"] != "test-channel" {
		t.Errorf("Expected channel name test-channel, got %s", callInfo.ChannelData["Channel-Name"])
	}
}

func TestStateMachineGetAllCalls(t *testing.T) {
	machine := NewMachine()

	uuids := []string{"uuid1", "uuid2", "uuid3"}

	for _, uuid := range uuids {
		event := &esl.Event{
			Headers: map[string]string{
				"Unique-ID":                 uuid,
				"Event-Name":                "CHANNEL_CREATE",
				"Caller-Username":           "test-caller",
				"Caller-Destination-Number": "1234567890",
				"Call-Direction":            "inbound",
			},
			ReceivedAt: time.Now(),
		}

		err := machine.HandleEvent(event)
		if err != nil {
			t.Errorf("Error creating call %s: %v", uuid, err)
		}
	}

	allCalls := machine.GetAllCalls()
	if len(allCalls) != 3 {
		t.Errorf("Expected 3 calls, got %d", len(allCalls))
	}

	for _, uuid := range uuids {
		if _, exists := allCalls[uuid]; !exists {
			t.Errorf("Expected call %s to exist in all calls", uuid)
		}
	}
}

func TestStateMachineGetCallsByState(t *testing.T) {
	machine := NewMachine()

	uuid1 := "uuid-early"
	uuid2 := "uuid-confirmed"

	createEvent1 := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":  uuid1,
			"Event-Name": "CHANNEL_CREATE",
		},
		ReceivedAt: time.Now(),
	}

	createEvent2 := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":  uuid2,
			"Event-Name": "CHANNEL_CREATE",
		},
		ReceivedAt: time.Now(),
	}

	machine.HandleEvent(createEvent1)
	machine.HandleEvent(createEvent2)

	progressEvent := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":  uuid1,
			"Event-Name": "CHANNEL_PROGRESS",
		},
		ReceivedAt: time.Now(),
	}

	machine.HandleEvent(progressEvent)

	answerEvent := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":  uuid2,
			"Event-Name": "CHANNEL_ANSWER",
		},
		ReceivedAt: time.Now(),
	}

	machine.HandleEvent(answerEvent)

	earlyCalls := machine.GetCallsByState(esl.CallStateEarly)
	if len(earlyCalls) != 1 {
		t.Errorf("Expected 1 early call, got %d", len(earlyCalls))
	}

	if earlyCalls[0].UUID != uuid1 {
		t.Errorf("Expected UUID %s, got %s", uuid1, earlyCalls[0].UUID)
	}

	confirmedCalls := machine.GetCallsByState(esl.CallStateConfirmed)
	if len(confirmedCalls) != 1 {
		t.Errorf("Expected 1 confirmed call, got %d", len(confirmedCalls))
	}

	if confirmedCalls[0].UUID != uuid2 {
		t.Errorf("Expected UUID %s, got %s", uuid2, confirmedCalls[0].UUID)
	}
}

func TestStateMachineStats(t *testing.T) {
	machine := NewMachine()

	uuid1 := "uuid-stats-1"
	uuid2 := "uuid-stats-2"

	createEvent1 := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":  uuid1,
			"Event-Name": "CHANNEL_CREATE",
		},
		ReceivedAt: time.Now(),
	}

	createEvent2 := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":  uuid2,
			"Event-Name": "CHANNEL_CREATE",
		},
		ReceivedAt: time.Now(),
	}

	machine.HandleEvent(createEvent1)
	machine.HandleEvent(createEvent2)

	progressEvent := &esl.Event{
		Headers: map[string]string{
			"Unique-ID":  uuid1,
			"Event-Name": "CHANNEL_PROGRESS",
		},
		ReceivedAt: time.Now(),
	}

	machine.HandleEvent(progressEvent)

	stats := machine.GetStats()

	if stats["total_calls"].(int) != 2 {
		t.Errorf("Expected 2 total calls, got %d", stats["total_calls"].(int))
	}

	if stats["active_calls"].(int) != 2 {
		t.Errorf("Expected 2 active calls, got %d", stats["active_calls"].(int))
	}

	callsByState := stats["calls_by_state"].(map[esl.CallState]int)
	if callsByState[esl.CallStateInit] != 1 {
		t.Errorf("Expected 1 call in init state, got %d", callsByState[esl.CallStateInit])
	}

	if callsByState[esl.CallStateEarly] != 1 {
		t.Errorf("Expected 1 call in early state, got %d", callsByState[esl.CallStateEarly])
	}
}

func TestStateMachineConcurrency(t *testing.T) {
	machine := NewMachine()

	done := make(chan bool, 10)

	for i := range 10 {
		go func(id int) {
			uuid := fmt.Sprintf("uuid-concurrent-%d", id)
			event := &esl.Event{
				Headers: map[string]string{
					"Unique-ID":  uuid,
					"Event-Name": "CHANNEL_CREATE",
				},
				ReceivedAt: time.Now(),
			}

			err := machine.HandleEvent(event)
			if err != nil {
				t.Errorf("Error handling concurrent event %d: %v", id, err)
			}

			done <- true
		}(i)
	}

	for range 10 {
		<-done
	}

	if machine.ActiveCalls() != 10 {
		t.Errorf("Expected 10 active calls, got %d", machine.ActiveCalls())
	}

	allCalls := machine.GetAllCalls()
	if len(allCalls) != 10 {
		t.Errorf("Expected 10 total calls, got %d", len(allCalls))
	}
}
