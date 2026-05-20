package scrcpy

import (
	"context"
	"errors"
	"net"
	"webscreen/sdriver"
)

var (
	ErrADBExecutableRequired  = errors.New("adb executable is required")
	ErrCapabilityNotSupported = errors.New("transport capability is not supported")
	ErrSCIDRequired           = errors.New("scrcpy scid is required")
)

type DeviceStatus string

const (
	DeviceConnected    DeviceStatus = "connected"
	DeviceUnauthorized DeviceStatus = "unauthorized"
	DeviceOffline      DeviceStatus = "offline"
	DeviceUnknown      DeviceStatus = "unknown"
)

type DeviceDescriptor struct {
	Ref       sdriver.DeviceRef
	Serial    string
	Model     string
	Status    DeviceStatus
	Transport sdriver.TransportID
	BridgeID  sdriver.BridgeID
}

type DeviceProvider interface {
	Devices(ctx context.Context) ([]DeviceDescriptor, error)
}

type ADBTransport interface {
	DeviceProvider

	Shell(ctx context.Context, serial string, argv ...string) ([]byte, error)
	Push(ctx context.Context, serial, src, dst string) error
	Reverse(ctx context.Context, serial, remote, local string) error
	ReverseRemove(ctx context.Context, serial, remote string) error
	OpenLocalAbstract(ctx context.Context, serial, socketName string) (net.Conn, error)
	Connect(ctx context.Context, addr string) error
	Pair(ctx context.Context, addr, code string) error

	Capabilities() ADBTransportCaps
	Bridge() sdriver.BridgeID
}

type ADBTransportCaps struct {
	CanReverse        bool
	CanTCPIPConnect   bool
	CanPair           bool
	SupportsDirectAbs bool
}

type ScrcpyTransport interface {
	DeviceProvider

	StartScrcpySession(ctx context.Context, ref sdriver.DeviceRef, opts ScrcpyOptions) (*ScrcpySession, error)
}

type ScrcpySession struct {
	SCID              string
	Video             net.Conn
	Audio             net.Conn
	Control           net.Conn
	NeedsForwardDummy bool
	Close             func() error
}
