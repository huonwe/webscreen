package scrcpy

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func remoteADBEndpoint() (string, int, bool) {
	if os.Getenv("WEBSCREEN_REMOTE_ADB") != "1" {
		return "", 0, false
	}

	host := os.Getenv("WEBSCREEN_REMOTE_ADB_HOST")
	if host == "" {
		host = os.Getenv("ANDROID_ADB_SERVER_ADDRESS")
	}

	portText := os.Getenv("WEBSCREEN_REMOTE_ADB_PORT")
	if portText == "" {
		portText = os.Getenv("ANDROID_ADB_SERVER_PORT")
	}
	if portText == "" {
		portText = "5037"
	}

	port, err := strconv.Atoi(portText)
	if host == "" || err != nil || port <= 0 {
		return "", 0, false
	}
	return host, port, true
}

func adbService(conn net.Conn, service string) error {
	payload := []byte(service)
	if _, err := fmt.Fprintf(conn, "%04x%s", len(payload), payload); err != nil {
		return err
	}

	status := make([]byte, 4)
	if _, err := io.ReadFull(conn, status); err != nil {
		return err
	}
	switch string(status) {
	case "OKAY":
		return nil
	case "FAIL":
		sizeBuf := make([]byte, 4)
		if _, err := io.ReadFull(conn, sizeBuf); err != nil {
			return err
		}
		size64, err := strconv.ParseInt(string(sizeBuf), 16, 32)
		if err != nil {
			return err
		}
		message := make([]byte, int(size64))
		if _, err := io.ReadFull(conn, message); err != nil {
			return err
		}
		return fmt.Errorf("adb service %q failed: %s", service, strings.TrimSpace(string(message)))
	default:
		return fmt.Errorf("adb service %q returned unexpected status %q", service, status)
	}
}

func (c *ADBClient) ConnectLocalAbstract(socketName string) (net.Conn, error) {
	if c.transportErr != nil {
		return nil, c.transportErr
	}
	if c.transport == nil {
		return nil, fmt.Errorf("adb transport is not configured")
	}
	if c.deviceSerial == "" {
		return nil, fmt.Errorf("device serial is required for remote adb direct socket")
	}
	return c.transport.OpenLocalAbstract(c.ctx, c.deviceSerial, socketName)
}

func (c *ADBClient) ConnectLocalAbstractWithRetry(socketName string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := c.ConnectLocalAbstract(socketName)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	return nil, lastErr
}
