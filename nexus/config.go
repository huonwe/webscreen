package nexus

func ConfigDescription() []ConfigParamDescription {

	return []ConfigParamDescription{
		// {
		// 	Name:        "av_sync",
		// 	Type:        "boolean",
		// 	Required:    true,
		// 	Default:     false,
		// 	Description: "Enable A/V sync. Useful when watching videos.",
		// },
		// {
		// 	Name:        "use_local_timestamp",
		// 	Type:        "boolean",
		// 	Required:    true,
		// 	Default:     true,
		// 	Description: "Use local timestamp instead of device timestamp. This may reduce latency but the video may be less smooth.",
		// },
		// {
		// 	Name:        "websocket_instead_of_webrtc",
		// 	Type:        "boolean",
		// 	Required:    true,
		// 	Default:     false,
		// 	Description: "Use WebSocket instead of WebRTC for streaming. (Now developing, no real effect yet)",
		// },
		{
			Name:        "protocol_transport",
			Type:        "string",
			Required:    true,
			Default:     "webrtc",
			Options:     []string{"webrtc", "websocket"},
			Description: "Connection protocol to use.",
		},
		{
			Name:        "video_codec",
			Type:        "string",
			Required:    true,
			Default:     "h264",
			Options:     []string{"h264", "h265", "av1"},
			Description: "Video codec to use for the connection.",
		},
		{
			Name:        "audio_codec",
			Type:        "string",
			Required:    true,
			Default:     "opus",
			Options:     []string{"opus"},
			Description: "Audio codec to use for the connection.",
		},
		{
			Name:        "av_sync",
			Type:        "boolean",
			Required:    true,
			Default:     false,
			Description: "Enable A/V sync. Useful when watching videos. But it may introduce additional latency.",
		},
		{
			Name:        "dont_trust_remote_timestamp",
			Type:        "boolean",
			Required:    true,
			Default:     false,
			Description: "Do not trust the remote timestamp. Useful for reducing latency in some cases. But it may cause video to be less smooth, and also degrade A/V sync.",
		},
	}
}
