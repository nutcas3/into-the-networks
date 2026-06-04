package esl

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/textproto"
	"strings"
	"time"
)

type EventReader struct {
	reader *bufio.Reader
}

func NewEventReader(conn io.Reader) *EventReader {
	return &EventReader{
		reader: bufio.NewReader(conn),
	}
}

func (er *EventReader) ReadEvent() (*Event, error) {
	headers, err := er.readHeaders()
	if err != nil {
		return nil, err
	}
	
	event := &Event{
		Headers:    headers,
		ReceivedAt: time.Now(),
	}
	
	if contentLength, ok := headers["Content-Length"]; ok {
		length := 0
		if _, err := fmt.Sscanf(contentLength, "%d", &length); err == nil && length > 0 {
			body := make([]byte, length)
			_, err := io.ReadFull(er.reader, body)
			if err != nil {
				return nil, fmt.Errorf("failed to read event body: %w", err)
			}
			event.Body = body
		}
	}
	
	return event, nil
}

func (er *EventReader) readHeaders() (map[string]string, error) {
	tp := textproto.NewReader(er.reader)
	
	headers, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}
	
	result := make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	
	return result, nil
}

func (er *EventReader) ReadCommandResponse() (string, error) {
	line, err := er.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(line), nil
}

func ParseEvent(data []byte) (*Event, error) {
	reader := bufio.NewReader(bytes.NewBuffer(data))
	tp := textproto.NewReader(reader)
	
	headers, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to parse headers: %w", err)
	}
	
	event := &Event{
		Headers:    make(map[string]string),
		ReceivedAt: time.Now(),
	}
	
	for k, v := range headers {
		if len(v) > 0 {
			event.Headers[k] = v[0]
		}
	}
	
	if contentLength, ok := event.Headers["Content-Length"]; ok {
		length := 0
		if _, err := fmt.Sscanf(contentLength, "%d", &length); err == nil && length > 0 {
			body := make([]byte, length)
			_, err := io.ReadFull(reader, body)
			if err == nil {
				event.Body = body
			}
		}
	}
	
	return event, nil
}
