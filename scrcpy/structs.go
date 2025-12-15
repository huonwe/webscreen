package scrcpy

type ScrcpyOptions struct {
	CLASSPATH         string
	Version           string
	SCID              string
	MaxSize           string
	MaxFPS            string
	VideoBitRate      string
	Control           string
	Audio             string
	VideoCodec        string
	VideoCodecOptions string
	LogLevel          string
}

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
type SPSInfo struct {
	Width              uint32  // 视频宽度（像素）
	Height             uint32  // 视频高度（像素）
	FrameRate          float64 // 估算帧率
	Profile            uint8   // 新增: H.264 Profile
	ConstraintSetFlags uint8
	Level              string // 新增: H.264 Level (字符串表示)
}

type WebRTCFrame struct {
	Data      []byte
	Timestamp int64
	NotConfig bool
}

// type WebRTCAudioFrame struct {
// 	Data      []byte
// 	Timestamp int64
// }

type WebRTCControlFrame struct {
	Data      []byte
	Timestamp int64
}
