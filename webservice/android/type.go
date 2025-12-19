package android

type AndroidDevice struct {
	DeviceID string `json:"device_id"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Status   string `json:"status"`
}

func (d AndroidDevice) GetType() string {
	return "android"
}

func (d AndroidDevice) GetDeviceID() string {
	return d.DeviceID
}

func (d AndroidDevice) GetIP() string {
	return d.IP
}

func (d AndroidDevice) GetPort() int {
	return d.Port
}

func (d AndroidDevice) GetStatus() string {
	return d.Status
}
