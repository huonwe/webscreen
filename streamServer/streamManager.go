package streamServer

import (
	"fmt"
	"io"
	"log"
	"sync"
	"time"
	"webcpy/scrcpy"

	"github.com/gin-gonic/gin"
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
	const StreamID = "android_live_stream"
	// 创建视频轨
	videoTrack, _ := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
		"video-track-id",
		StreamID, // <--- 关键点
	)

	// 创建音频轨
	audioTrack, _ := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, // 假设音频是 Opus
		"audio-track-id",
		StreamID, // <--- 必须和视频的一样！
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

	sample := media.Sample{
		Data:      webrtcFrame.Data,
		Duration:  duration,
		Timestamp: time.UnixMicro(webrtcFrame.Timestamp),
	}
	// sm.RLock()
	// track := sm.VideoTrack
	// sm.RUnlock()
	err := sm.VideoTrack.WriteSample(sample)
	if err != nil {
		return fmt.Errorf("写入视频样本失败: %v", err)
	}
	// sm.DataAdapter.VideoPayloadPool.Put(webrtcFrame.Data)
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
	// sm.DataAdapter.AudioPayloadPool.Put(webrtcFrame.Data)
	return nil
}

func (sm *StreamManager) HandleSDP(c *gin.Context) {
	// 允许跨域 (方便调试)
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	if c.Request.Method == "OPTIONS" {
		return
	}

	// A. 读取浏览器发来的 Offer SDP
	body, _ := io.ReadAll(c.Request.Body)
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  string(body),
	}

	// B. 创建 PeerConnection
	// 配置 ICE 服务器 (STUN)，用于穿透 NAT
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	peerConnection, err := webrtc.NewAPI().NewPeerConnection(config)
	if err != nil {
		log.Println("创建 PeerConnection 失败:", err)
		c.String(500, err.Error())
		return
	}

	// 加读锁，防止读取的时候 track 正在被替换
	sm.RLock()
	currentVideoTrack := sm.VideoTrack
	currentAudioTrack := sm.AudioTrack
	sm.RUnlock()
	if currentVideoTrack == nil || currentAudioTrack == nil {
		log.Println("视频或音频轨道尚未准备好")
		c.String(500, "视频或音频轨道尚未准备好")
		return
	}

	// C. 添加视频轨道 (Video Track)
	// 只要浏览器一连上来，就把我们从 Android 收到的 H.264 流推给它
	sm.rtpSenderVideo, err = peerConnection.AddTrack(currentVideoTrack)
	if err != nil {
		log.Println("添加 Track 失败:", err)
		c.String(500, err.Error())
		return
	}
	// 添加音频轨道 (Audio Track)
	sm.rtpSenderAudio, err = peerConnection.AddTrack(currentAudioTrack)
	if err != nil {
		log.Println("添加 Audio Track 失败:", err)
		c.String(500, err.Error())
		return
	}
	// 启动协程读取 RTCP 包 (如 PLI 请求关键帧)
	go sm.HandleRTCP()
	// D. 设置 Remote Description (浏览器发来的 Offer)
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		log.Println("设置 Remote Description 失败:", err)
		c.String(500, err.Error())
		return
	}

	// E. 创建 Answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Println("创建 Answer 失败:", err)
		c.String(500, err.Error())
		return
	}
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("连接状态改变: %s", s)
		if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
			// 做一些清理工作，比如移除引用
			peerConnection.Close()
		}
	})

	// F. 设置 Local Description 并等待 ICE 收集完成
	// 这一步是为了生成一个包含所有网络路径信息的完整 SDP，
	// 这样我们就不需要写复杂的 Trickle ICE 逻辑了。
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	if err := peerConnection.SetLocalDescription(answer); err != nil {
		log.Println("设置 Local Description 失败:", err)
		c.String(500, err.Error())
		return
	}

	// 阻塞等待 ICE 收集完成 (通常几百毫秒)
	<-gatherComplete

	// G. 将最终的 SDP Answer 返回给浏览器
	c.Writer.Header().Set("Content-Type", "application/sdp")
	fmt.Fprint(c.Writer, peerConnection.LocalDescription().SDP)

	// H. 请求关键帧 (IDR)
	// 连接建立后，立即请求一个新的关键帧，确保客户端能马上看到画面
	if sm.DataAdapter != nil {
		go func() {
			// 稍微延迟一下，确保连接完全建立
			time.Sleep(500 * time.Millisecond)
			sm.DataAdapter.RequestKeyFrame()
		}()
	}
}
