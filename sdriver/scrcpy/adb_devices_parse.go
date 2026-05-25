package scrcpy

import (
	"strings"
	"webscreen/sdriver"
)

func parseADBDevices(output []byte, transport sdriver.TransportID, bridgeID sdriver.BridgeID) []DeviceDescriptor {
	var devices []DeviceDescriptor
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices attached") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		serial := fields[0]
		status := parseADBStatus(fields[1])
		model := parseADBModel(fields[2:])
		ref := sdriver.DeviceRef{
			BridgeID:  bridgeID,
			Serial:    serial,
			Transport: transport,
		}
		if transport == sdriver.TransportLocalADB {
			ref.BridgeID = ""
		}

		devices = append(devices, DeviceDescriptor{
			Ref:       ref,
			Serial:    serial,
			Model:     model,
			Status:    status,
			Transport: transport,
			BridgeID:  ref.BridgeID,
		})
	}
	return devices
}

func parseADBStatus(status string) DeviceStatus {
	switch status {
	case "device":
		return DeviceConnected
	case "unauthorized":
		return DeviceUnauthorized
	case "offline":
		return DeviceOffline
	default:
		return DeviceUnknown
	}
}

func parseADBModel(fields []string) string {
	for _, field := range fields {
		if model, ok := strings.CutPrefix(field, "model:"); ok {
			return model
		}
	}
	return ""
}
