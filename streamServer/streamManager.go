package streamServer

import (
	"fmt"
	"io"
	"log"
	"strings"
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

	var pool *sync.Pool
	var dataToWrite []byte
	if webrtcFrame.NotConfig {
		pool = &sm.DataAdapter.PayloadPoolLarge
		dataToWrite = webrtcFrame.Data[4:]

	} else {
		duration = 0
		pool = &sm.DataAdapter.PayloadPoolSmall
		dataToWrite = webrtcFrame.Data
	}
	sample := media.Sample{
		Data:      dataToWrite,
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
	pool.Put(webrtcFrame.Data)
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
	sm.DataAdapter.PayloadPoolSmall.Put(webrtcFrame.Data)
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

	// 创建 MediaEngine 并注册默认 Codec
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		log.Println("RegisterDefaultCodecs failed:", err)
		c.String(500, err.Error())
		return
	}

	// 检查是否需要注册 H.265
	sm.RLock()
	videoMime := sm.VideoTrack.Codec().MimeType
	sm.RUnlock()

	if videoMime == webrtc.MimeTypeH265 {
		log.Println("Registering H.265 Codec")
		if err := m.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:     webrtc.MimeTypeH265,
				ClockRate:    90000,
				Channels:     0,
				SDPFmtpLine:  "",
				RTCPFeedback: []webrtc.RTCPFeedback{{"goog-remb", ""}, {"ccm", "fir"}, {"nack", ""}, {"nack", "pli"}},
			},
			PayloadType: 102, // 动态 Payload Type
		}, webrtc.RTPCodecTypeVideo); err != nil {
			log.Println("RegisterCodec H265 failed:", err)
		}
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))
	peerConnection, err := api.NewPeerConnection(config)
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

	// 手动修改 SDP 以突破浏览器默认带宽限制 (SDP Munging)
	finalSDP := setSDPBandwidth(peerConnection.LocalDescription().SDP, 20000) // 20 Mbps
	fmt.Fprint(c.Writer, finalSDP)

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
