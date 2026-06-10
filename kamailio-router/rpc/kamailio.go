package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

type KamailioClient struct {
	host string
	port int
	conn net.Conn
}

func NewKamailioClient(host string, port int) *KamailioClient {
	return &KamailioClient{
		host: host,
		port: port,
	}
}

func (k *KamailioClient) Connect() error {
	address := net.JoinHostPort(k.host, fmt.Sprintf("%d", k.port))
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
	k.conn = conn
	return nil
}

func (k *KamailioClient) Close() error {
	if k.conn != nil {
		return k.conn.Close()
	}
	return nil
}

func (k *KamailioClient) SendCommand(cmd string) (string, error) {
	if k.conn == nil {
		if err := k.Connect(); err != nil {
			return "", err
		}
		defer k.Close()
	}

	_, err := k.conn.Write([]byte(cmd + "\n"))
	if err != nil {
		return "", err
	}

	reader := bufio.NewReader(k.conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response), nil
}

func (k *KamailioClient) DispatcherAdd(setid int, destination string) error {
	cmd := fmt.Sprintf("dispatcher.add %d %s", setid, destination)
	_, err := k.SendCommand(cmd)
	return err
}

func (k *KamailioClient) DispatcherRemove(setid int, destination string) error {
	cmd := fmt.Sprintf("dispatcher.remove %d %s", setid, destination)
	_, err := k.SendCommand(cmd)
	return err
}

func (k *KamailioClient) DispatcherList() (string, error) {
	return k.SendCommand("dispatcher.list")
}

func (k *KamailioClient) DispatcherReload() error {
	_, err := k.SendCommand("dispatcher.reload")
	return err
}

func (k *KamailioClient) HealthCheck(destination string) (bool, time.Duration, error) {
	start := time.Now()

	// Send OPTIONS request to destination
	cmd := fmt.Sprintf("t_uac_dlg OPTIONS \"%s\" \".\" \".\" \".\"", destination)
	response, err := k.SendCommand(cmd)

	duration := time.Since(start)

	if err != nil {
		return false, duration, err
	}

	// Check if response indicates success
	isHealthy := strings.Contains(response, "200") || strings.Contains(response, "OK")

	return isHealthy, duration, nil
}
