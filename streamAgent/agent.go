package sagent

import (
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"
	"webscreen/sdriver"
	"webscreen/sdriver/dummy"
	"webscreen/sdriver/scrcpy"
	"webscreen/sdriver/sunshine"
	linuxXvfbDriver "webscreen/sdriver/xvfb"

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
	controlCh chan sdriver.Event

	negotiatedCodec chan webrtc.RTPCodecParameters

	// 用于音视频推流的 PTS 记录
	lastVideoPTS time.Duration
	lastAudioPTS time.Duration
	baseTime     time.Time
}

// ========================
// SAgent 负责接受来自sdriver的数据，并处理来自前端的控制命令。
// ========================
// 创建视频轨和音频轨，并初始化 Agent. 可以选择是否开启音视频同步.
func NewAgent(config AgentConfig) (*Agent, error) {
	sa := &Agent{
		config: config,
	}
	log.Printf("Driver config: %+v", config.DriverConfig)
	var videoMimeType, audioMimeType string
	switch config.DriverConfig["video_codec"] {
	case "h264":
		videoMimeType = webrtc.MimeTypeH264
	case "h265":
		videoMimeType = webrtc.MimeTypeH265
	case "av1":
		videoMimeType = webrtc.MimeTypeAV1
	default:
		log.Printf("Unsupported video codec: %s", config.DriverConfig["video_codec"])
	}
	switch config.DriverConfig["audio_codec"] {
	case "opus":
		audioMimeType = webrtc.MimeTypeOpus
	default:
		log.Printf("Unsupported audio codec: %s", config.DriverConfig["audio_codec"])
		audioMimeType = webrtc.MimeTypeOpus
	}
	log.Printf("Creating tracks with MIME types - Video: %s, Audio: %s", videoMimeType, audioMimeType)
	streamID := generateStreamID()

	var videoStreamID, audioStreamID string
	if sa.config.AVSync {
		log.Printf("AV Sync enabled: using same StreamID for audio and video")
		videoStreamID = streamID
		audioStreamID = streamID
	} else {
		log.Printf("AV Sync disabled: using different StreamIDs for audio and video")
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

func (sa *Agent) waitFinalPayloadType() error {
	for {
		select {
		case negotiatedCodec := <-sa.negotiatedCodec:
			sa.config.DriverConfig["webrtc_codec"] = fmt.Sprintf("%d||%s||%s", negotiatedCodec.PayloadType, negotiatedCodec.MimeType, negotiatedCodec.SDPFmtpLine)
			return nil
		case <-time.After(10 * time.Second):
			log.Printf("Timeout waiting for video payload type from SDP negotiation")
			return fmt.Errorf("timeout waiting for video payload type")
		}
	}
}

func (sa *Agent) InitDriver() error {
	err := sa.waitFinalPayloadType()
	if err != nil {
		return err
	}
	// finalPayloadType := <-sa.videoPayloadType
	// sa.config.DriverConfig["video_payload_type"] = fmt.Sprintf("%d", finalPayloadType)
	switch sa.config.DeviceType {
	case DEVICE_TYPE_DUMMY:
		// 初始化 Dummy Driver
		dummyDriver, err := dummy.New(sa.config.DriverConfig)
		if err != nil {
			log.Printf("Failed to initialize dummy driver: %v", err)
			return err
		}
		sa.driver = dummyDriver
	case DEVICE_TYPE_ANDROID:
		// 初始化 Android Driver
		androidDriver, err := scrcpy.New(sa.config.DriverConfig, sa.config.DeviceID)
		if err != nil {
			log.Printf("Failed to initialize Android driver: %v", err)
			return err
		}
		sa.driver = androidDriver
	case DEVICE_TYPE_XVFB:
		// 初始化 Linux Driver
		linuxDriver, err := linuxXvfbDriver.New(sa.config.DriverConfig)
		if err != nil {
			log.Printf("Failed to initialize Linux driver: %v", err)
			return err
		}
		sa.driver = linuxDriver
	case DEVICE_TYPE_SUNSHINE:
		sunshine.SSTest()
	default:
		log.Printf("Unsupported device type: %s", sa.config.DeviceType)
		return fmt.Errorf("unsupported device type: %s", sa.config.DeviceType)
	}
	sa.driverCaps = sa.driver.Capabilities()
	// sa.videoCh, sa.audioCh, sa.controlCh = sa.driver.GetReceivers()
	sa.videoCh, sa.audioCh, sa.controlCh = sa.driver.GetReceivers()

	return nil
}

func (sa *Agent) HandleRTCP() {
	rtcpBuf := make([]byte, 1500)
	lastRTCPTime := time.Now()
	for {
		n, _, err := sa.rtpSenderVideo.Read(rtcpBuf)
		if err != nil {
			log.Printf("Error reading RTCP: %v", err)
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
				sa.driver.RequestIDR(false)
			}
		}
	}
}

func (sa *Agent) CreateWebRTCConnection(offer string) string {
	var finalSDP string
	sa.negotiatedCodec = make(chan webrtc.RTPCodecParameters, 1)
	finalSDP = sa.handleSDP(offer)
	return finalSDP
}

func (sa *Agent) Close() {
	log.Printf("Closing agent for device %s", sa.config.DeviceID)
	if sa.driver != nil {
		sa.driver.Stop()
	}
}

func (sa *Agent) GetCodecInfo() (string, string) {
	m := sa.driver.MediaMeta()
	return m.VideoCodec, m.AudioCodec
}

func (sa *Agent) GetMediaMeta() sdriver.MediaMeta {
	return sa.driver.MediaMeta()
}

func (sa *Agent) Capabilities() sdriver.DriverCaps {
	return sa.driver.Capabilities()
}

func (sa *Agent) StartStreaming() {
	sa.driver.Start()
	sa.baseTime = time.Now()
	go sa.StreamingVideo()
	go sa.StreamingAudio()
	if sa.rtpSenderVideo != nil {
		log.Printf("RTCP handler started")
		go sa.HandleRTCP()
	}
	sa.driver.RequestIDR(true)
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
