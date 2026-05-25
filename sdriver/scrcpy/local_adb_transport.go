package scrcpy

import (
	"context"
	"fmt"
	"net"
	"strings"
	"webscreen/sdriver"
	"webscreen/utils"
)

type LocalADBTransport struct {
	ADBExecutable string
}

func NewLocalADBTransport(adbExecutable ...string) (*LocalADBTransport, error) {
	executable := ""
	if len(adbExecutable) > 0 {
		executable = adbExecutable[0]
	}
	if executable == "" {
		var err error
		executable, err = utils.GetADBPath()
		if err != nil {
			return nil, err
		}
	}
	return &LocalADBTransport{ADBExecutable: executable}, nil
}

func (t *LocalADBTransport) Devices(ctx context.Context) ([]DeviceDescriptor, error) {
	output, err := runADBCommand(ctx, t.ADBExecutable, "devices", "-l")
	if err != nil {
		return nil, err
	}
	return parseADBDevices(output, sdriver.TransportLocalADB, ""), nil
}

func (t *LocalADBTransport) Shell(ctx context.Context, serial string, argv ...string) ([]byte, error) {
	args := appendSerial(nil, serial)
	args = append(args, "shell")
	args = append(args, argv...)
	return runADBCommand(ctx, t.ADBExecutable, args...)
}

func (t *LocalADBTransport) Push(ctx context.Context, serial, src, dst string) error {
	args := appendSerial(nil, serial)
	args = append(args, "push", src, dst)
	_, err := runADBCommand(ctx, t.ADBExecutable, args...)
	return err
}

func (t *LocalADBTransport) Reverse(ctx context.Context, serial, remote, local string) error {
	args := appendSerial(nil, serial)
	args = append(args, "reverse", remote, local)
	_, err := runADBCommand(ctx, t.ADBExecutable, args...)
	return err
}

func (t *LocalADBTransport) ReverseRemove(ctx context.Context, serial, remote string) error {
	args := appendSerial(nil, serial)
	args = append(args, "reverse", "--remove", remote)
	_, err := runADBCommand(ctx, t.ADBExecutable, args...)
	return err
}

func (t *LocalADBTransport) OpenLocalAbstract(ctx context.Context, serial, socketName string) (net.Conn, error) {
	return nil, ErrCapabilityNotSupported
}

func (t *LocalADBTransport) Connect(ctx context.Context, addr string) error {
	output, err := runADBCommand(ctx, t.ADBExecutable, "connect", addr)
	if err != nil {
		return err
	}
	if adbOutputFailed(output) {
		return adbOutputError("adb connect failed", output)
	}
	return nil
}

func (t *LocalADBTransport) Pair(ctx context.Context, addr, code string) error {
	output, err := runADBCommand(ctx, t.ADBExecutable, "pair", addr, code)
	if err != nil {
		return err
	}
	if !strings.Contains(string(output), "Successfully paired") {
		return adbOutputError("adb pair failed", output)
	}
	return nil
}

func (t *LocalADBTransport) Capabilities() ADBTransportCaps {
	return ADBTransportCaps{
		CanReverse:        true,
		CanTCPIPConnect:   true,
		CanPair:           true,
		SupportsDirectAbs: false,
	}
}

func (t *LocalADBTransport) Bridge() sdriver.BridgeID {
	return ""
}

func appendSerial(args []string, serial string) []string {
	if serial == "" {
		return args
	}
	return append(args, "-s", serial)
}

func adbOutputFailed(output []byte) bool {
	text := string(output)
	return strings.Contains(text, "unable to connect") || strings.Contains(text, "failed to connect")
}

func adbOutputError(prefix string, output []byte) error {
	return fmt.Errorf("%s: %s", prefix, strings.TrimSpace(string(output)))
}
