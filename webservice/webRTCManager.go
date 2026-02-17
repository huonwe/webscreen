package webservice

import (
	"log"
	"sync"

	"github.com/pion/interceptor"
	pionSDP "github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"
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

type WebRTCManager struct {
	sync.RWMutex
	PeerConnections map[string]map[uint32]*webrtc.PeerConnection

	VideoTracks map[string]map[uint32]*webrtc.TrackLocalStaticSample
	AudioTracks map[string]map[uint32]*webrtc.TrackLocalStaticSample

	currentReceiptNumber uint32
}

func NewWebRTCManager() *WebRTCManager {
	return &WebRTCManager{
		PeerConnections:      make(map[string]map[uint32]*webrtc.PeerConnection),
		VideoTracks:          make(map[string]map[uint32]*webrtc.TrackLocalStaticSample),
		AudioTracks:          make(map[string]map[uint32]*webrtc.TrackLocalStaticSample),
		currentReceiptNumber: 0,
	}
}

// 0 -> 1 -> 2 -> 3 -> 0
func (manager *WebRTCManager) AfterReceiptFinished() {
	manager.currentReceiptNumber++
	manager.currentReceiptNumber = manager.currentReceiptNumber % MAX_CLIENTS_PER_DEVICE
}

// 接受设备标识号，客户端的 SDP Offer，返回最终的 SDP Answer，以及对应Connection的ReceiptNo，对应数组中的位置（因为可能有多个浏览器连接同一个设备）。如果发生错误，返回错误信息。
func (manager *WebRTCManager) HandleNewConnection(DeviceIdentifier string, clientSDP string) (string, uint32, error) {
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  clientSDP,
	}
	// log.Println("Handling SDP Offer", sdp)
	// Create MediaEngine
	mimeTypes := []string{}
	if manager.VideoTracks[DeviceIdentifier] != nil {
		for _, track := range manager.VideoTracks[DeviceIdentifier] {
			mimeTypes = append(mimeTypes, track.Codec().MimeType)
		}
	}
	if manager.AudioTracks[DeviceIdentifier] != nil {
		for _, track := range manager.AudioTracks[DeviceIdentifier] {
			mimeTypes = append(mimeTypes, track.Codec().MimeType)
		}
	}
	m := createMediaEngine(mimeTypes)
	if err := m.RegisterHeaderExtension(
		webrtc.RTPHeaderExtensionCapability{URI: pionSDP.TransportCCURI},
		webrtc.RTPCodecTypeVideo,
	); err != nil {
		panic(err)
	}
	// if err := m.RegisterHeaderExtension(
	// 	webrtc.RTPHeaderExtensionCapability{URI: "http://www.webrtc.org/experiments/rtp-hdrext/playout-delay"},
	// 	webrtc.RTPCodecTypeVideo,
	// ); err != nil {
	// 	panic(err)
	// }
	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		panic(err)
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

	// Add Tracks
	if manager.VideoTracks[DeviceIdentifier] != nil {
		_, err = peerConnection.AddTrack(manager.VideoTracks[DeviceIdentifier][0])
		if err != nil {
			log.Println("add Track failed:", err)
		}
	}
	if manager.AudioTracks[DeviceIdentifier] != nil {
		_, err = peerConnection.AddTrack(manager.AudioTracks[DeviceIdentifier][0])
		if err != nil {
			log.Println("add Audio Track failed:", err)
		}
	}

	// peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
	// 	log.Printf("Have DataChannel: Label '%s', ID: %d\n", d.Label(), d.ID())
	// 	switch d.Label() {
	// 	case "control-ordered", "control-unordered":
	// 		d.OnMessage(func(msg webrtc.DataChannelMessage) {
	// 			// log.Printf("DataChannel '%s'-'%d' message: %s\n", d.Label(), d.ID(), string(msg.Data))
	// 			sa.SendEvent(msg.Data)
	// 		})
	// 	default:
	// 		// d.OnMessage(func(msg webrtc.DataChannelMessage) {
	// 		// 	log.Printf("DataChannel '%s'-'%d' message: %s\n", d.Label(), d.ID(), string(msg.Data))
	// 		// 	// sa.SendEvent(msg.Data)
	// 		// })
	// 		log.Printf("Unknown DataChannel label: %s\n", d.Label())
	// 	}
	// })

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

	negotiatedCodecLevel := make(chan webrtc.RTPCodecParameters, 1)
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("webrtc Connection State: %s", s)
		if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
			// Do some cleanup, like removing references
			peerConnection.Close()

		}
		if s == webrtc.PeerConnectionStateConnected {
			for _, sender := range peerConnection.GetSenders() {
				if sender.Track() == nil {
					continue
				}
				if sender.Track().Kind() != webrtc.RTPCodecTypeVideo {
					continue
				}
				params := sender.GetParameters()
				selectedCodec := params.Codecs[0] // 通常只有一个活跃的 codec
				log.Printf("Negotiation result: %v", selectedCodec)
				// 根据 PayloadType 决定 scrcpy 参数
				negotiatedCodecLevel <- selectedCodec
				break
			}
		}
	})

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
	receiptNo := manager.currentReceiptNumber
	manager.Lock()
	manager.PeerConnections[DeviceIdentifier][receiptNo] = peerConnection
	manager.VideoTracks[DeviceIdentifier][receiptNo] = nil // 占位，实际 Track 由 Agent 创建后替换
	manager.AudioTracks[DeviceIdentifier][receiptNo] = nil // 占位，实际 Track 由 Agent 创建后替换
	manager.Unlock()
	defer manager.AfterReceiptFinished()
	return finalSDP, receiptNo, nil
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
