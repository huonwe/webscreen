package xvfb

type XvfbDevice struct {
	DeviceID string `json:"device_id"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Status   string `json:"status"`
}

func (d XvfbDevice) GetType() string {
	return "xvfb"
}

func (d XvfbDevice) GetDeviceID() string {
	return d.DeviceID
}

func (d XvfbDevice) GetIP() string {
	return d.IP
}

func (d XvfbDevice) GetPort() int {
	return d.Port
}

func (d XvfbDevice) GetStatus() string {
	return d.Status
}
