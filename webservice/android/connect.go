package android

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"webscreen/sdriver/scrcpy"
	"webscreen/utils"
)

func ExecADB(args ...string) error {
	adbPath, err := utils.GetADBPath()
	if err != nil {
		return err
	}
	cmd := exec.Command(adbPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetDevices returns a list of connected devices.
// In on-device (root) mode the device itself is the only available device.
func GetDevices() ([]AndroidDevice, error) {
	if utils.IsLocalRootMode() {
		return []AndroidDevice{
			{
				DeviceID: scrcpy.LocalDeviceName(),
				IP:       "127.0.0.1",
				Port:     0,
				Status:   "connected",
			},
		}, nil
	}

	adbPath, err := utils.GetADBPath()
	if err != nil {
		return nil, err
	}

	// 设备管理器在跑时以它的结果为准：那里区分了 USB / Wi-Fi、带授权状态、
	// 用户自定义名称与热插拔跟踪。首次轮询尚未完成时退回下面的直接解析。
	if m := GetManager(); m != nil {
		if legacy := m.legacyDevices(); len(legacy) > 0 {
			return legacy, nil
		}
	}

	cmd := exec.Command(adbPath, "devices", "-l")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var adbDevices []AndroidDevice
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "List of devices attached") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			transport := TransportWiFi
			for _, f := range parts[2:] {
				if strings.HasPrefix(f, "usb:") {
					transport = TransportUSB
				}
			}
			if strings.Contains(parts[0], ":") {
				transport = TransportWiFi
			}
			switch parts[1] {
			case "device":
				adbDevices = append(adbDevices, AndroidDevice{
					DeviceID:   parts[0],
					Status:     "connected",
					Connection: transport,
					State:      StatusOnline,
				})
			case "offline":
				adbDevices = append(adbDevices, AndroidDevice{
					DeviceID:   parts[0],
					Status:     "offline",
					Connection: transport,
					State:      StatusOffline,
				})
			case "unauthorized":
				adbDevices = append(adbDevices, AndroidDevice{
					DeviceID:   parts[0],
					Status:     "unauthorized",
					Connection: transport,
					State:      StatusUnauthorized,
				})
			}
		}
	}
	return adbDevices, nil
}

// ConnectDevice connects to a device via TCP/IP
func ConnectDevice(address string) error {
	adbPath, err := utils.GetADBPath()
	if err != nil {
		return err
	}
	cmd := exec.Command(adbPath, "connect", address)
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
	adbPath, err := utils.GetADBPath()
	if err != nil {
		return err
	}
	cmd := exec.Command(adbPath, "pair", address, code)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb pair failed: %v, output: %s", err, string(output))
	}
	if !strings.Contains(string(output), "Successfully paired") {
		return fmt.Errorf("adb pair failed: %s", string(output))
	}
	return nil
}
