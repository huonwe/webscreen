package nexus

func ConfigDescription() []ConfigParamDescription {

	return []ConfigParamDescription{
		{
			Name:        "av_sync",
			Type:        "boolean",
			Required:    true,
			Default:     false,
			Description: "Enable A/V sync. Useful when watching videos.",
		},
		{
			Name:        "use_local_timestamp",
			Type:        "boolean",
			Required:    true,
			Default:     true,
			Description: "Use local timestamp instead of device timestamp. This may reduce latency but the video may be less smooth.",
		},
		{
			Name:        "websocket_instead_of_webrtc",
			Type:        "boolean",
			Required:    true,
			Default:     false,
			Description: "Use WebSocket instead of WebRTC for streaming. (Now developing, no real effect yet)",
		},
		{
			Name:        "protocol",
			Type:        "string",
			Required:    true,
			Default:     "webrtc",
			Options:     []string{"webrtc", "websocket"},
			Description: "Connection protocol to use. (Now developing, no real effect yet)",
		},
	}
}
