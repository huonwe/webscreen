package android

import sagent "webscreen/streamAgent"

type AndroidDevice struct {
	DeviceID string `json:"device_id"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Status   string `json:"status"`
}

func (d AndroidDevice) GetType() string {
	return sagent.DEVICE_TYPE_ANDROID
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
