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
	videoTSMapper     *videoTimestampMapper
	audioTSMapper     *audioTimestampMapper

	// 各协议私有字段（WebRTC peerConnection / Track，或 WS/WT Conn）
	// Only used for WebRTC connections, to store the final SDP answer after negotiation.
	FinalSDP2Nexus       string
	videoTrack           *webrtc.TrackLocalStaticRTP
	audioTrack           *webrtc.TrackLocalStaticRTP
	peerConnection       *webrtc.PeerConnection
	audioPacketizer      rtp.Packetizer
	videoPacketizer      rtp.Packetizer
	dataChannelOrdered   *webrtc.DataChannel
	dataChannelUnordered *webrtc.DataChannel

	// 所有线路公用的回调，用于处理来自前端的控制命令
	onMessageCallback func([]byte) error

	// 建议内部带一个带缓冲的 sendCh chan AVBox，由独立写协程消费，防止单个慢观众拖垮全局广播
	cacheVideoCh chan sdriver.AVBox // 例如 cap = 64
	cacheAudioCh chan sdriver.AVBox // 例如 cap = 32

	msgCh     chan sdriver.Event
	controlCh chan sdriver.Event

	done      chan struct{}
	closeOnce sync.Once
	closeFn   func() // 协议专属关闭逻辑（如 peerConnection.Close()）
}

// func New

// 非阻塞写入：如果某个观众网络极卡导致缓冲区满，直接丢帧保护其他观众
func (v *ViewSession) PushingAVBox() {
	if v.cacheVideoCh == nil || v.cacheAudioCh == nil {
		log.Printf("[nexus] PushPacket: videoCh or audioCh is nil, reject to pushing")
		return
	}

	// go func() {
	for {
		select {
		case box := <-v.cacheVideoCh:
			ts := v.videoTSMapper.NextTimestamp(box)
			packets := v.videoPacketizer.Packetize(box.Data, 0)
			for _, packet := range packets {
				packet.Timestamp = ts
				_ = v.videoTrack.WriteRTP(packet)
			}
			// nalTypeF := func(payloadBuf byte) byte { return (payloadBuf >> 1) & 0x3F }
			// naltype := nalTypeF(box.Data[0]) // 注意：box.Data 前 4 字节是起始码
			// log.Printf("[nexus] Pushed video frame: PTS=%d, RTP TS=%d, size=%d bytes, NAL type=%d, sessionID=%s", box.PTS, ts, len(box.Data), naltype, v.SessionID)
		case box := <-v.cacheAudioCh:
			// Audio
			ts := v.audioTSMapper.NextTimestamp(box)
			packets := v.audioPacketizer.Packetize(box.Data, 0)
			for _, packet := range packets {
				packet.Timestamp = ts
				_ = v.audioTrack.WriteRTP(packet)
			}
		// log.Printf("[nexus] Pushed audio frame: PTS=%d, RTP TS=%d, size=%d bytes", box.PTS, ts, len(box.Data))
		case <-v.done:
			log.Printf("[nexus] ViewSession %s done, exiting PushingBox goroutine", v.SessionID)
			return
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
