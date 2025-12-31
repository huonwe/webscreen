package sagent

const (
	DEVICE_TYPE_XVFB    = "xvfb"
	DEVICE_TYPE_ANDROID = "android"
	DEVICE_TYPE_DUMMY   = "dummy"
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

const (
	PAYLOAD_TYPE_AV1_PROFILE_MAIN_5_1            = 100 // 2560x1440 @ 60fps
	PAYLOAD_TYPE_H265_PROFILE_MAIN_TIER_MAIN_5_1 = 102 // 2560x1440 @ 60fps 40Mbps Max
	PAYLOAD_TYPE_H265_PROFILE_MAIN_TIER_MAIN_4_1 = 103 // 1920x1080 @ 60fps 20Mbps Max
	PAYLOAD_TYPE_H264_PROFILE_HIGH_5_1           = 104 // 2560x1440 @ 60fps
	PAYLOAD_TYPE_H264_PROFILE_HIGH_5_1_0C        = 105 // 2560x1440 @ 60fps for iphone safari
	PAYLOAD_TYPE_H264_PROFILE_BASELINE_3_1       = 106 // 720p @ 30fps
	PAYLOAD_TYPE_H264_PROFILE_BASELINE_3_1_0C    = 107 // 720p @ 30fps for iphone safari
)
