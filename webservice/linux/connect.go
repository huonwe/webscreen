package linux

import "os/exec"

func GetDevices() ([]LinuxDevice, error) {
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		return []LinuxDevice{
			{
				DeviceID: "Linux Desktop",
				IP:       "127.0.0.1",
				Port:     0,
				Status:   "active",
			},
		}, nil
	}
	return nil, nil
}
