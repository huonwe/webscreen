package sagent

import (
	"time"

	"github.com/pion/webrtc/v4/pkg/media"
)

func (sa *Agent) StreamingVideo() {
	// 默认帧间隔 (比如 60fps = 16.6ms)
	// 用于当 PTS 计算出现异常时的默认值
	const defaultDuration = 16 * time.Millisecond

	// 用于记录第一帧的 PTS，用于归一化
	var firstPTS time.Duration = -1

	for vBox := range sa.videoCh {
		// 如果是第一帧，记录下它的 PTS 作为起点
		if firstPTS == -1 {
			firstPTS = vBox.PTS
		}

		// 计算当前帧相对于第一帧过了多久
		// 这样可以确保 ptsOffset 从 0 开始增长
		ptsOffset := vBox.PTS - firstPTS

		// Timestamp = Agent启动时间 + 视频流逝的时间
		timestamp := sa.baseTime.Add(ptsOffset)

		var duration time.Duration

		if vBox.IsConfig {
			// Config 帧 (SPS/PPS) 不应该占据时间轴
			duration = 0
		} else {
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
		}

		sample := media.Sample{
			Data:      vBox.Data,
			Duration:  duration,
			Timestamp: timestamp,
		}

		// 错误处理是必要的，防止 Track 关闭后 panic
		if err := sa.VideoTrack.WriteSample(sample); err != nil {
			// log.Println("WriteSample error:", err)
			return
		}
	}
}

func (sa *Agent) StreamingAudio() {
	// 音频通常是非常规律的，Opus 默认帧长通常是 20ms
	const defaultDuration = 20 * time.Millisecond

	// 记录首帧 PTS，用于归一化
	var firstPTS time.Duration = -1

	for aBox := range sa.audioCh {
		// ---------------------------------------------------------
		// 1. PTS 归一化 (核心：为了和视频同步)
		// ---------------------------------------------------------
		if firstPTS == -1 {
			firstPTS = aBox.PTS
		}

		// 计算偏移量：当前帧距离第一帧过了多久
		ptsOffset := aBox.PTS - firstPTS

		// 计算绝对时间戳
		// 【关键】必须使用和视频流完全相同的 sa.baseTime
		timestamp := sa.baseTime.Add(ptsOffset)

		// ---------------------------------------------------------
		// 2. 计算 Duration
		// ---------------------------------------------------------
		var duration time.Duration

		// 如果是 Config 帧 (AAC/Opus Header)，不占时间
		if aBox.IsConfig {
			duration = 0
		} else {
			if sa.lastAudioPTS == 0 {
				duration = defaultDuration
			} else {
				delta := aBox.PTS - sa.lastAudioPTS
				if delta <= 0 {
					// 音频乱序或重叠。音频对连续性要求极高。
					// 如果 delta <= 0，Pion 可能会丢弃包或报错。
					// 这里强制给一个标准值，保证 RTP 时间戳递增。
					duration = defaultDuration
				} else {
					duration = delta
				}
			}
			// 更新上一帧 PTS
			sa.lastAudioPTS = aBox.PTS
		}

		// ---------------------------------------------------------
		// 3. 构造 Sample
		// ---------------------------------------------------------
		sample := media.Sample{
			Data:      aBox.Data,
			Duration:  duration,
			Timestamp: timestamp,
		}

		if err := sa.AudioTrack.WriteSample(sample); err != nil {
			// log.Printf("Audio WriteSample err: %v\n", err)
			return
		}
	}
}
