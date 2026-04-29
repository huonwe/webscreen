package scrcpy

import (
	"context"
	"strings"
	"webscreen/sdriver"
)

// Receive an optional params
func ConfigDescription(opt string) []sdriver.ConfigParamDescription {
	deviceID := opt
	var encoderListStr string
	if deviceID != "" {
		adbClient := NewADBClient(deviceID, "", context.Background())
		encoderList := adbClient.SupportedEncoderList()
		encoderListStr = strings.Join(encoderList, ",")
		adbClient.Stop()
	} else {
		encoderListStr = ""
	}

	return []sdriver.ConfigParamDescription{
		{
			Name:        "audio",
			Type:        "boolean",
			Required:    true,
			Default:     true,
			Badge:       true,
			Description: "enable audio stream",
		},
		{
			Name:        "control",
			Type:        "boolean",
			Required:    true,
			Default:     true,
			Badge:       true,
			Description: "enable control stream",
		},
		{
			Name:        "video_codec",
			Type:        "string",
			Required:    true,
			Default:     "h264",
			Options:     []string{"h264", "h265"},
			Badge:       true,
			Description: "video codec to use",
		},
		{
			Name:     "video_encoder",
			Type:     "string",
			Required: false,
			Badge:    true,
			Options: func() []string {
				if encoderListStr == "" {
					return nil
				}
				return strings.Split(encoderListStr, ",")
			}(),
			Description: "video encoder to use, e.g. 'omx' for hardware encoding on Raspberry Pi",
		},
		{
			Name:        "video_bit_rate",
			Type:        "string",
			Required:    false,
			Badge:       true,
			Description: "video bit rate in bits per second, e.g. 20M for 20,000,000 bps",
		},
		{
			Name:        "video_codec_options",
			Type:        "string",
			Required:    false,
			Description: "additional options for the video codec, e.g. 'profile=1' for h264",
		},
		{
			Name:        "max_size",
			Type:        "integer",
			Required:    false,
			Description: "maximum video dimension (width or height) in pixels, e.g. 1920",
		},
		{
			Name:        "max_fps",
			Type:        "integer",
			Required:    false,
			Description: "maximum video frames per second, e.g. 60",
		},

		{
			Name:        "new_display",
			Type:        "boolean",
			Required:    false,
			Default:     false,
			Description: "whether to create a new virtual display for the session (Android 10+)",
		},
		{
			Name:        "resolution",
			Type:        "string",
			Required:    false,
			Description: "new display resolution, e.g. 1920x1080",
		},
		{
			Name:        "no_video_codec_options",
			Type:        "boolean",
			Required:    false,
			Default:     false,
			Description: "If you face issues with video streaming, you can try to enable this to remove video_codec_options options",
		},
	}
}
