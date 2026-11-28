package nexus

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"scrmux/consts"
	"time"

	"github.com/pion/webrtc/v5"
)

func getMimeTypeFromString(codecName string) (string, error) {
	var mimeType string

	switch codecName {
	case consts.CODEC_TYPE_H264, "avc":
		mimeType = webrtc.MimeTypeH264
	case consts.CODEC_TYPE_H265, "hevc":
		mimeType = webrtc.MimeTypeH265
	case consts.CODEC_TYPE_AV1:
		mimeType = webrtc.MimeTypeAV1
	case consts.CODEC_TYPE_OPUS:
		mimeType = webrtc.MimeTypeOpus
	default:
		return "", errors.New("unsupported video codec")
	}
	return mimeType, nil
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
				PayloadType: consts.PAYLOAD_TYPE_AV1_PROFILE_MAIN_5_1,
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
		PayloadType: consts.PAYLOAD_TYPE_H264_PROFILE_HIGH_5_1,
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
		PayloadType: consts.PAYLOAD_TYPE_H264_PROFILE_HIGH_5_1_0C,
	}, webrtc.RTPCodecTypeVideo)
	if err != nil {
		log.Println("RegisterCodec H264 failed:", err)
	}
	// Baseline Profile
	err = m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			Channels:    0,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
			RTCPFeedback: []webrtc.RTCPFeedback{
				{Type: "transport-cc", Parameter: ""},
				{Type: "ccm", Parameter: "fir"},
				{Type: "nack", Parameter: ""},
				{Type: "nack", Parameter: "pli"},
			},
		},
		PayloadType: consts.PAYLOAD_TYPE_H264_PROFILE_BASELINE_3_1,
	}, webrtc.RTPCodecTypeVideo)
	if err != nil {
		log.Println("RegisterCodec H264 failed:", err)
	}
	// baseline profile for iphone safari
	err = m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			Channels:    0,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=420c1f",
			RTCPFeedback: []webrtc.RTCPFeedback{
				{Type: "transport-cc", Parameter: ""},
				{Type: "ccm", Parameter: "fir"},
				{Type: "nack", Parameter: ""},
				{Type: "nack", Parameter: "pli"},
			},
		},
		PayloadType: consts.PAYLOAD_TYPE_H264_PROFILE_BASELINE_3_1_0C,
	}, webrtc.RTPCodecTypeVideo)
	if err != nil {
		log.Println("RegisterCodec H264 failed:", err)
	}
	log.Println("Registered H264 codec")
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
		PayloadType: consts.PAYLOAD_TYPE_H265_PROFILE_MAIN_TIER_MAIN_5_1,
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
		PayloadType: consts.PAYLOAD_TYPE_H265_PROFILE_MAIN_TIER_MAIN_4_1,
	}, webrtc.RTPCodecTypeVideo)
	if err != nil {
		log.Println("RegisterCodec H265 failed:", err)
	}
	log.Println("Registered H265 codec")
}

func WaitAndGetFinalCodecParams(pc *webrtc.PeerConnection) (webrtc.RTPCodecParameters, error) {
	startTime := time.Now()
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
		if time.Since(startTime) > 10*time.Second {
			return webrtc.RTPCodecParameters{}, fmt.Errorf("timeout waiting for final codec parameters")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func createAVTrack(videoMimeType string, AVSync bool) (*webrtc.TrackLocalStaticRTP, *webrtc.TrackLocalStaticRTP) {
	mark := fmt.Sprintf("%d", time.Now().UnixNano())
	trackID := fmt.Sprintf("%s-%s", "scrmux-track", randomString(8))
	trackIDVideo := trackID + "-" + mark + "-video"
	trackIDAudio := trackID + "-" + mark + "-audio"
	streamID := fmt.Sprintf("%s-%s", "scrmux-stream", randomString(8))
	streamIDVideo := streamID + "-" + mark + "-video"
	streamIDAudio := streamID + "-" + mark + "-audio"
	if AVSync {
		streamIDVideo = streamID + "-" + mark
		streamIDAudio = streamID + "-" + mark
	}

	trackVideo, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: videoMimeType}, trackIDVideo, streamIDVideo)
	if err != nil {
		log.Printf("Failed to create track for MIME type %s: %v", videoMimeType, err)
	}
	trackAudio, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, trackIDAudio, streamIDAudio)
	if err != nil {
		log.Printf("Failed to create audio track for MIME type %s: %v", webrtc.MimeTypeOpus, err)
	}
	return trackVideo, trackAudio
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
