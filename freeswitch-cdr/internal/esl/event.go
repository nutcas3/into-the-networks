package esl

import (
	"bufio"
	"bytes"
	"io"
	"net/textproto"
	"strconv"
	"strings"
)

type Event struct {
	Headers map[string]string
	Body    []byte
}

type EventListener func(*Event)

func (e *Event) GetName() string {
	return e.GetHeader("Event-Name")
}

func (e *Event) GetHeader(key string) string {
	return e.Headers[key]
}

func (e *Event) HasHeader(key string) bool {
	_, ok := e.Headers[key]
	return ok
}

func parseEvent(data []byte) (*Event, error) {
	reader := bufio.NewReader(bytes.NewBuffer(data))
	header := textproto.NewReader(reader)

	headers, err := header.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}

	event := &Event{
		Headers: make(map[string]string),
	}

	for k, v := range headers {
		if len(v) > 0 {
			event.Headers[k] = v[0]
		}
	}

	if contentLength := event.GetHeader("Content-Length"); contentLength != "" {
		length, err := strconv.Atoi(contentLength)
		if err == nil && length > 0 {
			event.Body = make([]byte, length)
			_, err = io.ReadFull(reader, event.Body)
			if err != nil {
				return event, err
			}
		}
	}

	return event, nil
}

func (e *Event) String() string {
	var builder strings.Builder
	builder.WriteString(e.GetName() + "\n")
	for k, v := range e.Headers {
		builder.WriteString(k + ": " + v + "\n")
	}
	if len(e.Body) > 0 {
		builder.Write(e.Body)
	}
	return builder.String()
}
