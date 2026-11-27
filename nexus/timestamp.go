package nexus

import (
	"math/rand"
	"time"
	"webscreen/sdriver"
)

type videoTimestampMapper struct {
	initialized              bool
	basePTS                  uint64 // 首个有效帧的 PTS
	lastPTS                  uint64 // 上一帧 PTS（用于处理 NoDuration 帧）
	baseRTP                  uint32 // 初始随机 RTP 时间戳偏移（符合 RFC 3550）
	lastRTP                  uint32 // 上一帧 RTP 时间戳
	baseTime                 time.Time
	clockRate                uint64 // 视频固定 90000，Opus 音频固定 48000
	dontTrustRemoteTimestamp bool
}

func newVideoTimestampMapper(clockRate uint64, dontTrustRemoteTimestamp bool) *videoTimestampMapper {
	return &videoTimestampMapper{
		baseRTP:                  rand.Uint32(),
		clockRate:                clockRate,
		dontTrustRemoteTimestamp: dontTrustRemoteTimestamp,
	}
}

// NextTimestamp 将 box.PTS 转换为 RTP 32位时间戳
// 假设 box.PTS 单位为微秒 (us，scrcpy/FFmpeg 默认单位)；若是纳秒则把 1_000_000 改为 1_000_000_000
func (m *videoTimestampMapper) NextTimestamp(box sdriver.AVBox) uint32 {
	if !m.initialized {
		m.initialized = true
		m.basePTS = box.PTS
		m.lastPTS = box.PTS
		m.baseTime = time.Now()
		m.lastRTP = m.baseRTP
		return m.baseRTP
	}

	// SPS/PPS 等不占时间轴的配置帧，直接复用上一帧的 RTP 时间戳
	if box.NoDuration {
		return m.lastRTP
	}

	var elapsedMicro uint64
	if m.dontTrustRemoteTimestamp {
		// 使用 time.Since 利用 Go 的单调时钟（Monotonic Clock），防止 NTP 校时导致时间回退
		elapsed := time.Since(m.baseTime)
		if elapsed < 0 {
			elapsed = 0
		}
		elapsedMicro = uint64(elapsed.Microseconds())
	} else {
		pts := box.PTS
		if pts < m.lastPTS {
			// 远端 PTS 异常回退时，保持上一帧时间戳
			pts = m.lastPTS
		} else {
			m.lastPTS = pts
		}
		elapsedMicro = pts - m.basePTS
	}

	// 换算为对应时钟频率的 ticks，利用 uint32 自然回绕
	rtpTicks := uint32((elapsedMicro * m.clockRate) / 1_000_000)
	m.lastRTP = m.baseRTP + rtpTicks
	return m.lastRTP
}

type audioTimestampMapper struct {
	isInit     bool
	startTime  time.Time
	baseRTP    uint32
	currentRTP uint32
}

func newAudioTimestampMapper() *audioTimestampMapper {
	return &audioTimestampMapper{}
}

func (m *audioTimestampMapper) NextTimestamp(box sdriver.AVBox) uint32 {
	now := time.Now()
	if !m.isInit {
		m.isInit = true
		m.startTime = now
		m.baseRTP = rand.Uint32()
		m.currentRTP = m.baseRTP
		if !box.NoDuration {
			m.currentRTP += 960
		}
		return m.baseRTP
	}

	// 1. 计算真实墙钟对应的理论 RTP 时间戳
	elapsedUs := now.Sub(m.startTime).Microseconds()
	realLocalRTP := m.baseRTP + uint32((elapsedUs*48)/1000)

	// 2. 防漂移检查（使用 int32 差值自动兼容 uint32 溢出回绕）
	diff := int32(realLocalRTP - m.currentRTP)
	if diff > 4800 || diff < -4800 {
		m.currentRTP = realLocalRTP
	}

	ts := m.currentRTP

	// 3. 严格按 20ms (960 samples) 递增，配置帧不推进时间轴
	if !box.NoDuration {
		m.currentRTP += 960
	}
	return ts
}
