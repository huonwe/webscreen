package nexus

import "github.com/gorilla/websocket"

type ConnectionConfig struct {
	Protocol string `json:"protocol_transport"`
	// Video codec should be the same as how remote connect to the nexus
	VideoCodec string `json:"video_codec"`
	// Audio codec must be opus
	AudioCodec string `json:"audio_codec"`
	// Whether to trust the remote timestamp
	DontTrustRemoteTimestamp bool `json:"dont_trust_remote_timestamp"`
	// if use webrtc, sdp should be stored here
	Extra map[string]string `json:"extra,omitempty"`

	WSConn *websocket.Conn `json:"-"`

	// RemoteType string `json:"remote_type"`
	RemoteUUID string `json:"remote_uuid"`
}

type RemoteConfig map[string]string

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
