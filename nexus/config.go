package nexus

import "scrmux/consts"

func ConfigDescription() []ConfigParamDescription {

	return []ConfigParamDescription{
		{
			Name:        "protocol_transport",
			Type:        "string",
			Required:    true,
			Default:     consts.PROTOCOL_TYPE_WEBRTC,
			Options:     []string{consts.PROTOCOL_TYPE_WEBRTC, consts.PROTOCOL_TYPE_WEBSOCKET},
			Description: "Connection protocol to use.",
		},
		{
			Name:        "video_codec",
			Type:        "string",
			Required:    true,
			Default:     consts.CODEC_TYPE_H264,
			Options:     []string{consts.CODEC_TYPE_H264, consts.CODEC_TYPE_H265},
			Description: "Video codec to use for the connection.",
		},
		{
			Name:        "audio_codec",
			Type:        "string",
			Required:    true,
			Default:     consts.CODEC_TYPE_OPUS,
			Options:     []string{consts.CODEC_TYPE_OPUS},
			Description: "Audio codec to use for the connection.",
		},
		{
			Name:        "av_sync",
			Type:        "boolean",
			Required:    true,
			Default:     false,
			Description: "Enable A/V sync. Only for WebRTC. Useful when watching videos. But it may introduce additional latency.",
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
