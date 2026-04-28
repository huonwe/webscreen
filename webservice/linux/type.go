package linux

import sagent "webscreen/streamAgent"

type LinuxDevice struct {
	DeviceID string `json:"device_id"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Status   string `json:"status"`
}

func (d LinuxDevice) GetType() string {
	return sagent.DEVICE_TYPE_LINUX
}

func (d LinuxDevice) GetDeviceID() string {
	return d.DeviceID
}

func (d LinuxDevice) GetIP() string {
	return d.IP
}

func (d LinuxDevice) GetPort() int {
	return d.Port
}

func (d LinuxDevice) GetStatus() string {
	return d.Status
}
