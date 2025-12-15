package scrcpy

import (
	"bytes"
	"iter"
	"sync"
	"time"
)

// GenerateWebRTCFrameH265_v2 使用 bytes.Index 实现零分配的高性能拆包
func (da *DataAdapter) GenerateWebRTCFrameH265(header ScrcpyFrameHeader, payload []byte) iter.Seq[WebRTCFrame] {
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
			// H.265 NALU Header: F(1) + Type(6) + LayerId(6) + TID(3)
			// Type 在第一个字节的中间 6 位: (nal[0] >> 1) & 0x3F
			nalType := (nal[0] >> 1) & 0x3F
			var pool *sync.Pool
			isConfig := false

			switch nalType {
			case 32: // VPS
				da.keyFrameMutex.Lock()
				da.LastVPS = nal
				da.keyFrameMutex.Unlock()
				isConfig = true
			case 33: // SPS
				da.updateVideoMetaFromSPS(nal, "h265")
				da.keyFrameMutex.Lock()
				da.LastSPS = nal
				da.keyFrameMutex.Unlock()
				isConfig = true
			case 34: // PPS
				da.keyFrameMutex.Lock()
				da.LastPPS = nal
				da.keyFrameMutex.Unlock()
				isConfig = true
			case 39, 40: // SEI (Prefix/Suffix)
				isConfig = true
			case 19, 20, 21: // IDR (W_RADL, W_LP, CRA)
				da.keyFrameMutex.Lock()
				da.LastIDR = nal
				da.LastIDRTime = time.Now()
				da.keyFrameMutex.Unlock()
			}

			if isConfig {
				pool = &da.PayloadPoolSmall
			} else {
				pool = &da.PayloadPoolLarge
			}

			// 如果是 IDR 帧，先发送缓存的 VPS/SPS/PPS
			if nalType == 19 || nalType == 20 || nalType == 21 {
				da.keyFrameMutex.RLock()
				vps, sps, pps := da.LastVPS, da.LastSPS, da.LastPPS
				da.keyFrameMutex.RUnlock()

				if vps != nil {
					if !yield(WebRTCFrame{Data: createCopy(vps, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
						return
					}
				}
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
