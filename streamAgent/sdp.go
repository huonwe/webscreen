package sagent

import (
	"fmt"
	"log"
	"strings"

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
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))
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
	// log.Println("Final Server SDP:", peerConnection.LocalDescription().SDP)
	// G. 将最终的 SDP Answer 返回给浏览器

	// 手动修改 SDP 以突破浏览器默认带宽限制 (SDP Munging)
	// 这里先不改，在函数外面改
	// finalSDP := setSDPBandwidth(peerConnection.LocalDescription().SDP, 20000) // 20 Mbps
	finalSDP := peerConnection.LocalDescription().SDP
	return finalSDP, rtpSenderVideo, rtpSenderAudio
}

func CreateMediaEngine(mimeTypes []string) *webrtc.MediaEngine {
	m := &webrtc.MediaEngine{}

	// m.RegisterDefaultCodecs()
	// return m

	for _, mime := range mimeTypes {
		switch mime {
		case webrtc.MimeTypeOpus:
			// 1. 注册 Opus (音频)
			err := m.RegisterCodec(webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10;useinbandfec=1"},
				PayloadType:        111,
			}, webrtc.RTPCodecTypeAudio)
			if err != nil {
				log.Println("RegisterCodec Opus failed:", err)
			}
		case webrtc.MimeTypeH264:
			// 注册 H.264 (视频) - 即使我们想用 H.265，注册 H.264 也是个好习惯，作为 fallback
			err := m.RegisterCodec(webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:     webrtc.MimeTypeH264,
					ClockRate:    90000,
					Channels:     0,
					SDPFmtpLine:  "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
					RTCPFeedback: []webrtc.RTCPFeedback{{Type: "goog-remb", Parameter: ""}, {Type: "ccm", Parameter: "fir"}, {Type: "nack", Parameter: "pli"}}, // 显式禁用 Generic NACK，只保留 PLI
				},
				PayloadType: 102,
			}, webrtc.RTPCodecTypeVideo)
			if err != nil {
				log.Println("RegisterCodec H264 failed:", err)
			}
			log.Println("Registered H264 codec")
		case webrtc.MimeTypeH265:
			// 注册 H.265 (视频)
			err := m.RegisterCodec(webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:     webrtc.MimeTypeH265,
					ClockRate:    90000,
					Channels:     0,
					SDPFmtpLine:  "",                                                                                                                           // H.265 通常不需要复杂的 fmtp，或者可以留空让 Pion 处理
					RTCPFeedback: []webrtc.RTCPFeedback{{Type: "goog-remb", Parameter: ""}, {Type: "ccm", Parameter: "fir"}, {Type: "nack", Parameter: "pli"}}, // 显式禁用 Generic NACK，只保留 PLI
				},
				PayloadType: 104, // 使用 104，避开 Offer 中的 49/51 和 H.264 的 102
			}, webrtc.RTPCodecTypeVideo)
			if err != nil {
				log.Println("RegisterCodec H265 failed:", err)
			}
		case webrtc.MimeTypeAV1:
			// 注册 AV1 (视频)
			err := m.RegisterCodec(webrtc.RTPCodecParameters{
				RTPCodecCapability: webrtc.RTPCodecCapability{
					MimeType:     webrtc.MimeTypeAV1,
					ClockRate:    90000,
					Channels:     0,
					SDPFmtpLine:  "",
					RTCPFeedback: []webrtc.RTCPFeedback{{Type: "goog-remb", Parameter: ""}, {Type: "ccm", Parameter: "fir"}}, // 禁用 {"nack", ""} 以关闭重传
				},
				PayloadType: 105, // 使用 105，避开其他视频编解码器的 Payload Type
			}, webrtc.RTPCodecTypeVideo)
			if err != nil {
				log.Println("RegisterCodec AV1 failed:", err)
			}
		default:
			log.Printf("Unsupported MIME type: %s", mime)
		}

	}
	return m
}

// SetSDPBandwidth 在 SDP 的 video m-line 后插入 b=AS:20000 (20Mbps)
func SetSDPBandwidth(sdp string, bandwidth int) string {
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
