package sagent

import (
	"log"

	"github.com/pion/interceptor"
	pionSDP "github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"
)

func HandleSDP(sdp string, vTrack *webrtc.TrackLocalStaticSample, aTrack *webrtc.TrackLocalStaticSample) (string, *webrtc.RTPSender, *webrtc.RTPSender) {
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}

	// 创建 MediaEngine
	mimeTypes := []string{}
	if vTrack != nil {
		mimeTypes = append(mimeTypes, vTrack.Codec().MimeType)
	}
	if aTrack != nil {
		mimeTypes = append(mimeTypes, aTrack.Codec().MimeType)
	}
	m := CreateMediaEngine(mimeTypes)
	if err := m.RegisterHeaderExtension(
		webrtc.RTPHeaderExtensionCapability{URI: pionSDP.TransportCCURI},
		webrtc.RTPCodecTypeVideo,
	); err != nil {
		panic(err)
	}
	if err := m.RegisterHeaderExtension(
		webrtc.RTPHeaderExtensionCapability{URI: "http://www.webrtc.org/experiments/rtp-hdrext/playout-delay"},
		webrtc.RTPCodecTypeVideo,
	); err != nil {
		panic(err)
	}
	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		panic(err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))
	// 配置 ICE 服务器 (STUN)，用于穿透 NAT
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}
	// 创建 PeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		log.Println("创建 PeerConnection 失败:", err)
		return "", nil, nil
	}

	var rtpSenderVideo *webrtc.RTPSender
	var rtpSenderAudio *webrtc.RTPSender
	// C. 添加视频轨道 (Video Track)
	if vTrack != nil {
		rtpSenderVideo, err = peerConnection.AddTrack(vTrack)
		if err != nil {
			log.Println("添加 Track 失败:", err)
			rtpSenderVideo = nil
		}
	}
	// 添加音频轨道 (Audio Track)
	if aTrack != nil {
		rtpSenderAudio, err = peerConnection.AddTrack(aTrack)
		if err != nil {
			log.Println("添加 Audio Track 失败:", err)
			rtpSenderAudio = nil
		}
	}
	// D. 设置 Remote Description (浏览器发来的 Offer)
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		log.Println("设置 Remote Description 失败:", err)
		return "", nil, nil
	}

	// E. 创建 Answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Println("创建 Answer 失败:", err)
		return "", nil, nil
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
		return "", nil, nil
	}

	// 阻塞等待 ICE 收集完成 (通常几百毫秒)
	<-gatherComplete
	finalSDP := peerConnection.LocalDescription().SDP
	return finalSDP, rtpSenderVideo, rtpSenderAudio
}

func CreateMediaEngine(mimeTypes []string) *webrtc.MediaEngine {
	m := &webrtc.MediaEngine{}

	// m.RegisterDefaultCodecs()
	// return m

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
				PayloadType: 100,
			}, webrtc.RTPCodecTypeVideo)
			if err != nil {
				log.Println("RegisterCodec AV1 failed:", err)
			}
			log.Println("Registered AV1 codec")
		case webrtc.MimeTypeH265:
			// Register H.265 (video)
			err := m.RegisterCodec(webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:    webrtc.MimeTypeH265,
					ClockRate:   90000,
					Channels:    0,
					SDPFmtpLine: "profile-id=1;tier-flag=0;level-id=153;level-asymmetry-allowed=1",
					RTCPFeedback: []webrtc.RTCPFeedback{
						{Type: "transport-cc", Parameter: ""},
						{Type: "ccm", Parameter: "fir"},
						{Type: "nack", Parameter: ""},
						{Type: "nack", Parameter: "pli"},
					},
				},
				PayloadType: 102,
			}, webrtc.RTPCodecTypeVideo)
			if err != nil {
				log.Println("RegisterCodec H265 failed:", err)
			}
			err = m.RegisterCodec(webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:    webrtc.MimeTypeH265,
					ClockRate:   90000,
					Channels:    0,
					SDPFmtpLine: "profile-id=1;tier-flag=0;level-id=123;level-asymmetry-allowed=1",
					RTCPFeedback: []webrtc.RTCPFeedback{
						{Type: "transport-cc", Parameter: ""},
						{Type: "ccm", Parameter: "fir"},
						{Type: "nack", Parameter: ""},
						{Type: "nack", Parameter: "pli"},
					},
				},
				PayloadType: 103,
			}, webrtc.RTPCodecTypeVideo)
			if err != nil {
				log.Println("RegisterCodec H265 failed:", err)
			}
			log.Println("Registered H265 codec")
		case webrtc.MimeTypeH264:
			err := m.RegisterCodec(webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:  webrtc.MimeTypeH264,
					ClockRate: 90000,
					Channels:  0,
					// profile-level-id 解析:
					// 64: High Profile (0x64)
					// 00: Constraint Set (默认)
					// 33: Level 5.1 (5.1 * 10 = 51 = 0x33)
					// packetization-mode=1: 支持非交错模式
					SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640033",
					RTCPFeedback: []webrtc.RTCPFeedback{
						{Type: "transport-cc", Parameter: ""},
						{Type: "ccm", Parameter: "fir"},
						{Type: "nack", Parameter: ""},
						{Type: "nack", Parameter: "pli"},
					},
				},
				PayloadType: 104, // 优先级最高
			}, webrtc.RTPCodecTypeVideo)
			err = m.RegisterCodec(webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:  webrtc.MimeTypeH264,
					ClockRate: 90000,
					Channels:  0,
					// profile-level-id 解析:
					// 4d: Main Profile (0x4d)
					// e0: Constraint Set (Constrained Baseline)
					// 1f: Level 4.2 (4.2 * 10 = 42 = 0x2a)
					SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=4de02a",
					RTCPFeedback: []webrtc.RTCPFeedback{
						{Type: "transport-cc", Parameter: ""},
						{Type: "ccm", Parameter: "fir"},
						{Type: "nack", Parameter: ""},
						{Type: "nack", Parameter: "pli"},
					},
				},
				PayloadType: 105, // 备选
			}, webrtc.RTPCodecTypeVideo)
			if err != nil {
				log.Println("RegisterCodec H264 failed:", err)
			}
			err = m.RegisterCodec(webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:  webrtc.MimeTypeH264,
					ClockRate: 90000,
					Channels:  0,
					// profile-level-id 解析:
					// 42: Baseline Profile (0x42)
					// e0: Constraint Set (Constrained Baseline)
					// 1f: Level 3.1 (3.1 * 10 = 31 = 0x1f)
					SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
					RTCPFeedback: []webrtc.RTCPFeedback{
						{Type: "transport-cc", Parameter: ""},
						{Type: "ccm", Parameter: "fir"},
						{Type: "nack", Parameter: ""},
						{Type: "nack", Parameter: "pli"},
					},
				},
				PayloadType: 106, // 保底
			}, webrtc.RTPCodecTypeVideo)
			if err != nil {
				log.Println("RegisterCodec H264 failed:", err)
			}
			log.Println("Registered H264 codec")

		case webrtc.MimeTypeOpus:
			// 1. 注册 Opus (音频)
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
