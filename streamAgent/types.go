package sagent

// type deviceType string

const (
	DEVICE_TYPE_SUNSHINE string = "sunshine"
	DEVICE_TYPE_LINUX    string = "linux"
	DEVICE_TYPE_ANDROID  string = "android"
	DEVICE_TYPE_DUMMY    string = "dummy"
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
