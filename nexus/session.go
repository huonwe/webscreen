package nexus

import (
	"log"
	"sync"
	"webscreen/sdriver"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v5"
)

// Remote：彻底干净，只持有设备驱动
type Remote struct {
	RemoteUUID string
	sdriver    sdriver.SDriver
}

// ViewSession：封装具体传输协议（WebRTC / WS / WT）
type ViewSession struct {
	SessionID        string
	UserID           string
	CanControl       bool
	ConnectionConfig ConnectionConfig

	useLocalTimestamp bool
	videoTSMapper     *rtpTimestampMapper
	audioTSMapper     *rtpTimestampMapper

	// 各协议私有字段（WebRTC peerConnection / Track，或 WS/WT Conn）
	// Only used for WebRTC connections, to store the final SDP answer after negotiation.
	FinalSDP2Nexus  string
	videoTrack      *webrtc.TrackLocalStaticRTP
	audioTrack      *webrtc.TrackLocalStaticRTP
	rtpSenderVideo  *webrtc.RTPSender
	rtpSenderAudio  *webrtc.RTPSender
	peerConnection  *webrtc.PeerConnection
	audioPacketizer rtp.Packetizer
	videoPacketizer rtp.Packetizer

	// 建议内部带一个带缓冲的 sendCh chan AVBox，由独立写协程消费，防止单个慢观众拖垮全局广播
	videoCh   chan sdriver.AVBox // 例如 cap = 64
	audioCh   chan sdriver.AVBox // 例如 cap = 32
	done      chan struct{}
	closeOnce sync.Once
	closeFn   func() // 协议专属关闭逻辑（如 peerConnection.Close()）
}

// func New

// PushAV 非阻塞写入：如果某个观众网络极卡导致缓冲区满，直接丢帧保护其他观众
func (v *ViewSession) PushBox(box sdriver.AVBox, thisIsVideo bool) {
	if v.videoCh == nil || v.audioCh == nil {
		log.Printf("[nexus] PushPacket: videoCh or audioCh is nil, dropping packet")
		return
	}
	select {
	case <-v.done:
		return
	default:
		switch v.ConnectionConfig.Protocol {
		case "webrtc":

			// 直接写入 WebRTC TrackLocalStaticRTP
			if thisIsVideo {
				ts := v.videoTSMapper.NextTimestamp(box)
				packets := v.videoPacketizer.Packetize(box.Data, 0)
				for _, packet := range packets {
					packet.Timestamp = ts
					_ = v.videoTrack.WriteRTP(packet)
				}
			} else {
				// Audio
				ts := v.audioTSMapper.NextTimestamp(box)
				packets := v.audioPacketizer.Packetize(box.Data, 0)
				for _, packet := range packets {
					packet.Timestamp = ts
					_ = v.audioTrack.WriteRTP(packet)
				}
			}
		}
	}
}

func (v *ViewSession) Close() {
	v.closeOnce.Do(func() {
		close(v.done) // 通知发送协程和 PushAV 安全退出，不关闭 data channel
		if v.closeFn != nil {
			v.closeFn()
		}
	})
}
