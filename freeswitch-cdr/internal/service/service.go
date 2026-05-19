package service

import (
	"freeswitch-cdr/internal/cdr"
	"freeswitch-cdr/internal/esl"
	"log"
	"time"
)

type Service struct {
	eslConn      *esl.Conn
	cdrRepo      *cdr.Repository
	eventHandler esl.EventListener
}

type Config struct {
	FreeSWITCHAddress  string
	FreeSWITCHPassword string
}

func New(eslConn *esl.Conn, cdrRepo *cdr.Repository) *Service {
	svc := &Service{
		eslConn: eslConn,
		cdrRepo: cdrRepo,
	}
	svc.eventHandler = svc.handleEvent
	return svc
}

func (s *Service) Start() error {
	if err := s.eslConn.EnableEvents("plain", []string{"CHANNEL_HANGUP_COMPLETE"}); err != nil {
		log.Printf("Warning: Could not enable events: %v", err)
	}

	listenerID := s.eslConn.RegisterEventListener(esl.EventListenAll, s.eventHandler)
	defer s.eslConn.RemoveEventListener(esl.EventListenAll, listenerID)

	log.Println("Service started. Listening for call events...")

	select {}
}

func (s *Service) handleEvent(event *esl.Event) {
	eventName := event.GetHeader("Event-Name")

	if eventName == "CHANNEL_HANGUP_COMPLETE" {
		callRecord := s.extractCDR(event)
		if err := s.cdrRepo.Save(callRecord); err != nil {
			log.Printf("Error saving CDR: %v", err)
		} else {
			log.Printf("CDR saved: Call UUID=%s, Caller=%s -> Destination=%s, Duration=%ds, Disposition=%s",
				callRecord.CallUUID, callRecord.Caller, callRecord.Destination, callRecord.DurationSeconds, callRecord.Disposition)
		}
	}
}

func (s *Service) extractCDR(event *esl.Event) *cdr.CDR {
	callUUID := event.GetHeader("Unique-ID")
	caller := event.GetHeader("Caller-Username")
	destination := event.GetHeader("Caller-Destination-Number")
	hangupCause := event.GetHeader("Hangup-Cause")

	callStartTimeStr := event.GetHeader("variable_start_epoch")
	callEndTimeStr := event.GetHeader("variable_end_epoch")

	callStartTime := time.Now()
	callEndTime := time.Now()
	duration := 0

	if callStartTimeStr != "" {
		if epoch, err := time.Parse("2006-01-02 15:04:05", callStartTimeStr); err == nil {
			callStartTime = epoch
		} else if epoch, err := time.Parse(time.RFC3339, callStartTimeStr); err == nil {
			callStartTime = epoch
		}
	}

	if callEndTimeStr != "" {
		if epoch, err := time.Parse("2006-01-02 15:04:05", callEndTimeStr); err == nil {
			callEndTime = epoch
		} else if epoch, err := time.Parse(time.RFC3339, callEndTimeStr); err == nil {
			callEndTime = epoch
		}
	}

	duration = max(int(callEndTime.Sub(callStartTime).Seconds()), 0)

	return &cdr.CDR{
		CallUUID:        callUUID,
		Caller:          caller,
		Destination:     destination,
		CallStartTime:   callStartTime,
		CallEndTime:     callEndTime,
		DurationSeconds: duration,
		Disposition:     string(cdr.DetermineDisposition(hangupCause)),
		HangupCause:     hangupCause,
	}
}
