package sagent

import (
	"sync"
	"webcpy/sdriver"

	"github.com/pion/webrtc/v4"
)

type Agent struct {
	sync.RWMutex
	VideoTrack  *webrtc.TrackLocalStaticSample
	AudioTrack  *webrtc.TrackLocalStaticSample
	ControlChan chan sdriver.ControlEvent
	MediaMeta   sdriver.MediaMeta

	Driver sdriver.SDriver

	// 用来接收前端的 RTCP 请求
	RtpSenderVideo *webrtc.RTPSender
	RtpSenderAudio *webrtc.RTPSender

	Config map[string]string
}
