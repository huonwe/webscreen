package scrcpy

type ScrcpyParams struct {
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
	CodecID [4]byte
	Width   [4]byte
	Height  [4]byte
}

type ScrcpyVideoFrameHeader struct {
	PTS  [8]byte
	Size [4]byte
}

type ScrcpyVideoFrame struct {
	Header  ScrcpyVideoFrameHeader
	Payload []byte
}

type ScrcpyAudioMeta struct {
	CodecID [4]byte
}
