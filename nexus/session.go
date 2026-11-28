package nexus

import (
	"encoding/binary"
	"fmt"
	"log"
	"scrmux/sdriver"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v5"
)

// Remote：彻底干净，只持有设备驱动
type Remote struct {
	RemoteUUID   string
	RemoteConfig RemoteConfig
	sdriver      sdriver.SDriver
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

	MasterSessionLock sync.RWMutex
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

	// For WebSocket connections, to store the connection object
	WSConn *websocket.Conn

	// 所有线路公用的回调，用于处理来自前端的控制命令
	controlEventHandler func([]byte) error

	// 建议内部带一个带缓冲的 sendCh chan AVBox，由独立写协程消费，防止单个慢观众拖垮全局广播
	cacheVideoCh chan sdriver.AVBox // 例如 cap = 64
	cacheAudioCh chan sdriver.AVBox // 例如 cap = 32
	// cacheMsgCh   chan []byte        // 例如 cap = 64

	done      chan struct{}
	closeOnce sync.Once
	closeFn   func() // 协议专属关闭逻辑（如 peerConnection.Close()）
}

// func New

func makeWebSocketMediaFrame(kind byte, keyFrame bool, pts uint64, payload []byte) []byte {
	frame := make([]byte, 10+len(payload))
	frame[0] = kind
	if keyFrame {
		frame[1] = 1
	}
	binary.BigEndian.PutUint64(frame[2:10], pts)
	copy(frame[10:], payload)
	return frame
}

func isKeyFrameVideo(payload []byte, codec string) bool {
	if len(payload) == 0 {
		return false
	}

	if codec == "h264" {
		for i := 0; i+4 <= len(payload); {
			naluSize := int(binary.BigEndian.Uint32(payload[i : i+4]))
			i += 4
			if naluSize <= 0 || i+naluSize > len(payload) {
				break
			}
			if payload[i]&0x1f == 5 {
				return true
			}
			i += naluSize
		}
	}

	if codec == "h265" || codec == "hevc" {
		for i := 0; i+4 <= len(payload); {
			naluSize := int(binary.BigEndian.Uint32(payload[i : i+4]))
			i += 4
			if naluSize <= 0 || i+naluSize > len(payload) {
				break
			}
			nalType := (payload[i] >> 1) & 0x3f
			if nalType >= 16 && nalType <= 21 {
				return true
			}
			i += naluSize
		}
	}

	startCodeOffsets := make([]int, 0, 4)
	for i := 0; i+3 < len(payload); i++ {
		if payload[i] == 0 && payload[i+1] == 0 && payload[i+2] == 1 {
			startCodeOffsets = append(startCodeOffsets, i+3)
			i += 2
			continue
		}
		if i+4 <= len(payload) && payload[i] == 0 && payload[i+1] == 0 &&
			payload[i+2] == 0 && payload[i+3] == 1 {
			startCodeOffsets = append(startCodeOffsets, i+4)
			i += 3
		}
	}

	switch codec {
	case "h264":
		for _, offset := range startCodeOffsets {
			if offset < len(payload) && payload[offset]&0x1f == 5 {
				return true
			}
		}
		if len(startCodeOffsets) == 0 && payload[0]&0x1f == 5 {
			return true
		}
	case "h265", "hevc":
		for _, offset := range startCodeOffsets {
			if offset < len(payload) {
				nalType := (payload[offset] >> 1) & 0x3f
				if nalType >= 16 && nalType <= 21 {
					return true
				}
			}
		}
		if len(startCodeOffsets) == 0 && (payload[0]>>1)&0x3f >= 16 &&
			(payload[0]>>1)&0x3f <= 21 {
			return true
		}
	}
	return false
}

// 非阻塞写入：如果某个观众网络极卡导致缓冲区满，直接丢帧保护其他观众
func (v *ViewSession) PushingAVBox() {
	if v.cacheVideoCh == nil || v.cacheAudioCh == nil {
		log.Printf("[nexus] PushPacket: videoCh or audioCh is nil, reject to pushing")
		return
	}
	log.Println("[nexus] ViewSession PushingAVBox started, sessionID:", v.SessionID, "protocol:", v.ConnectionConfig.Protocol)
	if v.ConnectionConfig.Protocol == "websocket" {
		for {
			select {
			case box := <-v.cacheVideoCh:
				frame := makeWebSocketMediaFrame(0, isKeyFrameVideo(box.Data, v.ConnectionConfig.VideoCodec), box.PTS, box.Data)
				// log.Printf("[nexus ws] ViewSession %s pushing video frame: PTS=%d, size=%d bytes", v.SessionID, box.PTS, len(box.Data))
				if err := v.WSConn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
					log.Printf("[nexus] ViewSession %s video WriteMessage failed: %v", v.SessionID, err)
					v.Close()
					return
				}
			case box := <-v.cacheAudioCh:
				frame := makeWebSocketMediaFrame(1, false, box.PTS, box.Data)
				if err := v.WSConn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
					log.Printf("[nexus] ViewSession %s audio WriteMessage failed: %v", v.SessionID, err)
					v.Close()
					return
				}
			case <-v.done:
				log.Printf("[nexus] ViewSession %s done, exiting PushingBox goroutine", v.SessionID)
				return
			}
		}
	}

	if v.ConnectionConfig.Protocol == "webrtc" {
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
}

func (v *ViewSession) HandleMsg(event sdriver.Event) error {
	var msg []byte
	eType := event.Type()
	switch eType {
	case sdriver.EVENT_TYPE_RECEIVE_CLIPBOARD:
		event := event.(sdriver.SampleEvent)
		content := event.GetContent()
		msg = make([]byte, 1+len(content))
		copy(msg[1:], content)
		msg[0] = byte(sdriver.EVENT_TYPE_RECEIVE_CLIPBOARD)
		// if !yield(msg) {
		// 	return
		// }
	case sdriver.EVENT_TYPE_TEXT_MSG:
		event := event.(sdriver.TextMsgEvent)
		content := []byte(event.Msg)
		msg = make([]byte, 1+len(content))
		copy(msg[1:], content)
		msg[0] = byte(sdriver.EVENT_TYPE_TEXT_MSG)
		// if !yield(msg) {
		// 	return
		// }
	case sdriver.EVENT_TYPE_UPDATE_VIDEO_META:
		event := event.(sdriver.UpdateVideoMetaEvent)
		content := make([]byte, 8)
		binary.BigEndian.PutUint32(content[0:4], event.Width)
		binary.BigEndian.PutUint32(content[4:8], event.Height)
		msg = make([]byte, 1+len(content))
		copy(msg[1:], content)
		msg[0] = byte(sdriver.EVENT_TYPE_UPDATE_VIDEO_META)
		// if !yield(msg) {
		// 	return
		// }
	default:
		return fmt.Errorf("Unhandled event type in HandleMsg: %d", eType)
	}
	// log.Printf("[nexus] Handling message of type: %d", eType)
	switch v.ConnectionConfig.Protocol {
	case "websocket":
		v.WSConn.WriteMessage(websocket.BinaryMessage, msg)
	case "webrtc":
		if v.dataChannelUnordered != nil {
			log.Printf("[nexus] Sending message of type: %d over WebRTC data channel", eType)
			v.dataChannelUnordered.Send(msg)
		}
	}

	return nil

}

func (v *ViewSession) Close() {
	v.closeOnce.Do(func() {
		close(v.done) // 通知发送协程和 PushAV 安全退出，不关闭 data channel
		// close(v.cacheMsgCh) // 关闭消息缓存通道
		if v.closeFn != nil {
			v.closeFn()
		}
	})
}

// Done returns a channel that is closed when the ViewSession is closed.
func (v *ViewSession) Done() <-chan struct{} {
	return v.done
}
