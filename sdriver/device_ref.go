package sdriver

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
)

type TransportID string
type BridgeID string

const (
	TransportLocalADB  TransportID = "local_adb"
	TransportRemoteADB TransportID = "remote_adb"
	TransportRootAgent TransportID = "root_agent"
)

var (
	ErrInvalidDeviceRef        = errors.New("invalid device ref")
	ErrUnknownDeviceRefVersion = errors.New("unknown device ref version")
)

var legacyADBSerialRE = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)

type DeviceRef struct {
	AgentID   string      `json:"agent_id"`
	BridgeID  BridgeID    `json:"bridge_id"`
	Serial    string      `json:"serial"`
	Transport TransportID `json:"transport"`
}

func (r DeviceRef) Validate() error {
	switch r.Transport {
	case TransportLocalADB:
		if r.Serial == "" || r.BridgeID != "" || r.AgentID != "" {
			return fmt.Errorf("%w: invalid local_adb fields", ErrInvalidDeviceRef)
		}
	case TransportRemoteADB:
		if r.Serial == "" || r.BridgeID == "" || r.AgentID != "" {
			return fmt.Errorf("%w: invalid remote_adb fields", ErrInvalidDeviceRef)
		}
	case TransportRootAgent:
		if r.AgentID == "" || r.Serial != "" {
			return fmt.Errorf("%w: invalid root_agent fields", ErrInvalidDeviceRef)
		}
	default:
		return fmt.Errorf("%w: invalid transport %q", ErrInvalidDeviceRef, r.Transport)
	}
	return nil
}

func (r DeviceRef) Encode() string {
	if err := r.Validate(); err != nil {
		return ""
	}
	payload, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return "v1." + base64.RawURLEncoding.EncodeToString(payload)
}

func DecodeDeviceRef(value string) (DeviceRef, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return DeviceRef{}, fmt.Errorf("%w: empty device ref", ErrInvalidDeviceRef)
	}
	if strings.HasPrefix(value, "v1.") {
		return decodeDeviceRefV1(strings.TrimPrefix(value, "v1."))
	}
	if strings.Contains(value, ".") {
		prefix, _, _ := strings.Cut(value, ".")
		if strings.HasPrefix(prefix, "v") {
			return DeviceRef{}, fmt.Errorf("%w: %s", ErrUnknownDeviceRefVersion, prefix)
		}
	}
	return DecodeLegacyDeviceRef(value)
}

func DecodeLegacyDeviceRef(serial string) (DeviceRef, error) {
	if !legacyADBSerialRE.MatchString(serial) {
		return DeviceRef{}, fmt.Errorf("%w: invalid legacy adb serial", ErrInvalidDeviceRef)
	}
	log.Printf("[device_ref] legacy adb serial %q upgraded to local_adb DeviceRef", serial)
	return DeviceRef{Transport: TransportLocalADB, Serial: serial}, nil
}

func decodeDeviceRefV1(encoded string) (DeviceRef, error) {
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return DeviceRef{}, fmt.Errorf("%w: invalid base64", ErrInvalidDeviceRef)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return DeviceRef{}, fmt.Errorf("%w: invalid json", ErrInvalidDeviceRef)
	}
	for key := range raw {
		switch key {
		case "agent_id", "bridge_id", "serial", "transport":
		default:
			return DeviceRef{}, fmt.Errorf("%w: unknown field %q", ErrInvalidDeviceRef, key)
		}
	}
	var ref DeviceRef
	if err := json.Unmarshal(payload, &ref); err != nil {
		return DeviceRef{}, fmt.Errorf("%w: invalid payload", ErrInvalidDeviceRef)
	}
	if err := ref.Validate(); err != nil {
		return DeviceRef{}, err
	}
	return ref, nil
}
