package android

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func ExecADB(args ...string) error {
	cmd := exec.Command("adb", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetDevices returns a list of connected devices in the format "serial@status"
func GetDevices() ([]string, error) {
	cmd := exec.Command("adb", "devices")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var devices []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "List of devices attached") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			switch parts[1] {
			case "device":
				devices = append(devices, parts[0]+"@connected")
			case "offline":
				devices = append(devices, parts[0]+"@offline")
			case "unauthorized":
				devices = append(devices, parts[0]+"@unauthorized")
			}
		}
	}
	return devices, nil
}

// ConnectDevice connects to a device via TCP/IP
func ConnectDevice(address string) error {
	cmd := exec.Command("adb", "connect", address)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb connect failed: %v, output: %s", err, string(output))
	}
	if strings.Contains(string(output), "unable to connect") || strings.Contains(string(output), "failed to connect") {
		return fmt.Errorf("adb connect failed: %s", string(output))
	}
	return nil
}

// PairDevice pairs with a device using a pairing code
func PairDevice(address, code string) error {
	cmd := exec.Command("adb", "pair", address, code)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb pair failed: %v, output: %s", err, string(output))
	}
	if !strings.Contains(string(output), "Successfully paired") {
		return fmt.Errorf("adb pair failed: %s", string(output))
	}
	return nil
}
