package rockchiphdmirx

import (
	"os"
	"path/filepath"
	"strings"
)

var deviceTreeProperties = []string{
	"/sys/firmware/devicetree/base/model",
	"/proc/device-tree/model",
	"/sys/firmware/devicetree/base/compatible",
	"/proc/device-tree/compatible",
}

type Device struct {
	DeviceID string `json:"device_id"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Status   string `json:"status"`
}

func (d Device) GetType() string     { return "rockchip-hdmirx" }
func (d Device) GetDeviceID() string { return d.DeviceID }
func (d Device) GetIP() string       { return d.IP }
func (d Device) GetPort() int        { return d.Port }
func (d Device) GetStatus() string   { return d.Status }

// GetDevices reports the local HDMI RX on an RK3588 board.
func GetDevices() ([]Device, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	if !isRK3588() || !hasHDMIRXDevice() {
		return nil, nil
	}
	if _, err := os.Stat(defaultFFmpeg); err != nil {
		return nil, nil
	}

	return []Device{{
		DeviceID: hostname,
		IP:       "127.0.0.1",
		Port:     0,
		Status:   "active",
	}}, nil
}

func isRK3588() bool {
	for _, path := range deviceTreeProperties {
		property, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(property)), "rk3588") {
			return true
		}
	}
	return false
}

func hasHDMIRXDevice() bool {
	entries, err := filepath.Glob("/sys/class/video4linux/video*/name")
	if err != nil {
		return false
	}
	for _, path := range entries {
		name, err := os.ReadFile(path)
		if err == nil && strings.Contains(strings.ToLower(string(name)), "hdmirx") {
			return true
		}
	}
	return false
}
