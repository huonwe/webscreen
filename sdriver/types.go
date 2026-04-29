package sdriver

// type DriverConfig map[string]string
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
type AVBox struct {
	Data       []byte // H.264/H.265/AV1/.../Opus 裸流数据
	PTS        uint64 // 相对开始时间的 PTS (Presentation Timestamp)
	NoDuration bool   // 是否不占用时间轴（如某些特殊的配置帧）
}

type MediaMeta struct {
	VideoCodec string `json:"video_codec"`
	Width      uint32 `json:"width"`
	Height     uint32 `json:"height"`
	FPS        uint32 `json:"fps"`
	AudioCodec string `json:"audio_codec"`
}

type DriverCaps struct {
	CanClipboard bool `json:"can_clipboard"`
	CanUHID      bool `json:"can_uhid"`
	CanVideo     bool `json:"can_video"`
	CanAudio     bool `json:"can_audio"`
	CanControl   bool `json:"can_control"`

	IsAndroid bool `json:"is_android"` // If true, show the android-specific buttons, like vol buttons, back, home, recent apps.
	IsLinux   bool `json:"is_linux"`
	IsWindows bool `json:"is_windows"`
}
