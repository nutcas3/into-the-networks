package state

import (
	"fmt"
	"time"

	"github.com/nutcas3/esl-resilience/internal/esl"
	"github.com/sirupsen/logrus"
)

func (m *Machine) HandleEvent(event *esl.Event) error {
	uuid := event.Headers["Unique-ID"]
	if uuid == "" {
		return fmt.Errorf("event missing Unique-ID header")
	}

	eventType := event.Headers["Event-Name"]

	m.mu.Lock()
	defer m.mu.Unlock()

	call, exists := m.calls[uuid]
	if !exists {
		if eventType == "CHANNEL_CREATE" {
			call = m.createNewCall(event)
			m.calls[uuid] = call
			m.logger.WithFields(logrus.Fields{
				"uuid":      uuid,
				"caller":    call.Caller,
				"callee":    call.Callee,
				"direction": call.Direction,
			}).Info("New call created")
		} else {
			return fmt.Errorf("received event %s for unknown call UUID %s", eventType, uuid)
		}
	}

	return m.processTransition(call, eventType, event)
}

func (m *Machine) createNewCall(event *esl.Event) *CallInfo {
	now := time.Now()

	call := &CallInfo{
		UUID:          event.Headers["Unique-ID"],
		CurrentState:  esl.CallStateInit,
		PreviousState: esl.CallStateInit,
		Caller:        event.Headers["Caller-Username"],
		Callee:        event.Headers["Caller-Destination-Number"],
		Direction:     event.Headers["Call-Direction"],
		CreatedAt:     now,
		UpdatedAt:     now,
		ChannelData:   make(map[string]string),
	}

	relevantHeaders := []string{
		"Channel-Name", "Channel-Call-State", "Channel-Call-UUID",
		"Channel-Read-Codec-Name", "Channel-Write-Codec-Name",
		"variable_sip_from_user", "variable_sip_req_user",
		"variable_domain_name", "variable_effective_caller_id_name",
	}

	for _, header := range relevantHeaders {
		if value := event.Headers[header]; value != "" {
			call.ChannelData[header] = value
		}
	}

	return call
}

func (m *Machine) processTransition(call *CallInfo, eventType string, event *esl.Event) error {
	var newState esl.CallState
	var validTransition bool

	switch eventType {
	case "CHANNEL_PROGRESS":
		newState = esl.CallStateEarly
	case "CHANNEL_ANSWER":
		newState = esl.CallStateConfirmed
		call.AnsweredAt = &[]time.Time{time.Now()}[0]
	case "CHANNEL_PARK":
		if call.CurrentState == esl.CallStateConfirmed {
			newState = esl.CallStateEstablished
		} else {
			return fmt.Errorf("CHANNEL_PARK received in invalid state: %s", call.CurrentState)
		}
	case "CHANNEL_HANGUP_COMPLETE":
		newState = esl.CallStateTerminated
		call.HangupCause = event.Headers["Hangup-Cause"]
		if call.AnsweredAt != nil {
			call.Duration = time.Since(*call.AnsweredAt)
		}
	case "CHANNEL_EXECUTE_COMPLETE":
		newState = esl.CallStateFailed
		call.HangupCause = event.Headers["Hangup-Cause"]
	default:
		return nil
	}

	if m.transitions[call.CurrentState] != nil {
		validTransition = m.transitions[call.CurrentState][newState]
	}

	if !validTransition {
		return fmt.Errorf("invalid state transition from %s to %s for event %s",
			call.CurrentState, newState, eventType)
	}

	call.PreviousState = call.CurrentState
	call.CurrentState = newState
	call.UpdatedAt = time.Now()

	for key, value := range event.Headers {
		call.ChannelData[key] = value
	}

	m.logger.WithFields(logrus.Fields{
		"uuid":           call.UUID,
		"previous_state": call.PreviousState,
		"current_state":  call.CurrentState,
		"event":          eventType,
		"caller":         call.Caller,
		"callee":         call.Callee,
	}).Info("Call state transition")

	if newState == esl.CallStateTerminated || newState == esl.CallStateFailed {
		go m.cleanupCall(call.UUID)
	}

	return nil
}

func (m *Machine) cleanupCall(uuid string) {
	time.Sleep(5 * time.Minute) // Keep call data for 5 minutes

	m.mu.Lock()
	defer m.mu.Unlock()

	if call, exists := m.calls[uuid]; exists {
		if call.CurrentState == esl.CallStateTerminated || call.CurrentState == esl.CallStateFailed {
			delete(m.calls, uuid)
			m.logger.WithField("uuid", uuid).Info("Call cleaned up from memory")
		}
	}
}

func (m *Machine) GetCallState(uuid string) (esl.CallState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	call, exists := m.calls[uuid]
	if !exists {
		return "", false
	}

	return call.CurrentState, true
}

func (m *Machine) GetCallInfo(uuid string) (*CallInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	call, exists := m.calls[uuid]
	if !exists {
		return nil, false
	}

	copyCall := *call
	copyCall.ChannelData = make(map[string]string)
	for k, v := range call.ChannelData {
		copyCall.ChannelData[k] = v
	}

	return &copyCall, true
}

func (m *Machine) ActiveCalls() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, call := range m.calls {
		if call.CurrentState != esl.CallStateTerminated && call.CurrentState != esl.CallStateFailed {
			count++
		}
	}

	return count
}

func (m *Machine) GetAllCalls() map[string]*CallInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*CallInfo)
	for uuid, call := range m.calls {
		copyCall := *call
		copyCall.ChannelData = make(map[string]string)
		for k, v := range call.ChannelData {
			copyCall.ChannelData[k] = v
		}
		result[uuid] = &copyCall
	}

	return result
}

func (m *Machine) GetCallsByState(state esl.CallState) []*CallInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var calls []*CallInfo
	for _, call := range m.calls {
		if call.CurrentState == state {
			copyCall := *call
			copyCall.ChannelData = make(map[string]string)
			for k, v := range call.ChannelData {
				copyCall.ChannelData[k] = v
			}
			calls = append(calls, &copyCall)
		}
	}

	return calls
}

func (m *Machine) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_calls":    len(m.calls),
		"active_calls":   m.ActiveCalls(),
		"calls_by_state": make(map[esl.CallState]int),
	}

	for _, call := range m.calls {
		state := call.CurrentState
		if stats["calls_by_state"].(map[esl.CallState]int)[state] == 0 {
			stats["calls_by_state"].(map[esl.CallState]int)[state] = 0
		}
		stats["calls_by_state"].(map[esl.CallState]int)[state]++
	}

	return stats
}
