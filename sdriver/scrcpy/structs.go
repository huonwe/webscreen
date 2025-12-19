package scrcpy

type ScrcpyOptions struct {
	Version      string `json:"version"`
	MaxSize      string `json:"max_size"`
	MaxFPS       string `json:"max_fps"`
	VideoBitRate string `json:"video_bit_rate"`

	VideoCodec        string `json:"video_codec"`
	VideoCodecOptions string `json:"video_codec_options"`
	NewDisplay        string `json:"new_display"`
}

// type ConnectOptions struct {
// 	DeviceSerial  string
// 	ReversePort   int
// 	ScrcpyOptions ScrcpyOptions
// }

type ScrcpyVideoMeta struct {
	CodecID string
	Width   uint32
	Height  uint32
}
type ScrcpyAudioMeta struct {
	CodecID string
}

type ScrcpyFrameHeader struct {
	IsConfig   bool
	IsKeyFrame bool
	PTS        uint64
	Size       uint32
}

type ScrcpyFrame struct {
	Header  ScrcpyFrameHeader
	Payload []byte
}

type OpusHead struct {
	Magic      [8]byte
	Version    byte
	Channels   byte
	PreSkip    uint16
	SampleRate uint32
	OutputGain int16 // 注意：有符号
	Mapping    byte
}

type WebRTCFrame struct {
	Data      []byte
	Timestamp int64
	NotConfig bool
}

type WebRTCControlFrame struct {
	Data      []byte
	Timestamp int64
}
