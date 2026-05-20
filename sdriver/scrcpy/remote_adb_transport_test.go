package scrcpy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"webscreen/sdriver"
)

func TestRemoteADBTransportOpenLocalAbstractUsesRawADBProtocol(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	services := make(chan string, 2)
	payload := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for i := 0; i < 2; i++ {
			service, err := readFakeADBService(reader)
			if err != nil {
				return
			}
			services <- service
			_, _ = conn.Write([]byte("OKAY"))
		}
		buf := make([]byte, 5)
		_, _ = io.ReadFull(reader, buf)
		payload <- buf
	}()

	addr := ln.Addr().(*net.TCPAddr)
	transport, err := NewRemoteADBTransport(sdriver.BridgeID("mac-coral"), addr.IP.String(), addr.Port, "/fake/adb")
	if err != nil {
		t.Fatalf("NewRemoteADBTransport returned error: %v", err)
	}
	conn, err := transport.OpenLocalAbstract(context.Background(), "5f9e7947", "scrcpy_AB12CD34")
	if err != nil {
		t.Fatalf("OpenLocalAbstract returned error: %v", err)
	}
	_, _ = conn.Write([]byte("hello"))
	_ = conn.Close()

	gotTransport := <-services
	gotSocket := <-services
	if gotTransport != "host:transport:5f9e7947" {
		t.Fatalf("transport service = %q", gotTransport)
	}
	if gotSocket != "localabstract:scrcpy_AB12CD34" {
		t.Fatalf("socket service = %q", gotSocket)
	}
	if string(<-payload) != "hello" {
		t.Fatalf("payload was not forwarded")
	}
}

func readFakeADBService(r *bufio.Reader) (string, error) {
	sizeBytes := make([]byte, 4)
	if _, err := io.ReadFull(r, sizeBytes); err != nil {
		return "", err
	}
	size64, err := strconv.ParseInt(string(sizeBytes), 16, 32)
	if err != nil {
		return "", err
	}
	if size64 <= 0 {
		return "", fmt.Errorf("invalid service size %d", size64)
	}
	payload := make([]byte, int(size64))
	if _, err := io.ReadFull(r, payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(string(payload)), nil
}
