package scrcpy

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"webscreen/sdriver"
)

type RemoteADBTransport struct {
	BridgeID      sdriver.BridgeID
	Host          string
	Port          int
	ADBExecutable string
}

func NewRemoteADBTransport(bridgeID sdriver.BridgeID, host string, port int, adbExecutable string) (*RemoteADBTransport, error) {
	if strings.TrimSpace(adbExecutable) == "" {
		return nil, ErrADBExecutableRequired
	}
	if strings.TrimSpace(host) == "" || port <= 0 {
		return nil, fmt.Errorf("invalid remote adb endpoint")
	}
	if bridgeID == "" {
		bridgeID = "bootstrap-env"
	}
	return &RemoteADBTransport{
		BridgeID:      bridgeID,
		Host:          host,
		Port:          port,
		ADBExecutable: adbExecutable,
	}, nil
}

func (t *RemoteADBTransport) Devices(ctx context.Context) ([]DeviceDescriptor, error) {
	output, err := runADBCommand(ctx, t.ADBExecutable, t.prefixArgs("devices", "-l")...)
	if err != nil {
		return nil, err
	}
	return parseADBDevices(output, sdriver.TransportRemoteADB, t.BridgeID), nil
}

func (t *RemoteADBTransport) Shell(ctx context.Context, serial string, argv ...string) ([]byte, error) {
	args := t.prefixArgs()
	args = appendSerial(args, serial)
	args = append(args, "shell")
	args = append(args, argv...)
	return runADBCommand(ctx, t.ADBExecutable, args...)
}

func (t *RemoteADBTransport) Push(ctx context.Context, serial, src, dst string) error {
	args := t.prefixArgs()
	args = appendSerial(args, serial)
	args = append(args, "push", src, dst)
	_, err := runADBCommand(ctx, t.ADBExecutable, args...)
	return err
}

func (t *RemoteADBTransport) Reverse(ctx context.Context, serial, remote, local string) error {
	return ErrCapabilityNotSupported
}

func (t *RemoteADBTransport) ReverseRemove(ctx context.Context, serial, remote string) error {
	return ErrCapabilityNotSupported
}

func (t *RemoteADBTransport) OpenLocalAbstract(ctx context.Context, serial, socketName string) (net.Conn, error) {
	if serial == "" {
		return nil, fmt.Errorf("device serial is required for remote adb direct socket")
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(t.Host, strconv.Itoa(t.Port)))
	if err != nil {
		return nil, err
	}
	if err := adbService(conn, "host:transport:"+serial); err != nil {
		conn.Close()
		return nil, err
	}
	if err := adbService(conn, "localabstract:"+socketName); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func (t *RemoteADBTransport) Connect(ctx context.Context, addr string) error {
	output, err := runADBCommand(ctx, t.ADBExecutable, t.prefixArgs("connect", addr)...)
	if err != nil {
		return err
	}
	if adbOutputFailed(output) {
		return adbOutputError("adb connect failed", output)
	}
	return nil
}

func (t *RemoteADBTransport) Pair(ctx context.Context, addr, code string) error {
	output, err := runADBCommand(ctx, t.ADBExecutable, t.prefixArgs("pair", addr, code)...)
	if err != nil {
		return err
	}
	if !strings.Contains(string(output), "Successfully paired") {
		return adbOutputError("adb pair failed", output)
	}
	return nil
}

func (t *RemoteADBTransport) Capabilities() ADBTransportCaps {
	return ADBTransportCaps{
		CanReverse:        false,
		CanTCPIPConnect:   true,
		CanPair:           true,
		SupportsDirectAbs: true,
	}
}

func (t *RemoteADBTransport) Bridge() sdriver.BridgeID {
	return t.BridgeID
}

func (t *RemoteADBTransport) prefixArgs(args ...string) []string {
	prefix := []string{"-H", t.Host, "-P", strconv.Itoa(t.Port)}
	return append(prefix, args...)
}
