package sagent

const (
	DEVICE_TYPE_SUNSHINE = "sunshine"
	DEVICE_TYPE_XVFB     = "xvfb"
	DEVICE_TYPE_ANDROID  = "android"
	DEVICE_TYPE_DUMMY    = "dummy"
)

type AgentConfig struct {
	DeviceType string `json:"device_type"`
	DeviceID   string `json:"device_id"`
	DeviceIP   string `json:"device_ip"`
	DevicePort string `json:"device_port"`
	// FilePath   string               `json:"file_path"` // move to StreamConfig.OtherOpts
	SDP          string            `json:"sdp"`
	AVSync       bool              `json:"av_sync"`
	DriverConfig map[string]string `json:"driver_config"`
}
