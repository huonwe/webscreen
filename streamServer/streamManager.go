package streamServer

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"webcpy/scrcpy"

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
}

// 创建视频轨和音频轨，并初始化 StreamManager. 需要手动添加dataAdapter
func NewStreamManager(dataAdapter *scrcpy.DataAdapter) *StreamManager {
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
	return &StreamManager{
		VideoTrack:  videoTrack,
		AudioTrack:  audioTrack,
		DataAdapter: dataAdapter,
	}
}

func (sm *StreamManager) Close() {
	close(sm.DataAdapter.VideoChan)
	close(sm.DataAdapter.AudioChan)
	close(sm.DataAdapter.ControlChan)
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

	// 简单的防抖动：如果计算出的间隔太离谱（比如由暂停引起），重置为标准值
	if duration > time.Second {
		duration = time.Millisecond * 16
	}

	var pool sync.Pool
	// var dataToWrite []byte
	// log.Printf("length of payload: %v", cap(webrtcFrame.Data))
	if webrtcFrame.NotConfig {
		pool = sm.DataAdapter.PayloadPoolLarge
	} else {
		duration = 0
		pool = sm.DataAdapter.PayloadPoolSmall
	}
	// if bytes.HasPrefix(webrtcFrame.Data, []byte{0, 0, 0, 1}) {
	// 	// 去掉 4 字节起始码
	// 	dataToWrite = webrtcFrame.Data[4:]
	// 	log.Fatalln("Removed 4-byte start code from video frame, which is unexpected.")
	// } else if bytes.HasPrefix(webrtcFrame.Data, []byte{0, 0, 1}) {
	// 	// 去掉 3 字节起始码
	// 	dataToWrite = webrtcFrame.Data[3:]
	// 	log.Fatalln("Removed 3-byte start code from video frame, which is unexpected.")
	// } else {
	// 	dataToWrite = webrtcFrame.Data
	// }
	// dataToWrite = append([]byte{0, 0, 0, 1}, dataToWrite...)
	// dataToWrite = webrtcFrame.Data
	sample := media.Sample{
		Data:      webrtcFrame.Data,
		Duration:  duration,
		Timestamp: time.UnixMicro(webrtcFrame.Timestamp),
	}
	// sm.RLock()
	// track := sm.VideoTrack
	// sm.RUnlock()
	// log.Printf("Writing video sample, size: %d, duration: %v", len(sample.Data), sample.Duration)
	err := sm.VideoTrack.WriteSample(sample)
	if err != nil {
		return fmt.Errorf("写入视频样本失败: %v", err)
	}
	pool.Put(webrtcFrame.Data) // ⚠️ 禁止回收！Pion 的 NACK/RTX 机制会持有切片引用，回收会导致重传数据损坏（绿屏/花屏）
	return nil
}

func (sm *StreamManager) WriteAudioSample(webrtcFrame *scrcpy.WebRTCFrame) error {
	//sm.Lock()
	//defer sm.Unlock()
	//todo
	if sm.AudioTrack == nil {
		log.Println("Audio track is nil")
		return fmt.Errorf("音频轨道尚未准备好")
	}

	sample := media.Sample{
		Data:      webrtcFrame.Data,
		Duration:  time.Millisecond * 20, // 假设每个 Opus 帧是 20ms
		Timestamp: time.UnixMicro(webrtcFrame.Timestamp),
	}
	// sm.RLock()
	// track := sm.AudioTrack
	// sm.RUnlock()
	err := sm.AudioTrack.WriteSample(sample)
	if err != nil {
		return fmt.Errorf("写入音频样本失败: %v", err)
	}
	sm.DataAdapter.PayloadPoolSmall.Put(webrtcFrame.Data) // 同上，防止音频重传数据损坏
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
