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
	NewDisplay        string
	LogLevel          string
}

type ConnectOptions struct {
	DeviceSerial  string
	ReversePort   int
	ScrcpyOptions ScrcpyOptions
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
	Width              uint32  // 显示宽度（扣除裁剪后）
	Height             uint32  // 显示高度（扣除裁剪后）
	FrameRate          float64 // 暂不支持 (需要解析 VUI，代价过高)
	Profile            uint8   // General Profile IDC
	ConstraintSetFlags uint8   // H.265 不像 H.264 有单个 byte 的 constraint，这里留空或存 Tier
	Level              string  // Level IDC (e.g. "5.1")
	Tier               string  // "Main" or "High"
	ChromaFormat       uint32  // 1=4:2:0, etc.
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
