package nexus

import (
	"log"
	"strconv"
	"strings"

	"github.com/pion/webrtc/v5"
)

func scrcpyVideoOptionsFromWebRTCCodecParameters(finalCodec webrtc.RTPCodecParameters) (map[string]any, error) {
	video_codec_options := ""
	max_size := 0
	max_fps := 0
	video_bit_rate := 0
	_, mimeType, sdpFmtpLine := finalCodec.PayloadType, finalCodec.MimeType, finalCodec.SDPFmtpLine
	if strings.EqualFold(mimeType, "video/AV1") {
		log.Println("AV1 Main 5.1")
		// --- AV1 ---
		// PAYLOAD_TYPE_AV1_PROFILE_MAIN_5_1
		video_codec_options += "profile=1"
		kv := strings.Split(sdpFmtpLine, ";")
		for _, item := range kv {
			item = strings.TrimSpace(item)
			if strings.HasPrefix(item, "level-idx=") {
				levelStr := strings.TrimPrefix(item, "level-idx=")
				levelID64, err := strconv.ParseUint(levelStr, 10, 32)
				if err != nil {
					log.Printf("Failed to parse level-idx: %v", err)
					return nil, err
				}
				levelID := uint(levelID64)
				switch levelID {
				case 8:
					log.Println("AV1 Level 4.0")
					max_size = 1920
					video_bit_rate = 12_000_000
				case 9:
					log.Println("AV1 Level 4.1")
					max_size = 1920
					max_fps = 60
					video_bit_rate = 20_000_000
				case 12:
					log.Println("AV1 Level 5.0")
					video_bit_rate = 30_000_000
				case 13:
					log.Println("AV1 Level 5.1")
					video_bit_rate = 40_000_000
				case 14:
					log.Println("AV1 Level 5.2")
					video_bit_rate = 60_000_000
				case 15:
					log.Println("AV1 Level 5.3")
					video_bit_rate = 80_000_000
				default:
					log.Println("AV1 Unexpected Level ID:", levelID)
					if levelID < 8 {
						video_bit_rate = 10_000_000
					}
					if levelID > 15 {
						video_bit_rate = 80_000_000
					}
				}
			}
		}
	} else if strings.EqualFold(mimeType, "video/H265") || strings.EqualFold(mimeType, "video/HEVC") {
		// --- H.265 (HEVC) ---
		// Main Profile
		video_codec_options += "profile=1"
		// FMTP example: "profile-id=1;tier-flag=0;level-id=123"
		// level-id=123 (Level 4.1), level-id=153 (Level 5.1)
		var levelID uint
		kv := strings.Split(sdpFmtpLine, ";")
		for _, item := range kv {
			item = strings.TrimSpace(item)
			if strings.HasPrefix(item, "level-id=") {
				levelStr := strings.TrimPrefix(item, "level-id=")
				levelID64, err := strconv.ParseUint(levelStr, 10, 32)
				if err != nil {
					log.Printf("Failed to parse level-id: %v", err)
					return nil, err
				}
				levelID = uint(levelID64)
				break
			}
		}
		switch levelID {
		case 123:
			log.Println("H.265 Level 4.1")
			max_size = 1920
			max_fps = 60
			video_bit_rate = 20_000_000
		case 150:
			log.Println("H.265 Level 5.0")
			video_bit_rate = 25_000_000
		case 153:
			log.Println("H.265 Level 5.1")
			video_bit_rate = 40_000_000
		case 156:
			log.Println("H.265 Level 5.2")
			video_bit_rate = 50_000_000
		case 180:
			log.Println("H.265 Level 6.0")
			video_bit_rate = 60_000_000
		default:
			log.Println("H.265 Unexpected Level ID:", levelID)
			if levelID < 123 {
				video_bit_rate = 10_000_000
			}
			if levelID > 180 {
				video_bit_rate = 60_000_000
			}
		}
	} else if strings.EqualFold(mimeType, "video/H264") {
		// --- H.264 (AVC) ---
		// profile-level-id : Baseline (42), Main (4d), High (64)
		// level-asymmetry-allowed=1 is supposed to be always set
		video_bit_rate = 300_000_000
		if strings.Contains(sdpFmtpLine, "profile-level-id=42") {
			log.Println("H.264 Baseline Profile")
			video_codec_options += "profile=1"
		} else if strings.Contains(sdpFmtpLine, "profile-level-id=4d") {
			log.Println("H.264 Main Profile")
			video_codec_options += "profile=2"
		} else if strings.Contains(sdpFmtpLine, "profile-level-id=64") {
			log.Println("H.264 High Profile")
			video_codec_options += "profile=8"
		}
	}

	return map[string]any{
		"video_codec_options": video_codec_options,
		"max_size":            max_size,
		"max_fps":             max_fps,
		"video_bit_rate":      video_bit_rate,
	}, nil
}
