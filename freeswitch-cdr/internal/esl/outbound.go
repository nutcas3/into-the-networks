package esl

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/textproto"
)

type OutboundHandler func(ctx context.Context, conn *Conn, response *Response)

type OutboundOptions struct {
	Address string
}

func ListenAndServe(address string, handler OutboundHandler) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()

	fmt.Printf("Outbound ESL server listening on %s\n", address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		go handleOutboundConnection(conn, handler)
	}
}

func handleOutboundConnection(netConn net.Conn, handler OutboundHandler) {
	reader := bufio.NewReader(netConn)
	header := textproto.NewReader(reader)

	ctx, cancel := context.WithCancel(context.Background())

	eslConn := &Conn{
		conn:           netConn,
		reader:         reader,
		header:         header,
		ctx:            ctx,
		cancel:         cancel,
		eventListeners: make(map[string]map[string]EventListener),
		responseChan:   make(chan *Response, 100),
	}

	response, err := eslConn.readResponse()
	if err != nil {
		netConn.Close()
		cancel()
		return
	}

	go eslConn.receiveLoop()

	handler(ctx, eslConn, response)

	eslConn.Close()
}
