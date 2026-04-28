package linuxDriver

import "webscreen/sdriver"

func ConfigDescription() []sdriver.ConfigParamDescription {
	return []sdriver.ConfigParamDescription{
		{
			Name:     "backend",
			Type:     "string",
			Required: true,
			Default:  "wayland",
			Options:  []string{"wayland", "xorg", "xvfb"},
			Description: "capture backend to use, e.g. 'wayland' for wf-recorder, " +
				"'xorg' for x11+kmsgrab, 'xvfb' for Xvfb + kmsgrab",
		},
		{
			Name:        "video_codec",
			Type:        "string",
			Required:    false,
			Default:     "h264",
			Options:     []string{"h264", "h265"},
			Description: "video codec to use",
		},

		{
			Name:        "video_bit_rate",
			Type:        "string",
			Required:    false,
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
			Required:    false,
			Description: "video resolution, e.g. 1920x1080",
		},
	}
}
