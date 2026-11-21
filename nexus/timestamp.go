package nexus

import (
	"math/rand"
	"webscreen/sdriver"
)

type rtpTimestampMapper struct {
	initialized bool
	basePTS     uint64 // 首个有效帧的 PTS
	lastPTS     uint64 // 上一帧 PTS（用于处理 NoDuration 帧）
	baseRTP     uint32 // 初始随机 RTP 时间戳偏移（符合 RFC 3550）
	clockRate   uint64 // 视频固定 90000，Opus 音频固定 48000
}

func newRTPTimestampMapper(clockRate uint64) *rtpTimestampMapper {
	return &rtpTimestampMapper{
		baseRTP:   rand.Uint32(),
		clockRate: clockRate,
	}
}

// NextTimestamp 将 box.PTS 转换为 RTP 32位时间戳
// 假设 box.PTS 单位为微秒 (us，scrcpy/FFmpeg 默认单位)；若是纳秒则把 1_000_000 改为 1_000_000_000
func (m *rtpTimestampMapper) NextTimestamp(box sdriver.AVBox) uint32 {
	if !m.initialized {
		m.initialized = true
		m.basePTS = box.PTS
		m.lastPTS = box.PTS
		return m.baseRTP
	}

	pts := box.PTS
	// 如果是 SPS/PPS 等不占时间轴的配置帧，直接复用上一帧时间戳
	if box.NoDuration || pts < m.lastPTS {
		pts = m.lastPTS
	} else {
		m.lastPTS = pts
	}

	elapsedPTS := pts - m.basePTS
	// 换算为对应时钟频率的 ticks，利用 uint32 自然回绕
	rtpTicks := uint32((elapsedPTS * m.clockRate) / 1_000_000)
	return m.baseRTP + rtpTicks
}
