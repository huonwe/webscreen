package gamestream

import (
	"fmt"
	"strconv"
	"strings"
	"webscreen/sdriver"
)

var (
	defaultHTTPPort = 47989
	defaultAuthDir  = "./sdriver/gamestream_auth"
)

func ConfigDescription() []sdriver.ConfigParamDescription {
	return []sdriver.ConfigParamDescription{
		{Name: "app", Type: "string", Required: false, Default: "Desktop", Badge: true, Description: "application name or id to launch on the host"},
		{Name: "video_codec", Type: "string", Required: true, Default: "h264", Options: []string{"h264", "h265"}, Badge: true, Description: "video codec"},
		{Name: "video_bit_rate", Type: "string", Required: true, Default: "10M", Badge: true, Description: "video bit rate, e.g. 10M"},
		{Name: "frame_rate", Type: "integer", Required: false, Default: 60, Description: "stream frame rate"},
		{Name: "resolution", Type: "string", Required: true, Default: "1920x1080", Options: []string{"3840x2160", "2560x1600", "2560x1440", "1920x1080", "1280x720"}, Badge: true, Description: "stream resolution"},
		// {Name: "pin", Type: "string", Required: false, Description: "4-digit PIN for first-time pairing; enter the same PIN on the host (Sunshine web UI) while connecting"},
		{Name: "port", Type: "integer", Required: false, Default: defaultHTTPPort, Description: "host HTTP port"},
		{Name: "sops", Type: "boolean", Required: false, Default: false, Description: "let the host optimize game settings (changes host resolution)"},
		{Name: "host_audio", Type: "boolean", Required: false, Default: false, Description: "also play audio on the host"},
		// {Name: "auth_dir", Type: "string", Required: false, Default: defaultAuthDir, Description: "local directory used to store the client key, certificate and paired server certificates"},
	}
}

type streamConfig struct {
	ip   string
	port int
	app  string
	// pin       string
	codec     string
	width     int
	height    int
	fps       int
	bitrate   int // kbps
	sops      bool
	hostAudio bool
	// authDir   string
}

func parseConfig(cfg map[string]string) (*streamConfig, error) {
	c := &streamConfig{
		ip:  strings.TrimSpace(cfg["ip"]),
		app: strings.TrimSpace(cfg["app"]),
		// pin:       strings.TrimSpace(cfg["pin"]),
		codec:     strings.ToLower(cfg["video_codec"]),
		sops:      isTrue(cfg["sops"]),
		hostAudio: isTrue(cfg["host_audio"]),
		// authDir:   cfg["auth_dir"],
	}
	if c.ip == "" {
		return nil, fmt.Errorf("gamestream: host ip is required")
	}
	if c.app == "" {
		c.app = "Desktop"
	}
	// if c.authDir == "" {
	// 	c.authDir = defaultAuthDir
	// }
	if c.codec == "" {
		c.codec = "h264"
	}
	if c.codec != "h264" && c.codec != "h265" {
		return nil, fmt.Errorf("gamestream: video_codec must be h264 or h265")
	}
	var err error
	if c.port, err = positiveInt(cfg["port"], defaultHTTPPort, "port"); err != nil {
		return nil, err
	}
	if c.fps, err = positiveInt(cfg["frame_rate"], 60, "frame_rate"); err != nil || c.fps > 240 {
		return nil, fmt.Errorf("gamestream: frame_rate must be between 1 and 240")
	}
	res := cfg["resolution"]
	if res == "" {
		res = "1920x1080"
	}
	if _, err := fmt.Sscanf(strings.ToLower(res), "%dx%d", &c.width, &c.height); err != nil ||
		c.width < 320 || c.height < 240 || c.width > 7680 || c.height > 4320 {
		return nil, fmt.Errorf("gamestream: invalid resolution %q", res)
	}
	if c.bitrate, err = parseBitrateKbps(cfg["video_bit_rate"]); err != nil {
		return nil, err
	}
	return c, nil
}

func isTrue(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func positiveInt(s string, def int, name string) (int, error) {
	if s == "" {
		return def, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return 0, fmt.Errorf("gamestream: %s must be a positive integer", name)
	}
	return v, nil
}

// parseBitrateKbps accepts "10M", "8000K", or a bare number of kbps.
func parseBitrateKbps(s string) (int, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 10000, nil
	}
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "M"):
		mult, s = 1000, strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "K"):
		s = strings.TrimSuffix(s, "K")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("gamestream: invalid video_bit_rate")
	}
	return int(v * mult), nil
}
