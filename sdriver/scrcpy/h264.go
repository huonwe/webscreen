package scrcpy

import (
	"bytes"
	"iter"
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

		// if header.IsKeyFrame {
		// 	fmt.Println("--------------Key Frame -------------------------")
		// }
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
			isConfig := true
			// fmt.Printf("Debug H264: Type: %d, Size: %d\n", nalType, len(nal))
			switch nalType {
			case 7: // SPS
				da.updateVideoMetaFromSPS(nal, "h264")
				da.keyFrameMutex.Lock()
				da.LastSPS = createCopy(nal) // 必须拷贝，因为 nal 引用的是 LinearBuffer
				da.keyFrameMutex.Unlock()
			case 8: // PPS
				da.keyFrameMutex.Lock()
				da.LastPPS = createCopy(nal) // 必须拷贝
				da.keyFrameMutex.Unlock()
				// configInPacket = true
			case 6: // SEI discard
				continue
			case 5: // IDR
				da.keyFrameMutex.Lock()
				da.LastIDR = createCopy(nal) // 必须拷贝
				da.LastIDRTime = time.Now()
				da.keyFrameMutex.Unlock()
				isConfig = false

				// 如果没有丢弃SEI，可以考虑发送缓存的 SPS/PPS
				//da.keyFrameMutex.RLock()
				sps, pps := da.LastSPS, da.LastPPS
				//da.keyFrameMutex.RUnlock()

				if sps != nil {
					if !yield(WebRTCFrame{Data: createCopy(sps), Timestamp: int64(header.PTS)}) {
						return
					}
					// 	log.Printf("(cached SPS) Sending NALU Type: %d, Size: %d", 7, len(sps))
				}
				if pps != nil {
					if !yield(WebRTCFrame{Data: createCopy(pps), Timestamp: int64(header.PTS)}) {
						return
					}
					// 	log.Printf("(cached PPS) Sending NALU Type: %d, Size: %d", 8, len(pps))
				}
			case 1:
				isConfig = false
			}

			// 发送当前 NALU (零拷贝，直接引用 LinearBuffer)
			if !yield(WebRTCFrame{
				Data:      nal,
				Timestamp: int64(header.PTS),
				NotConfig: !isConfig,
			}) {
				return
			}
			// log.Printf("Sending NALU Type: %d, Size: %d", nalType, len(nal))
		}
		// if header.IsKeyFrame {
		// 	fmt.Println("-------------------Key Frame End-----------------------------")
		// }
	}
}

func (da *DataAdapter) GenerateWebRTCFrameH264_v2(header ScrcpyFrameHeader, payload []byte) iter.Seq[WebRTCFrame] {
	return func(yield func(WebRTCFrame) bool) {
		startCode := []byte{0x00, 0x00, 0x00, 0x01}
		// 如果是 IDR 帧，先发送缓存的 SPS/PPS

		// 核心修复：无条件拆分所有包，解决 SEI+IDR 粘包问题
		parts := bytes.Split(payload, startCode)

		for _, nal := range parts {
			if len(nal) == 0 {
				continue
			}

			nalType := nal[0] & 0x1F
			// log.Printf("Debug H264_v2: Part %d, Type: %d, Size: %d", i, nalType, len(nal))

			isConfig := true

			switch nalType {
			case 7: // SPS
				da.updateVideoMetaFromSPS(nal, "h264")
				da.keyFrameMutex.Lock()
				da.LastSPS = nal
				da.keyFrameMutex.Unlock()
			case 8: // PPS
				da.keyFrameMutex.Lock()
				da.LastPPS = nal
				da.keyFrameMutex.Unlock()
			case 5: // IDR
				da.keyFrameMutex.Lock()
				da.LastIDR = nal
				da.LastIDRTime = time.Now()
				da.keyFrameMutex.Unlock()
				isConfig = false
			case 6: // SEI
				continue
			case 1:
				isConfig = false
			}

			// 发送当前 NALU (Raw NALU)
			if !yield(WebRTCFrame{
				Data:      nal,
				Timestamp: int64(header.PTS),
				NotConfig: !isConfig,
			}) {
				return
			}
		}
	}
}
