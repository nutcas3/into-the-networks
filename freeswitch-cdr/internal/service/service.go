package service

import (
	"freeswitch-cdr/internal/cdr"
	"freeswitch-cdr/internal/esl"
	"log"
	"strconv"
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
	listenerID := s.eslConn.RegisterEventListener(esl.EventListenAll, s.eventHandler)
	defer s.eslConn.RemoveEventListener(esl.EventListenAll, listenerID)

	if err := s.eslConn.EnableEvents("plain", []string{"CHANNEL_HANGUP_COMPLETE"}); err != nil {
		return err
	}

	log.Println("Service started. Listening for call events...")

	select {}
}

func (s *Service) handleEvent(event *esl.Event) {
	eventName := event.GetHeader("Event-Name")

	if eventName == "CHANNEL_HANGUP_COMPLETE" {
		log.Printf("Received CHANNEL_HANGUP_COMPLETE for UUID=%s", event.GetHeader("Unique-ID"))

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
	caller := firstNonEmpty(
		event.GetHeader("Caller-Username"),
		event.GetHeader("Caller-Caller-ID-Number"),
		event.GetHeader("variable_sip_from_user"),
		"unknown",
	)
	destination := firstNonEmpty(
		event.GetHeader("Caller-Destination-Number"),
		event.GetHeader("variable_sip_req_user"),
		event.GetHeader("variable_dialed_user"),
		"unknown",
	)
	hangupCause := event.GetHeader("Hangup-Cause")

	callStartTimeStr := event.GetHeader("variable_start_epoch")
	callEndTimeStr := event.GetHeader("variable_end_epoch")

	callStartTime := time.Now()
	callEndTime := time.Now()
	duration := 0

	if parsed, ok := parseFreeSWITCHTime(callStartTimeStr); ok {
		callStartTime = parsed
	}

	if parsed, ok := parseFreeSWITCHTime(callEndTimeStr); ok {
		callEndTime = parsed
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

func parseFreeSWITCHTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}

	if epoch, err := strconv.ParseInt(value, 10, 64); err == nil {
		// Detect microsecond epochs (values > 1e12 indicate microseconds)
		if epoch > 1e12 || epoch < -1e12 {
			return time.UnixMicro(epoch), true
		}
		return time.Unix(epoch, 0), true
	}

	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}

	return time.Time{}, false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
