package scrcpy

import (
	"context"
	"embed"
	"scrmux/sdriver"
	"scrmux/utils"
)

const (
	// SCRCPY_SERVER_LOCAL_PATH  = "/tmp/scrcpy-server"
	SCRCPY_SERVER_ANDROID_DST = "/data/local/tmp/scrcpy-server"
	SCRCPY_PROXY_PORT_DEFAULT = "27183"
	SCRCPY_VERSION            = "5.0.1"
	SCRCPY_EMBED_PATH         = "bin/scrcpy-server-v5.0.1"
)

//go:embed bin/scrcpy-server-v5.0.1
var scrcpyServerData embed.FS

// Receive an optional params
func ConfigDescription(opt string) []sdriver.ConfigParamDescription {
	deviceID := opt
	// var encoderList []string
	// var appList []string
	if utils.IsLocalRootMode() {
		// On the device itself the encoders are read from the local codec
		// configuration files instead of `adb shell grep`.
		// encoderList = LocalVideoEncoderList()
	} else if deviceID != "" {
		adbClient := NewADBClient(deviceID, "", context.Background())
		// encoderList = adbClient.SupportedVideoEncoderList()
		// appList = adbClient.AppList("3")
		adbClient.Stop()
	} else {
		// encoderList = []string{}
		// appList = []string{}
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
		// {
		// 	Name:        "video_codec",
		// 	Type:        "string",
		// 	Required:    true,
		// 	Default:     consts.CODEC_TYPE_H264,
		// 	Options:     []string{consts.CODEC_TYPE_H264, consts.CODEC_TYPE_H265},
		// 	Badge:       true,
		// 	Description: "video codec to use",
		// },
		// {
		// 	Name:        "video_encoder",
		// 	Type:        "string",
		// 	Required:    false,
		// 	Badge:       true,
		// 	Options:     encoderList,
		// 	Description: "video encoder to use",
		// },
		{
			Name:        "video_bit_rate",
			Type:        "string",
			Required:    true,
			Default:     "8M",
			Badge:       true,
			Description: "video bit rate",
		},
		{
			Name:        "intra_refresh",
			Type:        "boolean",
			Required:    true,
			Default:     true,
			Description: "enable intra refresh for the video stream",
		},
		// {
		// 	Name:        "video_codec_options",
		// 	Type:        "string",
		// 	Required:    false,
		// 	Description: "overwrite video codec options, e.g. profile=1,level=4.1,bitrate=8000000",
		// },
		{
			Name:        "max_size",
			Type:        "integer",
			Required:    false,
			Default:     1920,
			Options:     []string{"1280", "1920", "2560", "3840"},
			Description: "maximum video dimension (width or height) in pixels, e.g. 1920 for 1080p",
		},
		{
			Name:        "max_fps",
			Type:        "integer",
			Required:    false,
			Default:     60,
			Options:     []string{"30", "60", "90", "120"},
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
			Default:     "1920x1080",
			Options:     []string{"3840x2160", "2560x1600", "2560x1440", "1920x1080", "1280x720"},
			Description: "new display resolution",
		},
		// {
		// 	Name:        "start_app",
		// 	Type:        "string",
		// 	Required:    false,
		// 	Options:     appList,
		// 	Description: "package name of the app to start",
		// },
		// {
		// 	Name:        "no_video_codec_options",
		// 	Type:        "boolean",
		// 	Required:    false,
		// 	Default:     false,
		// 	Description: "If you face issues with video streaming, you can try to enable this to remove video_codec_options options",
		// },
	}
}
