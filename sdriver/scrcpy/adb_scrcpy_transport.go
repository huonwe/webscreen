package scrcpy

import (
	"context"
	"webscreen/sdriver"
)

type adbScrcpyTransport struct {
	adb ADBTransport
}

func newADBScrcpyTransport(adb ADBTransport) *adbScrcpyTransport {
	return &adbScrcpyTransport{adb: adb}
}

func (t *adbScrcpyTransport) Devices(ctx context.Context) ([]DeviceDescriptor, error) {
	return t.adb.Devices(ctx)
}

func (t *adbScrcpyTransport) StartScrcpySession(ctx context.Context, ref sdriver.DeviceRef, opts ScrcpyOptions) (*ScrcpySession, error) {
	if opts.SCID == "" {
		return nil, ErrSCIDRequired
	}
	return nil, ErrCapabilityNotSupported
}
