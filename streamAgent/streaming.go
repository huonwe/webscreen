package sagent

import (
	"log"
	"time"
	"webscreen/sdriver"

	"github.com/pion/webrtc/v4/pkg/media"
)

func (sa *Agent) StreamingVideo() {
	if sa.videoCh == nil {
		log.Println("[Agent] Video channel is nil, skipping video streaming")
		sa.controlCh <- sdriver.TextMsgEvent{Msg: "Video channel is nil, cannot stream video."}
		return
	}
	sa.lastVideoPTS = 0
	const defaultDuration = time.Millisecond * 16
	var firstPTS time.Duration = -1
	for vBox := range sa.videoCh {
		if firstPTS == -1 {
			firstPTS = vBox.PTS
		}

		// 计算当前帧相对于第一帧过了多久
		// 这样可以确保 ptsOffset 从 0 开始增长
		ptsOffset := vBox.PTS - firstPTS
		// Timestamp = Agent启动时间 + 视频流逝的时间
		timestamp := sa.baseTime.Add(ptsOffset)

		var duration time.Duration

		// if vBox.IsConfig {
		// 	// Config 帧 (SPS/PPS) 不应该占据时间轴
		// 	duration = 0
		// } else {
		// 如果是第一帧 VCL
		if sa.lastVideoPTS == 0 {
			duration = defaultDuration
		} else {
			// 计算与上一帧的时间差
			delta := vBox.PTS - sa.lastVideoPTS

			if delta <= 0 {
				duration = defaultDuration
			} else {
				duration = delta
			}
		}
		sa.lastVideoPTS = vBox.PTS
		// }

		sample := media.Sample{
			Data:      vBox.Data,
			Duration:  duration,
			Timestamp: timestamp,
		}

		if err := sa.VideoTrack.WriteSample(sample); err != nil {
			// log.Println("WriteSample error:", err)
			return
		}
	}
}

func (sa *Agent) StreamingAudio() {
	if sa.audioCh == nil {
		log.Println("[Agent] Audio channel is nil, skipping audio streaming")
		sa.controlCh <- sdriver.TextMsgEvent{Msg: "Audio channel is nil, cannot stream audio."}
		return
	}
	log.Println("[Agent] Audio streaming started")
	// Opus 默认帧长通常是 20ms
	const defaultDuration = 20 * time.Millisecond
	var currentTimestamp = sa.baseTime
	for aBox := range sa.audioCh {
		if err := sa.AudioTrack.WriteSample(media.Sample{
			Data:      aBox.Data,
			Duration:  defaultDuration,
			Timestamp: currentTimestamp,
		}); err != nil {
			// log.Printf("Audio WriteSample err: %v\n", err)
			return
		}
		currentTimestamp = currentTimestamp.Add(defaultDuration)
	}
}
