package state

import (
	"sync"
	"time"

	"github.com/nutcas3/esl-resilience/internal/esl"
	"github.com/sirupsen/logrus"
)

type CallInfo struct {
	UUID          string
	CurrentState  esl.CallState
	PreviousState esl.CallState
	Caller        string
	Callee        string
	Direction     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	AnsweredAt    *time.Time
	HangupCause   string
	Duration      time.Duration
	ChannelData   map[string]string
}

type StateTransition struct {
	From  esl.CallState
	To    esl.CallState
	Event string
}

type Machine struct {
	calls       map[string]*CallInfo
	transitions map[esl.CallState]map[esl.CallState]bool
	mu          sync.RWMutex
	logger      *logrus.Logger
}

func NewMachine() *Machine {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	m := &Machine{
		calls:       make(map[string]*CallInfo),
		transitions: make(map[esl.CallState]map[esl.CallState]bool),
		logger:      logger,
	}

	m.initializeTransitions()
	return m
}

func (m *Machine) initializeTransitions() {
	validTransitions := []StateTransition{
		{From: esl.CallStateInit, To: esl.CallStateEarly, Event: "CHANNEL_PROGRESS"},
		{From: esl.CallStateInit, To: esl.CallStateConfirmed, Event: "CHANNEL_ANSWER"},
		{From: esl.CallStateInit, To: esl.CallStateTerminated, Event: "CHANNEL_HANGUP_COMPLETE"},
		{From: esl.CallStateInit, To: esl.CallStateFailed, Event: "CHANNEL_EXECUTE_COMPLETE"},

		{From: esl.CallStateEarly, To: esl.CallStateConfirmed, Event: "CHANNEL_ANSWER"},
		{From: esl.CallStateEarly, To: esl.CallStateTerminated, Event: "CHANNEL_HANGUP_COMPLETE"},
		{From: esl.CallStateEarly, To: esl.CallStateFailed, Event: "CHANNEL_EXECUTE_COMPLETE"},

		{From: esl.CallStateConfirmed, To: esl.CallStateEstablished, Event: "CHANNEL_PARK"},
		{From: esl.CallStateConfirmed, To: esl.CallStateTerminated, Event: "CHANNEL_HANGUP_COMPLETE"},
		{From: esl.CallStateConfirmed, To: esl.CallStateFailed, Event: "CHANNEL_EXECUTE_COMPLETE"},

		{From: esl.CallStateEstablished, To: esl.CallStateTerminated, Event: "CHANNEL_HANGUP_COMPLETE"},
		{From: esl.CallStateEstablished, To: esl.CallStateFailed, Event: "CHANNEL_EXECUTE_COMPLETE"},

		{From: esl.CallStateTerminated, To: esl.CallStateInit, Event: "CHANNEL_CREATE"}, // New call on same UUID (rare)
		{From: esl.CallStateFailed, To: esl.CallStateInit, Event: "CHANNEL_CREATE"},     // Retry call
	}

	for _, transition := range validTransitions {
		if m.transitions[transition.From] == nil {
			m.transitions[transition.From] = make(map[esl.CallState]bool)
		}
		m.transitions[transition.From][transition.To] = true
	}
}
