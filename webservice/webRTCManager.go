package webservice

import (
	"fmt"
	"log"
	"sync"
	"time"
	sagent "webscreen/streamAgent"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	pionSDP "github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

const (
	PAYLOAD_TYPE_AV1_PROFILE_MAIN_5_1            = 100 // 2560x1440 @ 60fps
	PAYLOAD_TYPE_H265_PROFILE_MAIN_TIER_MAIN_5_1 = 102 // 2560x1440 @ 60fps 40Mbps Max
	PAYLOAD_TYPE_H265_PROFILE_MAIN_TIER_MAIN_4_1 = 103 // 1920x1080 @ 60fps 20Mbps Max
	PAYLOAD_TYPE_H264_PROFILE_HIGH_5_1           = 104 // 2560x1440 @ 60fps
	PAYLOAD_TYPE_H264_PROFILE_HIGH_5_1_0C        = 105 // 2560x1440 @ 60fps for iphone safari
	PAYLOAD_TYPE_H264_PROFILE_BASELINE_3_1       = 106 // 720p @ 30fps
	PAYLOAD_TYPE_H264_PROFILE_BASELINE_3_1_0C    = 107 // 720p @ 30fps for iphone safari
)

const (
	MAX_CLIENTS_PER_DEVICE = 4
)

type Subscriber struct {
	PeerConnection       *webrtc.PeerConnection
	dataChannelUnordered *webrtc.DataChannel
	dataChannelOrdered   *webrtc.DataChannel
	dataChannelReady     bool
	rtpSenderVideo       *webrtc.RTPSender
	rtpSenderAudio       *webrtc.RTPSender
	videoTrack           *webrtc.TrackLocalStaticSample
	audioTrack           *webrtc.TrackLocalStaticSample
	videoChan            chan media.Sample
	audioChan            chan media.Sample
	eventChan            chan []byte // sdriver -> Agent -> WebMaster -> 前端
}

type WebRTCManager struct {
	sync.RWMutex
	subscribers map[string]map[uint32]*Subscriber // deviceIdentifier -> ReceiptNo -> Subscriber
	agents      map[string]*sagent.Agent

	currentReceiptNumber map[string]uint32
}

func NewWebRTCManager() *WebRTCManager {
	wm := &WebRTCManager{
		subscribers:          make(map[string]map[uint32]*Subscriber),
		agents:               make(map[string]*sagent.Agent),
		currentReceiptNumber: make(map[string]uint32),
	}
	go func() {
		for {
			time.Sleep(30 * time.Second)
			log.Printf("WebRTCManager status: %d devices, %d agents\n", len(wm.subscribers), len(wm.agents))
			for deviceID, subs := range wm.subscribers {
				log.Printf("Device %s has %d subscribers\n", deviceID, len(subs))
				for receiptNo, sub := range subs {
					log.Printf("Device %s, ReceiptNo %d, Subscriber: %+v\n", deviceID, receiptNo, sub)
					if sub.PeerConnection.ConnectionState() == webrtc.PeerConnectionStateFailed {
						log.Printf("Removing failed subscriber for device %s, receiptNo %d", deviceID, receiptNo)
						wm.Lock()
						delete(wm.subscribers[deviceID], receiptNo)
						wm.Unlock()
					}
				}
			}
		}
	}()
	return wm
}

// 接受设备标识号，客户端的 SDP Offer，返回最终的 SDP Answer，以及对应Connection的ReceiptNo，对应数组中的位置（因为可能有多个浏览器连接同一个设备）。如果发生错误，返回错误信息。
// 不添加 Track
func (manager *WebRTCManager) NewSubscriber(deviceIdentifier string, clientSDP string, AgentConfig sagent.AgentConfig) (string, uint32, error) {
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  clientSDP,
	}
	// log.Println("Handling SDP Offer", sdp)
	videoMimeType, audioMimeType := getMimeTypeFromConfig(AgentConfig)
	// Create MediaEngine
	mimeTypes := []string{videoMimeType, audioMimeType}
	m := createMediaEngine(mimeTypes)
	if err := m.RegisterHeaderExtension(
		webrtc.RTPHeaderExtensionCapability{URI: pionSDP.TransportCCURI},
		webrtc.RTPCodecTypeVideo,
	); err != nil {
		log.Printf("RegisterHeaderExtension failed: %v", err)
		return "", 0, err
	}
	// if err := m.RegisterHeaderExtension(
	// 	webrtc.RTPHeaderExtensionCapability{URI: "http://www.webrtc.org/experiments/rtp-hdrext/playout-delay"},
	// 	webrtc.RTPCodecTypeVideo,
	// ); err != nil {
	// 	panic(err)
	// }
	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		log.Printf("RegisterDefaultInterceptors failed: %v", err)
		return "", 0, err
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))
	// Configure ICE servers (STUN) for NAT traversal
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}
	// Create PeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		log.Println("Create PeerConnection failed:", err)
		return "", 0, err
	}
	videoTrack, audioTrack := createAVTrack(videoMimeType, audioMimeType, AgentConfig)
	if videoTrack == nil && audioTrack == nil {
		log.Printf("Failed to create both video and audio tracks")
		return "", 0, fmt.Errorf("failed to create media tracks")
	}
	rtpSenderVideo, err := peerConnection.AddTrack(videoTrack)
	if err != nil {
		log.Printf("Failed to add video track: %v", err)
		return "", 0, err
	}
	rtpSenderAudio, err := peerConnection.AddTrack(audioTrack)
	if err != nil {
		log.Printf("Failed to add audio track: %v", err)
		return "", 0, err
	}

	// Set Remote Description (Offer from browser)
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		log.Println("set Remote Description failed:", err)
		return "", 0, err
	}

	// Create Answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Println("Create Answer failed:", err)
		return "", 0, err
	}

	// 设置 Local Description 并等待 ICE 收集完成
	// 这一步是为了生成一个包含所有网络路径信息的完整 SDP，
	// 这样我们就不需要写复杂的 Trickle ICE 逻辑了。
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	if err := peerConnection.SetLocalDescription(answer); err != nil {
		log.Println("Set Local Description failed:", err)
		return "", 0, err
	}

	// 阻塞等待 ICE 收集完成 (通常几百毫秒)
	<-gatherComplete
	finalSDP := peerConnection.LocalDescription().SDP
	manager.Lock()
	receiptNo := manager.currentReceiptNumber[deviceIdentifier]
	if manager.subscribers[deviceIdentifier] == nil {
		manager.subscribers[deviceIdentifier] = make(map[uint32]*Subscriber)
	}
	if manager.subscribers[deviceIdentifier][receiptNo] != nil {
		log.Printf("Warning: Overwriting existing subscriber for device %s, receiptNo %d", deviceIdentifier, receiptNo)
		manager.subscribers[deviceIdentifier][receiptNo].PeerConnection.Close()
	}
	sub := &Subscriber{
		PeerConnection:   peerConnection,
		dataChannelReady: false,
		rtpSenderVideo:   rtpSenderVideo,
		rtpSenderAudio:   rtpSenderAudio,
		videoTrack:       videoTrack,
		audioTrack:       audioTrack,
		videoChan:        make(chan media.Sample, 30),
		audioChan:        make(chan media.Sample, 30),
		eventChan:        make(chan []byte, 30),
	}
	manager.subscribers[deviceIdentifier][receiptNo] = sub
	manager.currentReceiptNumber[deviceIdentifier] = (manager.currentReceiptNumber[deviceIdentifier] + 1) % MAX_CLIENTS_PER_DEVICE
	manager.Unlock()

	sub.setDataChannel()

	return finalSDP, receiptNo, nil
}

func (manager *WebRTCManager) Start(deviceIdentifier string, receiptNo uint32, agentConfig sagent.AgentConfig) error {
	err := manager.ensureAgent(deviceIdentifier, receiptNo, agentConfig)
	if err != nil {
		log.Printf("Failed to ensure agent for device %s: %v", deviceIdentifier, err)
		return err
	}
	agent := manager.agents[deviceIdentifier]
	sub := manager.subscribers[deviceIdentifier][receiptNo]
	go ListenRTPVideo(sub.rtpSenderVideo, manager.agents[deviceIdentifier])
	go ListenRTPAudio(sub.rtpSenderAudio, manager.agents[deviceIdentifier])
	manager.setCleanup(sub.PeerConnection, deviceIdentifier, receiptNo)

	// log.Println("Sub: ", sub)
	sub.setDataChannelCallback(agent.SendEvent)
	sub.startPushAVSample()
	sub.startPushEvent()
	return nil
}

func (manager *WebRTCManager) ensureAgent(deviceIdentifier string, receiptNo uint32, agentConfig sagent.AgentConfig) error {
	manager.Lock()
	defer manager.Unlock()
	agent, exists := manager.agents[deviceIdentifier]
	if !exists {
		agent = sagent.New(agentConfig)
		finalCodec, err := WaitAndGetFinalCodecParams(manager.subscribers[deviceIdentifier][receiptNo].PeerConnection)
		if err != nil {
			log.Printf("Failed to get final codec parameters for device %s: %v", deviceIdentifier, err)
			return err
		}
		manager.agents[deviceIdentifier] = agent

		agent.InitDriver(finalCodec)
		agent.Start()
		manager.startBroadcastAVToSubscribers(deviceIdentifier)
		manager.startBroadcastEventToSubscribers(deviceIdentifier)
	}

	return nil
}

func (manager *WebRTCManager) GetAgent(deviceIdentifier string) (*sagent.Agent, bool) {
	manager.RLock()
	defer manager.RUnlock()
	agent, exists := manager.agents[deviceIdentifier]
	return agent, exists
}

func createMediaEngine(mimeTypes []string) *webrtc.MediaEngine {
	m := &webrtc.MediaEngine{}
	for _, mime := range mimeTypes {
		switch mime {
		case webrtc.MimeTypeAV1:
			err := m.RegisterCodec(webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:  webrtc.MimeTypeAV1,
					ClockRate: 90000,
					Channels:  0,
					// profile=0 (Main Profile), level-idx=13 (Level 5.1), tier=0 (Main Tier)
					SDPFmtpLine: "profile=0;level-idx=13;tier=0",
					RTCPFeedback: []webrtc.RTCPFeedback{
						{Type: "transport-cc", Parameter: ""},
						{Type: "ccm", Parameter: "fir"},
						{Type: "nack", Parameter: ""},
						{Type: "nack", Parameter: "pli"},
					},
				},
				PayloadType: PAYLOAD_TYPE_AV1_PROFILE_MAIN_5_1,
			}, webrtc.RTPCodecTypeVideo)
			if err != nil {
				log.Println("RegisterCodec AV1 failed:", err)
			}
			log.Println("Registered AV1 codec")
		case webrtc.MimeTypeH265:
			batchRegisterCodecH265(m)
		case webrtc.MimeTypeH264:
			batchRegisterCodecH264(m)
		case webrtc.MimeTypeOpus:
			// Register Opus (Audio)
			err := m.RegisterCodec(webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:  webrtc.MimeTypeOpus,
					ClockRate: 48000,
					Channels:  2,
					// force 10ms low latency, but not working well, so disable it
					// force stereo (spatial audio)
					// enable FEC (forward error correction)
					// disable DTX (discontinuous transmission) (usedtx=0)
					SDPFmtpLine: "minptime=10;maxptime=20;useinbandfec=1;stereo=1;sprop-stereo=1",
				},
				PayloadType: 111,
			}, webrtc.RTPCodecTypeAudio)
			if err != nil {
				log.Println("RegisterCodec Opus failed:", err)
			}
		default:
			log.Printf("Unsupported MIME type: %s", mime)
		}

	}
	return m
}

func batchRegisterCodecH264(m *webrtc.MediaEngine) {
	// profile-level-id :
	// High Profile (0x64) 4d: Main Profile (0x4d) 42: Baseline Profile (0x42)
	// Constraint Set (00) Constrained Baseline (e0)
	// Level 5.1 (5.1 * 10 = 51 = 0x33) Level 4.2 (4.2 * 10 = 42 = 0x2a) Level 3.1 (3.1 * 10 = 31 = 0x1f)
	// packetization-mode=1: 支持非交错模式
	// high profile
	err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			Channels:    0,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640033",
			RTCPFeedback: []webrtc.RTCPFeedback{
				{Type: "transport-cc", Parameter: ""},
				{Type: "ccm", Parameter: "fir"},
				{Type: "nack", Parameter: ""},
				{Type: "nack", Parameter: "pli"},
			},
		},
		PayloadType: PAYLOAD_TYPE_H264_PROFILE_HIGH_5_1,
	}, webrtc.RTPCodecTypeVideo)
	// high profile for iphone safari
	err = m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			Channels:    0,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640c33",
			RTCPFeedback: []webrtc.RTCPFeedback{
				{Type: "transport-cc", Parameter: ""},
				{Type: "ccm", Parameter: "fir"},
				{Type: "nack", Parameter: ""},
				{Type: "nack", Parameter: "pli"},
			},
		},
		PayloadType: PAYLOAD_TYPE_H264_PROFILE_HIGH_5_1_0C,
	}, webrtc.RTPCodecTypeVideo)
	if err != nil {
		log.Println("RegisterCodec H264 failed:", err)
	}
	// Baseline Profile
	// err = m.RegisterCodec(webrtc.RTPCodecParameters{
	// 	RTPCodecCapability: webrtc.RTPCodecCapability{
	// 		MimeType:    webrtc.MimeTypeH264,
	// 		ClockRate:   90000,
	// 		Channels:    0,
	// 		SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
	// 		RTCPFeedback: []webrtc.RTCPFeedback{
	// 			{Type: "transport-cc", Parameter: ""},
	// 			{Type: "ccm", Parameter: "fir"},
	// 			{Type: "nack", Parameter: ""},
	// 			{Type: "nack", Parameter: "pli"},
	// 		},
	// 	},
	// 	PayloadType: PAYLOAD_TYPE_H264_PROFILE_BASELINE_3_1,
	// }, webrtc.RTPCodecTypeVideo)
	// if err != nil {
	// 	log.Println("RegisterCodec H264 failed:", err)
	// }
	// baseline profile for iphone safari
	// err = m.RegisterCodec(webrtc.RTPCodecParameters{
	// 	RTPCodecCapability: webrtc.RTPCodecCapability{
	// 		MimeType:    webrtc.MimeTypeH264,
	// 		ClockRate:   90000,
	// 		Channels:    0,
	// 		SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=420c1f",
	// 		RTCPFeedback: []webrtc.RTCPFeedback{
	// 			{Type: "transport-cc", Parameter: ""},
	// 			{Type: "ccm", Parameter: "fir"},
	// 			{Type: "nack", Parameter: ""},
	// 			{Type: "nack", Parameter: "pli"},
	// 		},
	// 	},
	// 	PayloadType: PAYLOAD_TYPE_H264_PROFILE_BASELINE_3_1_0C,
	// }, webrtc.RTPCodecTypeVideo)
	// if err != nil {
	// 	log.Println("RegisterCodec H264 failed:", err)
	// }
	// log.Println("Registered H264 codec")
}

func batchRegisterCodecH265(m *webrtc.MediaEngine) {
	// Register H.265 (video)
	err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH265,
			ClockRate:   90000,
			Channels:    0,
			SDPFmtpLine: "profile-id=1;tier-flag=0;level-id=153",
			RTCPFeedback: []webrtc.RTCPFeedback{
				{Type: "transport-cc", Parameter: ""},
				{Type: "ccm", Parameter: "fir"},
				{Type: "nack", Parameter: ""},
				{Type: "nack", Parameter: "pli"},
			},
		},
		PayloadType: PAYLOAD_TYPE_H265_PROFILE_MAIN_TIER_MAIN_5_1,
	}, webrtc.RTPCodecTypeVideo)
	if err != nil {
		log.Println("RegisterCodec H265 failed:", err)
	}
	err = m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH265,
			ClockRate:   90000,
			Channels:    0,
			SDPFmtpLine: "profile-id=1;tier-flag=0;level-id=123",
			RTCPFeedback: []webrtc.RTCPFeedback{
				{Type: "transport-cc", Parameter: ""},
				{Type: "ccm", Parameter: "fir"},
				{Type: "nack", Parameter: ""},
				{Type: "nack", Parameter: "pli"},
			},
		},
		PayloadType: PAYLOAD_TYPE_H265_PROFILE_MAIN_TIER_MAIN_4_1,
	}, webrtc.RTPCodecTypeVideo)
	if err != nil {
		log.Println("RegisterCodec H265 failed:", err)
	}
	log.Println("Registered H265 codec")
}

func WaitAndGetFinalCodecParams(pc *webrtc.PeerConnection) (webrtc.RTPCodecParameters, error) {
	// startTime := time.Now()
	for {
		if pc.ConnectionState() == webrtc.PeerConnectionStateFailed {
			return webrtc.RTPCodecParameters{}, fmt.Errorf("peer connection failed")
		}
		if pc.ConnectionState() == webrtc.PeerConnectionStateClosed {
			return webrtc.RTPCodecParameters{}, fmt.Errorf("peer connection closed")
		}
		if pc.ConnectionState() == webrtc.PeerConnectionStateConnected {
			for _, sender := range pc.GetSenders() {
				if sender.Track() == nil {
					continue
				}
				if sender.Track().Kind() != webrtc.RTPCodecTypeVideo {
					continue
				}
				params := sender.GetParameters()
				selectedCodec := params.Codecs[0] // 通常只有一个活跃的 codec
				// log.Printf("Negotiation result: %v", selectedCodec)
				// 根据 PayloadType 决定 scrcpy 参数
				return selectedCodec, nil
			}
		}
		// if time.Since(startTime) > 10*time.Second {
		// 	return webrtc.RTPCodecParameters{}, fmt.Errorf("timeout waiting for final codec parameters")
		// }
		time.Sleep(500 * time.Millisecond)
	}
}

func getMimeTypeFromConfig(config sagent.AgentConfig) (string, string) {
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
	return videoMimeType, audioMimeType
}

func createAVTrack(videoMimeType, audioMimeType string, config sagent.AgentConfig) (*webrtc.TrackLocalStaticSample, *webrtc.TrackLocalStaticSample) {
	trackID := fmt.Sprintf("%s-%s", "webscreen-track", randomString(8))
	trackIDVideo := trackID + "-" + config.DeviceID + "-video"
	trackIDAudio := trackID + "-" + config.DeviceID + "-audio"
	streamID := fmt.Sprintf("%s-%s", "webscreen-stream", randomString(8))
	streamIDVideo := streamID + "-" + config.DeviceID + "-video"
	streamIDAudio := streamID + "-" + config.DeviceID + "-audio"
	if !config.AVSync {
		streamIDVideo = streamID + "-" + config.DeviceID
		streamIDAudio = streamID + "-" + config.DeviceID
	}

	trackVideo, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: videoMimeType}, trackIDVideo, streamIDVideo)
	if err != nil {
		log.Printf("Failed to create track for MIME type %s: %v", videoMimeType, err)
	}
	trackAudio, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: audioMimeType}, trackIDAudio, streamIDAudio)
	if err != nil {
		log.Printf("Failed to create audio track for MIME type %s: %v", audioMimeType, err)
	}
	return trackVideo, trackAudio
}

func (sub *Subscriber) setDataChannel() {
	sub.PeerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		log.Printf("Have DataChannel: Label '%s', ID: %d\n", d.Label(), d.ID())
		switch d.Label() {
		case "control-ordered":
			sub.dataChannelOrdered = d
		case "control-unordered":
			sub.dataChannelUnordered = d
		default:
			d.OnMessage(func(msg webrtc.DataChannelMessage) {
				log.Printf("DataChannel '%s'-'%d' message: %s\n", d.Label(), d.ID(), string(msg.Data))
			})
			log.Printf("Unknown DataChannel label: %s\n", d.Label())
		}
		sub.dataChannelReady = true
	})

}

func (sub *Subscriber) setDataChannelCallback(callback func([]byte) error) {
	go func() {
		for {
			if sub.dataChannelReady {
				sub.dataChannelOrdered.OnMessage(func(msg webrtc.DataChannelMessage) {
					// log.Printf("DataChannel '%s'-'%d' message: %s\n", sub.dataChannelOrdered.Label(), sub.dataChannelOrdered.ID(), string(msg.Data))
					if err := callback(msg.Data); err != nil {
						log.Printf("Error handling data channel message: %v", err)
					}
				})
				sub.dataChannelUnordered.OnMessage(func(msg webrtc.DataChannelMessage) {
					// log.Printf("DataChannel '%s'-'%d' message: %s\n", sub.dataChannelUnordered.Label(), sub.dataChannelUnordered.ID(), string(msg.Data))
					if err := callback(msg.Data); err != nil {
						log.Printf("Error handling data channel message: %v", err)
					}
				})
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

}

func (manager *WebRTCManager) setCleanup(pc *webrtc.PeerConnection, deviceIdentifier string, receiptNo uint32) {
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("PeerConnection state changed: %s\n", state.String())
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			log.Printf("PeerConnection is in state %s, cleaning up resources\n", state.String())
			pc.Close()
			manager.Lock()
			delete(manager.subscribers[deviceIdentifier], receiptNo)
			if len(manager.subscribers[deviceIdentifier]) == 0 {
				manager.agents[deviceIdentifier].Close()
				delete(manager.agents, deviceIdentifier)
				delete(manager.subscribers, deviceIdentifier)
				delete(manager.currentReceiptNumber, deviceIdentifier)
			}
			manager.Unlock()
		}
	})
}

func ListenRTPVideo(rtpSender *webrtc.RTPSender, agent *sagent.Agent) {
	rtcpBuf := make([]byte, 1500)
	for {
		n, _, err := rtpSender.Read(rtcpBuf)
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
				log.Println("IDR requested via RTCP PLI")
				agent.PLIRequest()
			}
		}
	}
}

func ListenRTPAudio(rtpSender *webrtc.RTPSender, agent *sagent.Agent) {
	rtcpBuf := make([]byte, 1500)
	for {
		n, _, err := rtpSender.Read(rtcpBuf)
		if err != nil {
			log.Printf("Error reading RTCP: %v", err)
			return
		}
		packets, err := rtcp.Unmarshal(rtcpBuf[:n])
		if err != nil {
			continue
		}
		for _, p := range packets {
			log.Printf("Received RTCP packet on audio track: %T\n", p)
			// 目前不处理音频相关的 RTCP 包
		}
	}
}

func (manager *WebRTCManager) startBroadcastAVToSubscribers(deviceIdentifier string) {
	agent := manager.agents[deviceIdentifier]
	go func() {
		for videoSample := range agent.VideoStream() {
			manager.RLock()
			for _, sub := range manager.subscribers[deviceIdentifier] {
				select {
				case sub.videoChan <- videoSample:
				default:
					log.Printf("Dropping video sample for subscriber (device %s) due to full channel buffer", deviceIdentifier)
				}
			}
			manager.RUnlock()
		}
	}()
	go func() {
		for audioSample := range agent.AudioStream() {
			manager.RLock()
			for _, sub := range manager.subscribers[deviceIdentifier] {
				select {
				case sub.audioChan <- audioSample:
				default:
					// log.Printf("Dropping audio sample for subscriber (device %s) due to full channel buffer", deviceIdentifier)
				}
			}
			manager.RUnlock()
		}
	}()
}

func (sub *Subscriber) startPushAVSample() {
	go func() {
		for videoSample := range sub.videoChan {
			if sub.videoTrack != nil {
				if err := sub.videoTrack.WriteSample(videoSample); err != nil {
					log.Printf("Error writing video sample to track: %v", err)
				}
			}
		}
	}()
	go func() {
		for audioSample := range sub.audioChan {
			if sub.audioTrack != nil {
				if err := sub.audioTrack.WriteSample(audioSample); err != nil {
					log.Printf("Error writing audio sample to track: %v", err)
				}
			}
		}
	}()
}

func (manager *WebRTCManager) startBroadcastEventToSubscribers(deviceIdentifier string) {
	go func() {
		for event := range manager.agents[deviceIdentifier].FeedbackEvents() {
			manager.RLock()
			for _, sub := range manager.subscribers[deviceIdentifier] {
				select {
				case sub.eventChan <- event:
				default:
					log.Printf("Dropping event for subscriber (device %s) due to full channel buffer", deviceIdentifier)
				}
			}
			manager.RUnlock()
		}
	}()
}

func (sub *Subscriber) startPushEvent() {
	go func() {
		for event := range sub.eventChan {
			if sub.dataChannelOrdered != nil {
				err := sub.dataChannelUnordered.Send(event)
				if err != nil {
					log.Printf("Error sending event on unordered data channel: %v", err)
				}
			}
		}
	}()
}
