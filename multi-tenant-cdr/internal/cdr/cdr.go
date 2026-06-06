package cdr

import (
	"time"
)

type CDR struct {
	CallUUID        string
	Caller          string
	Destination     string
	CallStartTime   time.Time
	CallEndTime     time.Time
	DurationSeconds int
	Disposition     string
	HangupCause     string
}

type Disposition string

const (
	DispositionAnswered Disposition = "answered"
	DispositionMissed   Disposition = "missed"
	DispositionFailed   Disposition = "failed"
)

func DetermineDisposition(hangupCause string) Disposition {
	switch hangupCause {
	case "NORMAL_CLEARING", "NORMAL_UNSPECIFIED":
		return DispositionAnswered
	case "NO_ANSWER", "USER_BUSY":
		return DispositionMissed
	default:
		return DispositionFailed
	}
}
