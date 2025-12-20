package sagent

import (
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"
	"webcpy/sdriver"
	"webcpy/sdriver/dummy"
	"webcpy/sdriver/scrcpy"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

type Agent struct {
	sync.RWMutex
	VideoTrack *webrtc.TrackLocalStaticSample
	AudioTrack *webrtc.TrackLocalStaticSample

	driver     sdriver.SDriver
	driverCaps sdriver.DriverCaps
	config     AgentConfig
	// 用来接收前端的 RTCP 请求
	rtpSenderVideo *webrtc.RTPSender
	rtpSenderAudio *webrtc.RTPSender
	// chan
	videoCh   <-chan sdriver.AVBox
	audioCh   <-chan sdriver.AVBox
	controlCh <-chan sdriver.Event

	// 用于音视频推流的 PTS 记录
	lastVideoPTS time.Duration
	lastAudioPTS time.Duration
	baseTime     time.Time
}

// ========================
// SAgent 主要负责从 sdriver 接收媒体流并通过 WebRTC 发送出去
// 同时处理来自客户端的控制命令并传递给 sdriver
// ========================
// 创建视频轨和音频轨，并初始化 Agent. 可以选择是否开启音视频同步.
func NewAgent(config AgentConfig) (*Agent, error) {
	sa := &Agent{
		config:   config,
		baseTime: time.Now(),
	}
	switch config.DeviceType {
	case DEVICE_TYPE_DUMMY:
		// 初始化 Dummy Driver
		dummyDriver, err := dummy.New(config.DriverConfig)
		if err != nil {
			log.Printf("Failed to initialize dummy driver: %v", err)
			return nil, err
		}
		sa.driver = dummyDriver
	case DEVICE_TYPE_ANDROID:
		// 初始化 Android Driver
		androidDriver, err := scrcpy.New(config.DriverConfig, config.DeviceID)
		if err != nil {
			log.Printf("Failed to initialize Android driver: %v", err)
			return nil, err
		}
		sa.driver = androidDriver
	default:
		log.Printf("Unsupported device type: %s", config.DeviceType)
		return nil, fmt.Errorf("unsupported device type: %s", config.DeviceType)
	}
	sa.driverCaps = sa.driver.Capabilities()
	// sa.videoCh, sa.audioCh, sa.controlCh = sa.driver.GetReceivers()
	sa.videoCh, sa.audioCh, sa.controlCh = sa.driver.GetReceivers()
	mediaMeta := sa.driver.MediaMeta()
	log.Printf("Driver media meta: %+v", mediaMeta)
	var videoMimeType, audioMimeType string
	switch mediaMeta.VideoCodecID {
	case "h264":
		videoMimeType = webrtc.MimeTypeH264
	case "h265":
		videoMimeType = webrtc.MimeTypeH265
	case "av1":
		videoMimeType = webrtc.MimeTypeAV1
	default:
		log.Printf("Unsupported video codec: %s", mediaMeta.VideoCodecID)
	}
	switch mediaMeta.AudioCodecID {
	case "opus":
		audioMimeType = webrtc.MimeTypeOpus
	default:
		log.Printf("Unsupported audio codec: %s", mediaMeta.AudioCodecID)
	}
	log.Printf("Creating tracks with MIME types - Video: %s, Audio: %s", videoMimeType, audioMimeType)
	streamID := generateStreamID()

	var videoStreamID, audioStreamID string
	if sa.config.AVSync {
		videoStreamID = streamID
		audioStreamID = streamID
	} else {
		videoStreamID = streamID + "_video"
		audioStreamID = streamID + "_audio"
	}

	var videoTrack, audioTrack *webrtc.TrackLocalStaticSample
	if videoMimeType != "" {
		// 创建视频轨
		videoTrack, _ = webrtc.NewTrackLocalStaticSample(
			webrtc.RTPCodecCapability{MimeType: videoMimeType},
			"video-track-id",
			videoStreamID, // <--- 使用不同的 StreamID 以取消强制同步
		)
	}
	if audioMimeType != "" {
		// 创建音频轨
		audioTrack, _ = webrtc.NewTrackLocalStaticSample(
			webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, // 假设音频是 Opus
			"audio-track-id",
			audioStreamID, // <--- 使用不同的 StreamID 以取消强制同步
		)
	}
	sa.VideoTrack = videoTrack
	sa.AudioTrack = audioTrack

	// go sa.StartBroadcaster()
	return sa, nil
}

func (sa *Agent) HandleRTCP() {
	rtcpBuf := make([]byte, 1500)
	lastRTCPTime := time.Now()
	for {
		n, _, err := sa.rtpSenderVideo.Read(rtcpBuf)
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
				log.Println("IDR requested via RTCP PLI")
				sa.driver.RequestIDR()
			}
		}
	}
}

func (sa *Agent) CreateWebRTCConnection(offer string) string {
	var finalSDP string
	finalSDP, sa.rtpSenderVideo, sa.rtpSenderAudio = HandleSDP(offer, sa.VideoTrack, sa.AudioTrack)
	return finalSDP
}

func (sa *Agent) Close() {
	log.Printf("Closing agent for device %s", sa.config.DeviceID)
	sa.driver.Stop()
}

func (sa *Agent) GetCodecInfo() (string, string) {
	m := sa.driver.MediaMeta()
	return m.VideoCodecID, m.AudioCodecID
}

func (sa *Agent) GetMediaMeta() sdriver.MediaMeta {
	return sa.driver.MediaMeta()
}

func (sa *Agent) Capabilities() sdriver.DriverCaps {
	return sa.driver.Capabilities()
}

func (sa *Agent) StartStreaming() {
	sa.driver.StartStreaming()
	go sa.StreamingVideo()
	go sa.StreamingAudio()
	go sa.HandleRTCP()
	sa.driver.RequestIDR()
}

func (sa *Agent) PauseStreaming() {
}

func (sa *Agent) ResumeStreaming() {

}

func (sa *Agent) SendEvent(raw []byte) error {
	if !sa.driverCaps.CanControl {
		return fmt.Errorf("driver does not support control events")
	}
	event, err := sa.parseEvent(raw)
	if err != nil {
		log.Printf("[agent] Failed to parse control event: %v", err)
		return err
	}
	// log.Printf("Parsed control event: %+v", event)
	return sa.driver.SendEvent(event)
}

func generateStreamID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "webscreen-stream"
	}
	return fmt.Sprintf("webscreen-%x", b)
}
