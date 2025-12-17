package sdriver

import "time"

type StreamConfig struct {
	DeviceID   string
	VideoCodec string
	AudioCodec string
	Bitrate    int
	OtherOpts  map[string]string
}

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
	VideoCodecID string
	Width        int
	Height       int
	AudioCodecID string
}

type DriverCaps struct {
	CanClipboard bool
	CanUHID      bool
	CanVideo     bool
	CanAudio     bool
	CanControl   bool
}
