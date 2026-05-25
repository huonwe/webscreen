package scrcpy

import (
	"testing"
	"webscreen/sdriver"
)

func TestParseADBDevicesLocal(t *testing.T) {
	output := []byte("List of devices attached\n5f9e7947 device usb:3-1 product:NE2210 model:NE2210 device:OP515BL1\nemulator-5554 offline\n")

	devices := parseADBDevices(output, sdriver.TransportLocalADB, "")
	if len(devices) != 2 {
		t.Fatalf("len(devices) = %d, want 2", len(devices))
	}
	if devices[0].Ref.Transport != sdriver.TransportLocalADB || devices[0].Ref.Serial != "5f9e7947" {
		t.Fatalf("unexpected first ref: %#v", devices[0].Ref)
	}
	if devices[0].Model != "NE2210" {
		t.Fatalf("model = %q, want NE2210", devices[0].Model)
	}
	if devices[1].Status != DeviceOffline {
		t.Fatalf("status = %q, want offline", devices[1].Status)
	}
}

func TestParseADBDevicesRemoteBridgeID(t *testing.T) {
	output := []byte("List of devices attached\n5f9e7947 unauthorized model:NE2210\n")

	devices := parseADBDevices(output, sdriver.TransportRemoteADB, "mac-coral")
	if len(devices) != 1 {
		t.Fatalf("len(devices) = %d, want 1", len(devices))
	}
	if devices[0].Ref.BridgeID != "mac-coral" || devices[0].BridgeID != "mac-coral" {
		t.Fatalf("bridge mismatch: %#v", devices[0])
	}
	if devices[0].Status != DeviceUnauthorized {
		t.Fatalf("status = %q, want unauthorized", devices[0].Status)
	}
}
