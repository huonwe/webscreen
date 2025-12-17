package sdriver

// ========================
// 定义接口 (你的抽象层)
// ========================

// 占位用的结构体

type SDriver interface {
	StartStream(config StreamConfig) (<-chan AVBox, <-chan AVBox, <-chan ControlEvent, error)
	SendControl(event ControlEvent) error
	RequestIDR() error
	Capabilities() DriverCaps
	CodecInfo() (videoCodec string, audioCodec string)
	Stop() error
}
