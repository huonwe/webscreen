package sdriver

type SDriver interface {
	// 第一个chan Event: Device -> Browser 的消息通道,主要用于传递剪贴板内容,以及文本消息(浏览器Toast显示). 下游依然可以使用MsgChan来发送消息(非只读)
	// 第二个chan Event: Browser-> Device 的消息通道,主要用于传递控制事件(触摸、按键、滚动、旋转、剪贴板等)
	// Browser-> Device 的消息通道,主要用于传递控制事件(触摸、按键、滚动、旋转、剪贴板等)
	GetReceivers() (<-chan AVBox, <-chan AVBox, chan Event, chan Event)
	SendEvent(event Event) error

	Start() error
	Pause()

	RequestIDR(firstFrame bool)
	Capabilities() DriverCaps
	// CanControl() bool
	// CodecInfo() (videoCodec string, audioCodec string)
	MediaMeta() MediaMeta
	Stop()

	// ConfigDescription() map[string]string
}
