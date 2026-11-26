package nexus

import "github.com/gorilla/websocket"

// type deviceType string

// const (
// 	CONNECTION_PROTOCOL_WEBRTC = iota
// 	CONNECTION_PROTOCOL_WEBTRANSPORT
// 	CONNECTION_PROTOCOL_WEBSOCKET
// )

const (
	UDP_PORT_START = 51200
	UDP_PORT_END   = 51299
)

const (
	PAYLOAD_TYPE_AV1_PROFILE_MAIN_5_1            = 100 // 2560x1440 @ 60fps
	PAYLOAD_TYPE_H265_PROFILE_MAIN_TIER_MAIN_5_1 = 102 // 2560x1440 @ 60fps 40Mbps Max
	PAYLOAD_TYPE_H265_PROFILE_MAIN_TIER_MAIN_4_1 = 103 // 1920x1080 @ 60fps 20Mbps Max
	PAYLOAD_TYPE_H264_PROFILE_HIGH_5_1           = 104 // 2560x1440 @ 60fps
	PAYLOAD_TYPE_H264_PROFILE_HIGH_5_1_0C        = 105 // 2560x1440 @ 60fps for iphone safari
	PAYLOAD_TYPE_H264_PROFILE_BASELINE_3_1       = 106 // 720p @ 30fps
	PAYLOAD_TYPE_H264_PROFILE_BASELINE_3_1_0C    = 107 // 720p @ 30fps for iphone safari
)

type ConnectionConfig struct {
	Protocol string `json:"protocol_transport"`
	// Video codec should be the same as how remote connect to the nexus
	VideoCodec string `json:"video_codec"`
	// Audio codec must be opus
	AudioCodec string `json:"audio_codec"`
	// if use webrtc, sdp should be stored here
	Extra map[string]string `json:"extra,omitempty"`

	WSConn *websocket.Conn `json:"-"`

	// RemoteType string `json:"remote_type"`
	RemoteUUID string `json:"remote_uuid"`
}

type RemoteConfig map[string]string

// type AgentConfig struct {
// 	DeviceType        string            `json:"device_type"`
// 	DeviceID          string            `json:"device_id"`
// 	DeviceIP          string            `json:"device_ip"`
// 	DevicePort        string            `json:"device_port"`
// 	SDP               string            `json:"sdp"`
// 	AVSync            bool              `json:"av_sync"`
// 	UseLocalTimestamp bool              `json:"use_local_timestamp"`
// 	DriverConfig      map[string]string `json:"driver_config"`
// }

type ConfigParamDescription struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Default  any      `json:"default,omitempty"`
	Options  []string `json:"options,omitempty"`
	// if true, the frontend can show this config param more prominently, like in a badge or highlight it in the UI, to indicate it's important or commonly used.
	Badge bool `json:"badge,omitempty"`

	Description string `json:"description"`
}
