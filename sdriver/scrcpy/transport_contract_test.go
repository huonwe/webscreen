package scrcpy

import (
	"context"
	"net"
	"webscreen/sdriver"
)

type contractADBTransport struct{}

var _ ADBTransport = (*contractADBTransport)(nil)

func (contractADBTransport) Devices(context.Context) ([]DeviceDescriptor, error) { return nil, nil }
func (contractADBTransport) Shell(context.Context, string, ...string) ([]byte, error) {
	return nil, nil
}
func (contractADBTransport) Push(context.Context, string, string, string) error { return nil }
func (contractADBTransport) Reverse(context.Context, string, string, string) error {
	return nil
}
func (contractADBTransport) ReverseRemove(context.Context, string, string) error { return nil }
func (contractADBTransport) OpenLocalAbstract(context.Context, string, string) (net.Conn, error) {
	return nil, nil
}
func (contractADBTransport) Connect(context.Context, string) error      { return nil }
func (contractADBTransport) Pair(context.Context, string, string) error { return nil }
func (contractADBTransport) Capabilities() ADBTransportCaps             { return ADBTransportCaps{} }
func (contractADBTransport) Bridge() sdriver.BridgeID                   { return "" }
func (contractADBTransport) StartScrcpySession(context.Context, sdriver.DeviceRef, ScrcpyOptions) (*ScrcpySession, error) {
	return nil, nil
}

type contractRootAgentScrcpyTransport struct{}

var (
	_ DeviceProvider  = (*contractRootAgentScrcpyTransport)(nil)
	_ ScrcpyTransport = (*contractRootAgentScrcpyTransport)(nil)
)

func (contractRootAgentScrcpyTransport) Devices(context.Context) ([]DeviceDescriptor, error) {
	return nil, nil
}
func (contractRootAgentScrcpyTransport) StartScrcpySession(context.Context, sdriver.DeviceRef, ScrcpyOptions) (*ScrcpySession, error) {
	return nil, nil
}
