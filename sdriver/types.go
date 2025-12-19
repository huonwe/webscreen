package sdriver

import "time"

// type DriverConfig map[string]string

type AVBox struct {
	Data       []byte        // H.264/H.265/AV1/.../Opus 裸流数据
	PTS        time.Duration // 相对开始时间的 PTS (Presentation Timestamp)
	IsKeyFrame bool          // 是否关键帧 (对 Video 很重要)
	IsConfig   bool          // 是否配置帧 (如果是配置帧, duration 应该为 0)
}

// ControlEvent represents an input event to be sent to the device.
// Everything that need send to the device. Touch, Key, Clipboard, etc.
type ControlEvent struct {
	Type uint8
	Data []byte
}

type MediaMeta struct {
	VideoCodecID string `json:"video_codec_id"`
	Width        uint32 `json:"width"`
	Height       uint32 `json:"height"`
	FPS          uint32 `json:"fps"`
	AudioCodecID string `json:"audio_codec_id"`
}

type DriverCaps struct {
	CanClipboard bool `json:"can_clipboard"`
	CanUHID      bool `json:"can_uhid"`
	CanVideo     bool `json:"can_video"`
	CanAudio     bool `json:"can_audio"`
	CanControl   bool `json:"can_control"`
}
