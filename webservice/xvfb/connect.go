package xvfb

import "os/exec"

func GetDevices() ([]XvfbDevice, error) {
	if _, err := exec.LookPath("Xvfb"); err == nil {
		return []XvfbDevice{
			{
				DeviceID: "local_xvfb",
				IP:       "127.0.0.1",
				Port:     0,
				Status:   "active",
			},
		}, nil
	}
	return nil, nil
}
