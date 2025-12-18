package sdriver

// ========================
// 定义接口 (你的抽象层)
// ========================

// 占位用的结构体

type SDriver interface {
	GetReceivers() (<-chan AVBox, <-chan AVBox, <-chan ControlEvent)
	SendEvent(event Event) error

	StartStreaming()
	StopStreaming()

	RequestIDR()
	Capabilities() DriverCaps
	// CodecInfo() (videoCodec string, audioCodec string)
	MediaMeta() MediaMeta
	Stop()
}
