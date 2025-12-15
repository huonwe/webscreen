package scrcpy

import (
	"bytes"
	"iter"
	"sync"
	"time"
)

// GenerateWebRTCFrameH264 使用 bytes.Index 实现零分配的高性能拆包
func (da *DataAdapter) GenerateWebRTCFrameH264(header ScrcpyFrameHeader, payload []byte) iter.Seq[WebRTCFrame] {
	return func(yield func(WebRTCFrame) bool) {
		// Scrcpy 始终使用 4 字节起始码
		startCode := []byte{0x00, 0x00, 0x00, 0x01}

		// 游标：指向当前 NALU 数据的起始位置
		pos := 0

		// 如果包头就是起始码，直接跳过
		if bytes.HasPrefix(payload, startCode) {
			pos = 4
		}

		totalLen := len(payload)

		for pos < totalLen {
			// 1. 查找下一个起始码的位置 (使用汇编优化的 bytes.Index)
			// 注意：搜索范围是 payload[pos:]，返回的是相对偏移量
			nextStartRelative := bytes.Index(payload[pos:], startCode)

			var end int
			if nextStartRelative == -1 {
				// 后面没有起始码了，说明当前 NALU 一直到包尾
				end = totalLen
			} else {
				// 当前 NALU 结束位置 = 当前起始位置 + 相对偏移量
				end = pos + nextStartRelative
			}

			// 2. 获取 Raw NALU (不含起始码，零拷贝切片)
			nal := payload[pos:end]

			// 更新游标到下一个 NALU 的数据开始处 (跳过 4 字节起始码)
			pos = end + 4

			if len(nal) == 0 {
				continue
			}

			// --- 以下是处理逻辑 ---
			nalType := nal[0] & 0x1F
			var pool *sync.Pool
			isConfig := false

			switch nalType {
			case 7: // SPS
				da.updateVideoMetaFromSPS(nal, "h264")
				da.keyFrameMutex.Lock()
				da.LastSPS = nal
				da.keyFrameMutex.Unlock()
				isConfig = true
			case 8: // PPS
				da.keyFrameMutex.Lock()
				da.LastPPS = nal
				da.keyFrameMutex.Unlock()
				isConfig = true
			case 5: // IDR
				da.keyFrameMutex.Lock()
				da.LastIDR = nal
				da.LastIDRTime = time.Now()
				da.keyFrameMutex.Unlock()
			case 6: // SEI
				isConfig = true
			}

			if isConfig {
				pool = &da.PayloadPoolSmall
			} else {
				pool = &da.PayloadPoolLarge
			}

			// 如果是 IDR 帧，先发送缓存的 SPS/PPS
			if nalType == 5 {
				da.keyFrameMutex.RLock()
				sps, pps := da.LastSPS, da.LastPPS
				da.keyFrameMutex.RUnlock()

				if sps != nil {
					if !yield(WebRTCFrame{Data: createCopy(sps, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
						return
					}
				}
				if pps != nil {
					if !yield(WebRTCFrame{Data: createCopy(pps, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
						return
					}
				}
			}

			// 发送当前 NALU (此时才进行内存拷贝)
			if !yield(WebRTCFrame{
				Data:      createCopy(nal, pool),
				Timestamp: int64(header.PTS),
				NotConfig: !isConfig,
			}) {
				return
			}
		}
	}
}
