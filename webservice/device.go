package webservice

const (
	DeviceTypeAndroid = "android"
)

type Device struct {
	Type     string `json:"type"`
	DeviceID string `json:"device_id"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
}
