package streamServer

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"webcpy/scrcpy"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

type StreamManager struct {
	sync.RWMutex
	VideoTrack *webrtc.TrackLocalStaticSample
	AudioTrack *webrtc.TrackLocalStaticSample

	rtpSenderVideo *webrtc.RTPSender
	rtpSenderAudio *webrtc.RTPSender

	DataAdapter *scrcpy.DataAdapter

	lastVideoTimestamp int64

	hasSentKeyFrame atomic.Bool // 引入 atomic 避免并发问题
	webrtcConnected atomic.Bool // 标记 WebRTC 连接状态

	clients      map[*websocket.Conn]bool
	clientsMutex sync.Mutex

	StreamController *StreamController
}

func (sm *StreamManager) IsConnected() bool {
	return sm.webrtcConnected.Load()
}

// 创建视频轨和音频轨，并初始化 StreamManager. 需要手动添加dataAdapter
func NewStreamManager(dataAdapter *scrcpy.DataAdapter, controller *StreamController) *StreamManager {
	VideoStreamID := "android_live_stream_video"
	AudioStreamID := "android_live_stream_audio"

	var videoMimeType string
	switch dataAdapter.VideoMeta.CodecID {
	case "h265":
		videoMimeType = webrtc.MimeTypeH265
	case "av1 ":
		videoMimeType = webrtc.MimeTypeAV1
	default:
		videoMimeType = webrtc.MimeTypeH264
	}
	log.Printf("Creating video track with codec: %s", videoMimeType)

	// 创建视频轨
	videoTrack, _ := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: videoMimeType},
		"video-track-id",
		VideoStreamID, // <--- 关键点
	)

	// 创建音频轨
	audioTrack, _ := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, // 假设音频是 Opus
		"audio-track-id",
		AudioStreamID, // <--- 使用不同的 StreamID 以取消强制同步
	)
	sm := &StreamManager{
		VideoTrack:       videoTrack,
		AudioTrack:       audioTrack,
		DataAdapter:      dataAdapter,
		clients:          make(map[*websocket.Conn]bool),
		StreamController: controller,
	}
	go sm.StartBroadcaster()
	return sm
}

func (sm *StreamManager) StartBroadcaster() {
	for msg := range sm.DataAdapter.ControlChan {
		// Currently only broadcasting clipboard data (Type 17)
		// But we can broadcast other types if needed
		if len(msg.Data) > 0 && msg.Data[0] == WS_TYPE_CLIPBOARD_DATA {
			sm.clientsMutex.Lock()
			for client := range sm.clients {
				err := client.WriteMessage(websocket.BinaryMessage, msg.Data)
				if err != nil {
					log.Println("Broadcast error:", err)
					client.Close()
					delete(sm.clients, client)
				}
			}
			sm.clientsMutex.Unlock()
		}
	}
}

func (sm *StreamManager) AddClient(conn *websocket.Conn) {
	sm.clientsMutex.Lock()
	defer sm.clientsMutex.Unlock()
	sm.clients[conn] = true
}

func (sm *StreamManager) RemoveClient(conn *websocket.Conn) {
	sm.clientsMutex.Lock()
	delete(sm.clients, conn)
	count := len(sm.clients)
	sm.clientsMutex.Unlock()

	if count == 0 {
		log.Println("No more clients, stopping stream...")
		if sm.StreamController != nil {
			// Run in goroutine to avoid deadlock if called from within a lock
			go sm.StreamController.StopStream()
		}
	}
}

func (sm *StreamManager) Close() {
	// Channels are closed by DataAdapter.Close() or should be managed there.
	// But DataAdapter.Close() in adapter.go currently does NOT close channels.
	// If we close them here, we might panic if writing to closed channel.
	// Better to let GC handle channels or close them if we are sure no writers.
	// For now, we just rely on DataAdapter.Close() closing the connection,
	// which stops the reader loop, which stops writing to channels.
}

func (sm *StreamManager) UpdateTracks(v *webrtc.TrackLocalStaticSample, a *webrtc.TrackLocalStaticSample) {
	sm.Lock()
	defer sm.Unlock()
	sm.VideoTrack = v
	sm.AudioTrack = a
}

func (sm *StreamManager) WriteVideoSample(webrtcFrame *scrcpy.WebRTCFrame) error {
	//sm.Lock()
	//defer sm.Unlock()
	//todo
	if sm.VideoTrack == nil {
		return fmt.Errorf("视频轨道尚未准备好")
	}

	var duration time.Duration
	if webrtcFrame.NotConfig {
		if sm.lastVideoTimestamp == 0 {
			duration = time.Millisecond * 16
		} else {
			delta := webrtcFrame.Timestamp - sm.lastVideoTimestamp
			if delta <= 0 {
				duration = time.Microsecond
			} else {
				duration = time.Duration(delta) * time.Microsecond
			}
		}
		sm.lastVideoTimestamp = webrtcFrame.Timestamp
	} else {
		// Config 帧 (VPS/SPS/PPS) 不需要持续时间
		duration = 0
	}

	// 简单的防抖动：如果计算出的间隔太离谱（比如由暂停引起），重置为标准值
	// if duration > time.Second {
	// 	duration = time.Millisecond * 16
	// }

	sample := media.Sample{
		Data:      webrtcFrame.Data,
		Duration:  duration,
		Timestamp: time.UnixMicro(webrtcFrame.Timestamp),
	}

	err := sm.VideoTrack.WriteSample(sample)
	if err != nil {
		return fmt.Errorf("写入视频样本失败: %v", err)
	}
	return nil
}

func (sm *StreamManager) WriteAudioSample(webrtcFrame *scrcpy.WebRTCFrame) error {
	if sm.AudioTrack == nil {
		log.Println("Audio track is nil")
		return fmt.Errorf("音频轨道尚未准备好")
	}

	sample := media.Sample{
		Data:      webrtcFrame.Data,
		Duration:  time.Millisecond * 20, // 假设每个 Opus 帧是 20ms
		Timestamp: time.UnixMicro(webrtcFrame.Timestamp),
	}

	err := sm.AudioTrack.WriteSample(sample)
	if err != nil {
		return fmt.Errorf("写入音频样本失败: %v", err)
	}
	return nil
}

// setSDPBandwidth 在 SDP 的 video m-line 后插入 b=AS:20000 (20Mbps)
func setSDPBandwidth(sdp string, bandwidth int) string {
	lines := strings.Split(sdp, "\r\n")
	var newLines []string
	for _, line := range lines {
		newLines = append(newLines, line)
		if strings.HasPrefix(line, "m=video") {
			// b=AS:<bandwidth>  (Application Specific Maximum, 单位 kbps)
			// 设置为 20000 kbps = 20 Mbps，远超默认的 2.5 Mbps
			newLines = append(newLines, fmt.Sprintf("b=AS:%d", bandwidth))
			// 也可以加上 TIAS (Transport Independent Application Specific Maximum, 单位 bps)
			// newLines = append(newLines, "b=TIAS:20000000")
		}
	}
	return strings.Join(newLines, "\r\n")
}
