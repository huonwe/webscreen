package sagent

import "webcpy/sdriver"

// ========================
// SAgent 主要负责从 sdriver 接收媒体流并通过 WebRTC 发送出去
// 同时处理来自客户端的控制命令并传递给 sdriver
// ========================
type SAgent interface {
	// SetDriver(sdriver sdriver.SDriver)
	HandleSDP(offerSDP string) string
	StartStream()
	PauseStream()
	ResumeStream()
	SendControlEvent(event sdriver.ControlEvent) error
	Close()
}
