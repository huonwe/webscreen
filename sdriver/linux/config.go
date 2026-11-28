package linuxDriver

import "scrmux/sdriver"

func ConfigDescription() []sdriver.ConfigParamDescription {
	return []sdriver.ConfigParamDescription{
		{
			Name:     "backend",
			Type:     "string",
			Required: true,
			Default:  "wayland",
			Options:  []string{"sway", "xorg", "xvfb"},
			Badge:    true,
			Description: "capture backend to use, e.g. 'wayland' for wf-recorder, " +
				"'xorg' for x11+kmsgrab, 'xvfb' for Xvfb + kmsgrab",
		},
		// {
		// 	Name:        "video_codec",
		// 	Type:        "string",
		// 	Required:    true,
		// 	Default:     consts.CODEC_TYPE_H265,
		// 	Options:     []string{consts.CODEC_TYPE_H264, consts.CODEC_TYPE_H265},
		// 	Badge:       true,
		// 	Description: "video codec to use",
		// },

		{
			Name:        "video_bit_rate",
			Type:        "string",
			Required:    true,
			Default:     "8M",
			Badge:       true,
			Description: "video bit rate in bits per second, e.g. 20M for 20,000,000 bps",
		},

		{
			Name:        "frame_rate",
			Type:        "integer",
			Required:    false,
			Default:     60,
			Description: "maximum video frames per second, e.g. 60",
		},

		{
			Name:        "resolution",
			Type:        "string",
			Required:    true,
			Default:     "1920x1080",
			Badge:       true,
			Options:     []string{"3840x2160", "2560x1600", "2560x1440", "1920x1080", "1280x720"},
			Description: "video resolution",
		},
	}
}
