package esl

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/textproto"
	"sync"
)

const (
	EventListenAll      = "ALL"
	EndOfMessage        = "\r\n\r\n"
	TypeReply           = "command/reply"
	TypeAPIResponse     = "api/response"
	TypeEventPlain      = "text/event-plain"
	TypeEventXML        = "text/event-xml"
	TypeEventJSON       = "text/event-json"
	TypeAuthRequest     = "auth/request"
	TypeDisconnect      = "text/disconnect-notice"
	ContentTypeHeader   = "Content-Type"
	ContentLengthHeader = "Content-Length"
)

type Conn struct {
	conn              net.Conn
	reader            *bufio.Reader
	header            *textproto.Reader
	writeLock         sync.Mutex
	ctx               context.Context
	cancel            context.CancelFunc
	eventListeners    map[string]map[string]EventListener
	eventListenerLock sync.RWMutex
	responseChan      chan *Response
	closeOnce         sync.Once
}

type Response struct {
	Headers map[string]string
	Body    []byte
}

type Options struct {
	Password string
}

func Dial(address string, opts Options) (*Conn, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	reader := bufio.NewReader(conn)

	eslConn := &Conn{
		conn:           conn,
		reader:         reader,
		header:         textproto.NewReader(reader),
		ctx:            ctx,
		cancel:         cancel,
		eventListeners: make(map[string]map[string]EventListener),
		responseChan:   make(chan *Response, 100),
	}

	// Authenticate
	if err := eslConn.authenticate(opts.Password); err != nil {
		conn.Close()
		cancel()
		return nil, err
	}

	// Start receive loop
	go eslConn.receiveLoop()
	go eslConn.eventLoop()

	return eslConn, nil
}

func (c *Conn) authenticate(password string) error {
	authCmd := AuthCommand{Password: password}
	if err := c.SendCommand(authCmd); err != nil {
		return err
	}

	response, err := c.ReadResponse()
	if err != nil {
		return err
	}

	if response.Headers["Reply-Text"] != "+OK" {
		return fmt.Errorf("authentication failed: %s", response.Headers["Reply-Text"])
	}

	return nil
}

func (c *Conn) SendCommand(cmd Command) error {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	_, err := c.conn.Write([]byte(cmd.BuildMessage() + EndOfMessage))
	return err
}

func (c *Conn) SendCommandWithContext(ctx context.Context, cmd Command) (*Response, error) {
	if err := c.SendCommand(cmd); err != nil {
		return nil, err
	}

	select {
	case response := <-c.responseChan:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *Conn) ReadResponse() (*Response, error) {
	select {
	case response := <-c.responseChan:
		return response, nil
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

func (c *Conn) RegisterEventListener(uuid string, listener EventListener) string {
	c.eventListenerLock.Lock()
	defer c.eventListenerLock.Unlock()

	listenerID := fmt.Sprintf("%d", len(c.eventListeners)+1)
	if _, ok := c.eventListeners[uuid]; !ok {
		c.eventListeners[uuid] = make(map[string]EventListener)
	}
	c.eventListeners[uuid][listenerID] = listener
	return listenerID
}

func (c *Conn) RemoveEventListener(uuid, listenerID string) {
	c.eventListenerLock.Lock()
	defer c.eventListenerLock.Unlock()

	if listeners, ok := c.eventListeners[uuid]; ok {
		delete(listeners, listenerID)
	}
}

func (c *Conn) EnableEvents(format string, events []string) error {
	cmd := EventCommand{Format: format, Events: events}
	return c.SendCommand(cmd)
}

func (c *Conn) Close() error {
	c.closeOnce.Do(func() {
		c.cancel()
		c.conn.Close()
	})
	return nil
}

func (c *Conn) ExitAndClose() error {
	c.SendCommand(ExitCommand{})
	return c.Close()
}

func (c *Conn) receiveLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			response, err := c.readResponse()
			if err != nil {
				return
			}

			contentType := response.Headers[ContentTypeHeader]
			if contentType == TypeEventPlain || contentType == TypeEventXML || contentType == TypeEventJSON {
				// Events are handled in eventLoop
				continue
			}

			select {
			case c.responseChan <- response:
			case <-c.ctx.Done():
				return
			}
		}
	}
}

func (c *Conn) eventLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			response, err := c.readResponse()
			if err != nil {
				return
			}

			contentType := response.Headers[ContentTypeHeader]
			if contentType != TypeEventPlain && contentType != TypeEventXML && contentType != TypeEventJSON {
				continue
			}

			event, err := parseEvent(response.Body)
			if err != nil {
				continue
			}

			c.dispatchEvent(event)
		}
	}
}

func (c *Conn) readResponse() (*Response, error) {
	headers, err := c.header.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}

	response := &Response{
		Headers: make(map[string]string),
	}

	for k, v := range headers {
		if len(v) > 0 {
			response.Headers[k] = v[0]
		}
	}

	if contentLength := response.Headers[ContentLengthHeader]; contentLength != "" {
		length, err := parseContentLength(contentLength)
		if err == nil && length > 0 {
			body := make([]byte, length)
			_, err = c.reader.Read(body)
			if err != nil {
				return response, err
			}
			response.Body = body
		}
	}

	return response, nil
}

func parseContentLength(s string) (int, error) {
	var length int
	_, err := fmt.Sscanf(s, "%d", &length)
	return length, err
}

func (c *Conn) dispatchEvent(event *Event) {
	c.eventListenerLock.RLock()
	defer c.eventListenerLock.RUnlock()

	// Call listeners for ALL events
	if listeners, ok := c.eventListeners[EventListenAll]; ok {
		for _, listener := range listeners {
			go listener(event)
		}
	}

	// Call listeners for specific UUID
	if event.HasHeader("Unique-Id") {
		uuid := event.GetHeader("Unique-Id")
		if listeners, ok := c.eventListeners[uuid]; ok {
			for _, listener := range listeners {
				go listener(event)
			}
		}
	}

	// Call listeners for Application-UUID
	if event.HasHeader("Application-UUID") {
		appUUID := event.GetHeader("Application-UUID")
		if listeners, ok := c.eventListeners[appUUID]; ok {
			for _, listener := range listeners {
				go listener(event)
			}
		}
	}

	// Call listeners for Job-UUID
	if event.HasHeader("Job-UUID") {
		jobUUID := event.GetHeader("Job-UUID")
		if listeners, ok := c.eventListeners[jobUUID]; ok {
			for _, listener := range listeners {
				go listener(event)
			}
		}
	}
}
