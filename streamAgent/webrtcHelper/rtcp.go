package webrtcHelper

import (
	"time"
	"webcpy/sdriver"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

func HandleRTCP(rtpSender *webrtc.RTPSender, sdriver sdriver.SDriver) {
	rtcpBuf := make([]byte, 1500)
	lastRTCPTime := time.Now()
	for {
		n, _, err := rtpSender.Read(rtcpBuf)
		if err != nil {
			return
		}
		packets, err := rtcp.Unmarshal(rtcpBuf[:n])
		if err != nil {
			continue
		}
		for _, p := range packets {
			switch p.(type) {
			case *rtcp.PictureLossIndication:
				now := time.Now()
				if now.Sub(lastRTCPTime) < time.Second*2 {
					continue
				}
				lastRTCPTime = now
				// log.Println("收到 PLI 请求 (Keyframe Request)")
				sdriver.RequestIDR()
			}
		}
	}
}
