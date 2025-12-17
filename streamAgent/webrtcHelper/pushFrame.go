package webrtcHelper

import (
	"time"
	"webcpy/sdriver"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

func PushVideoFrame(vFrame sdriver.AVBox, vTrack webrtc.TrackLocalStaticSample, lastPTS *time.Duration) error {
	var duration time.Duration
	if !vFrame.IsConfig {
		if *lastPTS == 0 {
			duration = time.Millisecond * 16
		} else {
			delta := vFrame.PTS - *lastPTS
			if delta <= 0 {
				duration = time.Microsecond
			} else {
				duration = delta
			}
		}
	} else {
		// Config 帧 (VPS/SPS/PPS) 不需要持续时间
		duration = 1 * time.Microsecond
	}
	if duration < 0 {
		duration = 1 * time.Microsecond
	}

	*lastPTS = vFrame.PTS

	sample := media.Sample{
		Data:      vFrame.Data,
		Duration:  duration,
		Timestamp: time.Now(),
	}
	return vTrack.WriteSample(sample)
}

// 注意：aTrack 类型应该是 *webrtc.TrackLocalStaticSample (指针)
func PushAudioFrame(aFrame sdriver.AVBox, aTrack *webrtc.TrackLocalStaticSample, lastPTS *time.Duration) error {
	var duration time.Duration

	// 1. 获取当前 PTS
	currentPTS := aFrame.PTS

	// 2. 计算差值 (Duration)
	if *lastPTS == 0 {
		// 第一帧：音频通常可以给一个标准值作为初始猜测
		// Opus 常见是 20ms
		duration = 20 * time.Millisecond
	} else {
		delta := currentPTS - *lastPTS
		if delta <= 0 {
			// 音频的时间戳通常非常规律，如果出现 <=0，说明乱序严重
			// 给个极小值，或者直接丢弃这一帧（音频对乱序很敏感）
			duration = time.Microsecond
		} else {
			duration = delta
		}
	}

	// 3. 更新上一帧时间
	*lastPTS = currentPTS

	// 4. 构造 Sample
	sample := media.Sample{
		Data:      aFrame.Data,
		Duration:  duration,   // ✅ 让 Pion 根据真实的间隔来打 RTP 时间戳
		Timestamp: time.Now(), // 可选，不需要用 UnixMicro 强转 PTS
	}

	return aTrack.WriteSample(sample)
}
