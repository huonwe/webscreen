package webservice

const (
	DeviceTypeAndroid = "android"
)

type Device interface {
	GetType() string
	GetDeviceID() string
	GetIP() string
	GetPort() int
	GetStatus() string
}

type DeviceInfo struct {
	Type     string `json:"device_type"`
	DeviceID string `json:"device_id"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Status   string `json:"status"`
}
