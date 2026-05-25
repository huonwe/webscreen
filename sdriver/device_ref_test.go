package sdriver

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeviceRefRoundTripRemoteADB(t *testing.T) {
	ref := DeviceRef{
		BridgeID:  "mac-coral",
		Serial:    "5f9e7947",
		Transport: TransportRemoteADB,
	}

	encoded := ref.Encode()
	if !strings.HasPrefix(encoded, "v1.") {
		t.Fatalf("encoded ref should have v1 prefix: %q", encoded)
	}
	decoded, err := DecodeDeviceRef(encoded)
	if err != nil {
		t.Fatalf("DecodeDeviceRef returned error: %v", err)
	}
	if decoded != ref {
		t.Fatalf("decoded ref mismatch: got %#v want %#v", decoded, ref)
	}
}

func TestDeviceRefFixtureRoundTrips(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.FromSlash("../openspec/changes/device-transport-refactor/fixtures/device_ref/*.json"))
	if err != nil {
		t.Fatalf("fixture glob failed: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatalf("no device_ref fixtures found")
	}
	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			data, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatalf("read fixture failed: %v", err)
			}
			var ref DeviceRef
			if err := json.Unmarshal(data, &ref); err != nil {
				t.Fatalf("unmarshal fixture failed: %v", err)
			}
			encoded := ref.Encode()
			if encoded == "" {
				t.Fatalf("Encode returned empty string for %#v", ref)
			}
			decoded, err := DecodeDeviceRef(encoded)
			if err != nil {
				t.Fatalf("DecodeDeviceRef returned error: %v", err)
			}
			if decoded != ref {
				t.Fatalf("decoded ref mismatch: got %#v want %#v", decoded, ref)
			}
		})
	}
}

func TestDeviceRefJSONFieldOrder(t *testing.T) {
	ref := DeviceRef{
		BridgeID:  "mac-coral",
		Serial:    "5f9e7947",
		Transport: TransportRemoteADB,
	}

	encoded := ref.Encode()
	const want = "v1.eyJhZ2VudF9pZCI6IiIsImJyaWRnZV9pZCI6Im1hYy1jb3JhbCIsInNlcmlhbCI6IjVmOWU3OTQ3IiwidHJhbnNwb3J0IjoicmVtb3RlX2FkYiJ9"
	if encoded != want {
		t.Fatalf("encoded ref mismatch:\ngot  %s\nwant %s", encoded, want)
	}
}

func TestDeviceRefLegacySerialAllowsWifiADBColon(t *testing.T) {
	ref, err := DecodeDeviceRef("192.168.1.100:5555")
	if err != nil {
		t.Fatalf("DecodeDeviceRef returned error: %v", err)
	}
	if ref.Transport != TransportLocalADB || ref.Serial != "192.168.1.100:5555" {
		t.Fatalf("legacy ref mismatch: %#v", ref)
	}
}

func TestDeviceRefRejectsUnknownVersion(t *testing.T) {
	_, err := DecodeDeviceRef("v2.abc")
	if !errors.Is(err, ErrUnknownDeviceRefVersion) {
		t.Fatalf("expected ErrUnknownDeviceRefVersion, got %v", err)
	}
}

func TestDeviceRefRejectsInvalidLegacySerial(t *testing.T) {
	for _, input := range []string{"serial/with/slash", "serial|pipe", "serial with space", "serial\x7f"} {
		t.Run(input, func(t *testing.T) {
			_, err := DecodeDeviceRef(input)
			if !errors.Is(err, ErrInvalidDeviceRef) {
				t.Fatalf("expected ErrInvalidDeviceRef, got %v", err)
			}
		})
	}
}
