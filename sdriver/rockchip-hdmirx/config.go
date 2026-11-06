package rockchiphdmirx

import "webscreen/sdriver"

func ConfigDescription() []sdriver.ConfigParamDescription {
	return []sdriver.ConfigParamDescription{
		{Name: "device", Type: "string", Required: false, Default: "auto", Description: "HDMI RX V4L2 device, e.g. /dev/video0"},
		{Name: "ffmpeg", Type: "string", Required: false, Default: "/usr/lib/jellyfin-ffmpeg/ffmpeg", Description: "FFmpeg executable with RKMPP encoders"},
		{Name: "video_codec", Type: "string", Required: true, Default: "h264", Options: []string{"h264", "h265"}, Badge: true, Description: "hardware video codec"},
		{Name: "video_bit_rate", Type: "string", Required: true, Default: "8M", Badge: true, Description: "video bit rate, e.g. 8M"},
		{Name: "frame_rate", Type: "integer", Required: false, Default: 60, Description: "capture frame rate"},
		{Name: "resolution", Type: "string", Required: true, Default: "1920x1080", Badge: true, Description: "capture resolution (might not work)"},
	}
}
