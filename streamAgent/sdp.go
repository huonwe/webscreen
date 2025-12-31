package sagent

import (
	"log"

	"github.com/pion/interceptor"
	pionSDP "github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"
)

func (sa *Agent) handleSDP(sdp string) string {
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}
	// log.Println("Handling SDP Offer", sdp)
	// Create MediaEngine
	mimeTypes := []string{}
	if sa.VideoTrack != nil {
		mimeTypes = append(mimeTypes, sa.VideoTrack.Codec().MimeType)
	}
	if sa.AudioTrack != nil {
		mimeTypes = append(mimeTypes, sa.AudioTrack.Codec().MimeType)
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
		return ""
	}

	var rtpSenderVideo *webrtc.RTPSender
	var rtpSenderAudio *webrtc.RTPSender
	// Add Tracks
	if sa.VideoTrack != nil {
		rtpSenderVideo, err = peerConnection.AddTrack(sa.VideoTrack)
		if err != nil {
			log.Println("add Track failed:", err)
			rtpSenderVideo = nil
		}
	}
	if sa.AudioTrack != nil {
		rtpSenderAudio, err = peerConnection.AddTrack(sa.AudioTrack)
		if err != nil {
			log.Println("add Audio Track failed:", err)
			rtpSenderAudio = nil
		}
	}
	sa.rtpSenderVideo = rtpSenderVideo
	sa.rtpSenderAudio = rtpSenderAudio
	// Set Remote Description (Offer from browser)
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		log.Println("set Remote Description failed:", err)
		return ""
	}

	// Create Answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Println("Create Answer failed:", err)
		return ""
	}
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
				sa.negotiatedCodec <- selectedCodec
				close(sa.negotiatedCodec)
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
		return ""
	}

	// 阻塞等待 ICE 收集完成 (通常几百毫秒)
	<-gatherComplete
	finalSDP := peerConnection.LocalDescription().SDP
	return finalSDP
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

// SetSDPBandwidth 在 SDP 的 video m-line 后插入 b=AS:20000 (20Mbps)
// func SetSDPBandwidth(sdp string, bandwidth int) string {
// 	lines := strings.Split(sdp, "\r\n")
// 	var newLines []string
// 	for _, line := range lines {
// 		newLines = append(newLines, line)
// 		if strings.HasPrefix(line, "m=video") {
// 			// b=AS:<bandwidth>  (Application Specific Maximum, 单位 kbps)
// 			// 设置为 20000 kbps = 20 Mbps，远超默认的 2.5 Mbps
// 			newLines = append(newLines, fmt.Sprintf("b=AS:%d", bandwidth))
// 			// 也可以加上 TIAS (Transport Independent Application Specific Maximum, 单位 bps)
// 			// newLines = append(newLines, "b=TIAS:20000000")
// 		}
// 	}
// 	return strings.Join(newLines, "\r\n")
// }

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
