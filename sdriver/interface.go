package sdriver

type SDriver interface {
	GetReceivers() (<-chan AVBox, <-chan AVBox, chan Event)
	SendEvent(event Event) error

	Start()
	Pause()

	RequestIDR(firstFrame bool)
	Capabilities() DriverCaps
	// CodecInfo() (videoCodec string, audioCodec string)
	MediaMeta() MediaMeta
	Stop()

	ConfigDescription() map[string]string
}
