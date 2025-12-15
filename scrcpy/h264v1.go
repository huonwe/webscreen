package scrcpy

import (
	"iter"
	"time"
)

// 辅助函数：查找下一个 Start Code (00 00 00 01) 的位置
// 如果找不到，返回 -1
func findNextStartCode(data []byte) int {
	n := len(data)
	for i := 0; i < n-3; i++ {
		if data[i] == 0 && data[i+1] == 0 && data[i+2] == 0 && data[i+3] == 1 {
			return i
		}
	}
	return -1
}

func (da *DataAdapter) GenerateWebRTCFrameH264v1(header ScrcpyFrameHeader, payload []byte) iter.Seq[WebRTCFrame] {
	return func(yield func(WebRTCFrame) bool) {
		// 1. 快速路径：非混合包（绝大多数 P 帧和简单 IDR 帧）
		// Scrcpy 发送的 raw 帧通常以 00 00 00 01 开头，第 5 字节是 NAL Header
		// NAL Type: payload[4] & 0x1F
		// Type 7 = SPS, Type 8 = PPS, Type 5 = IDR, Type 1 = Non-IDR
		nalType := payload[4] & 0x1F

		// 如果不是 SPS (7)，通常意味着这是单一的 P 帧或不带配置的 IDR 帧
		// 直接走零拷贝透传
		if nalType != 7 {
			if header.IsKeyFrame {
				// 发送缓存的 SPS/PPS
				da.keyFrameMutex.RLock()
				if da.LastSPS != nil {
					if !yield(WebRTCFrame{Data: createCopy(da.LastSPS, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
						da.keyFrameMutex.RUnlock()
						return
					}
				}
				if da.LastPPS != nil {
					if !yield(WebRTCFrame{Data: createCopy(da.LastPPS, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
						da.keyFrameMutex.RUnlock()
						return
					}
				}
				da.keyFrameMutex.RUnlock()

				// 缓存当前的 IDR (注意：这里必须拷贝，不能引用 payload，因为 payload 会被复用/覆盖)
				da.keyFrameMutex.Lock()
				da.LastIDR = createCopy(payload, &da.PayloadPoolLarge)
				da.LastIDRTime = time.Now()
				da.keyFrameMutex.Unlock()
			}

			// 直接 yield 切片，不进行拷贝
			// 注意：PayloadPoolLarge 的归还责任转交给了消费者 (StreamManager)
			if !yield(WebRTCFrame{
				Data:      payload, // 去掉 Start Code (4 bytes) 传给 WebRTC
				Timestamp: int64(header.PTS),
				NotConfig: true,
			}) {
				return
			}
			return
		}

		// 2. 慢速路径：混合包 (SPS + PPS + IDR)
		// 这种包只在设备启动或旋转时出现。我们需要手动切分，避免 bytes.Split 的内存分配。
		offset := 0
		dataLen := len(payload)

		for offset < dataLen {
			// 当前 NALU 开始位置 (包含 Start Code)
			start := offset
			// 跳过当前的 Start Code (4 bytes) 查找下一个
			searchStart := offset + 4
			if searchStart >= dataLen {
				break
			}

			// 查找下一个 Start Code
			nextStartRelative := findNextStartCode(payload[searchStart:])
			var end int
			if nextStartRelative == -1 {
				end = dataLen
			} else {
				end = searchStart + nextStartRelative
			}

			// 当前 NALU 数据 (包含 Start Code 00 00 00 01)
			nalUnit := payload[start:end]
			if len(nalUnit) > 4 {
				currentNalType := nalUnit[4] & 0x1F
				switch currentNalType {
				case 7: // SPS
					da.updateVideoMetaFromSPS(nalUnit[4:])
					da.keyFrameMutex.Lock()
					// SPS 很小，拷贝一份缓存
					da.LastSPS = createCopy(nalUnit, &da.PayloadPoolSmall)
					da.keyFrameMutex.Unlock()
					// 发送 SPS (使用 Small Pool)
					if !yield(WebRTCFrame{Data: createCopy(nalUnit, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
						return
					}
				case 8: // PPS
					da.keyFrameMutex.Lock()
					da.LastPPS = createCopy(nalUnit, &da.PayloadPoolSmall)
					da.keyFrameMutex.Unlock()
					// 发送 PPS (使用 Small Pool)
					if !yield(WebRTCFrame{Data: createCopy(nalUnit, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
						return
					}
				case 5: // IDR
					da.keyFrameMutex.Lock()
					// 缓存 IDR (必须拷贝)
					da.LastIDR = createCopy(nalUnit, &da.PayloadPoolLarge)
					da.LastIDRTime = time.Now()
					da.keyFrameMutex.Unlock()

					// 发送 IDR。
					// 注意：这里不能直接使用 nalUnit[4:] (Zero-Copy)，因为：
					// 1. nalUnit 是 payload 的切片，Put 回去会导致 Pool 中的 buffer 容量减小 (偏移了 start+4)。
					// 2. StreamManager 会再次切片 [4:]，导致数据丢失 (如果这里已经切掉了 start code)。
					// 因此，这里必须拷贝一份完整的 NALU (含 Start Code) 给 StreamManager。
					// 虽然多了一次拷贝，但混合包 (SPS+PPS+IDR) 仅在启动或旋转时出现，频率极低，影响可忽略。
					idrCopy := createCopy(nalUnit, &da.PayloadPoolLarge)
					if !yield(WebRTCFrame{Data: idrCopy, Timestamp: int64(header.PTS), NotConfig: true}) {
						return
					}
				}
			}
			offset = end
		}
	}
}
