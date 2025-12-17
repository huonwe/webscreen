package ScrcpyAgent

import (
	"crypto/rand"
	"fmt"
	"log"
	"strings"
	"webcpy/sdriver"
	"webcpy/sdriver/dummy"
	sagent "webcpy/streamAgent"
	"webcpy/streamAgent/webrtcHelper"

	"github.com/pion/webrtc/v4"
)

func generateStreamID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "scrcpy-stream"
	}
	return fmt.Sprintf("scrcpy-%x", b)
}

type ScrcpyAgent sagent.Agent

// 创建视频轨和音频轨，并初始化 ScrcpyAgent. 可以选择是否开启音视频同步.
func NewScrcpyAgent(config map[string]string) *ScrcpyAgent {
	sa := &ScrcpyAgent{
		Config: config,
	}
	videoCodec, audioCodec := sa.Driver.CodecInfo()
	log.Printf("Driver codecs - Video: %s, Audio: %s", videoCodec, audioCodec)
	var videoMimeType, audioMimeType string
	switch videoCodec {
	case "h264":
		videoMimeType = webrtc.MimeTypeH264
	case "h265":
		videoMimeType = webrtc.MimeTypeH265
	case "av1":
		videoMimeType = webrtc.MimeTypeAV1
	default:
		log.Printf("Unsupported video codec: %s", videoCodec)
	}
	switch audioCodec {
	case "opus":
		audioMimeType = webrtc.MimeTypeOpus
	default:
		log.Printf("Unsupported audio codec: %s", audioCodec)
	}
	log.Printf("Creating tracks with MIME types - Video: %s, Audio: %s", videoMimeType, audioMimeType)
	streamID := generateStreamID()

	var videoStreamID, audioStreamID string
	if strings.ToLower(sa.Config["avsync"]) == "on" {
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
			videoStreamID, // <--- 关键点
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

	sa.Driver = dummy.New(sdriver.StreamConfig{
		// TODO
	})
	@@@@

	// go sa.StartBroadcaster()
	return sa
}

func (sa *ScrcpyAgent) HandleSDP(offer string) string {
	var finalSDP string
	finalSDP, sa.RtpSenderVideo, sa.RtpSenderAudio = webrtcHelper.HandleSDP(offer, sa.VideoTrack, sa.AudioTrack)
	return finalSDP
}

func (sa *ScrcpyAgent) Close() {

}

func (sa *ScrcpyAgent) StartStream() {

}

func (sa *ScrcpyAgent) PauseStream() {

}

func (sa *ScrcpyAgent) ResumeStream() {

}

func (sa *ScrcpyAgent) SendControlEvent(event sdriver.ControlEvent) error {
	return nil
}

func (sa *ScrcpyAgent) SetDriver(sdriver sdriver.SDriver) {
	sa.Driver = sdriver
}
