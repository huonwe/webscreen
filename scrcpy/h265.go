package scrcpy

import (
	"bytes"
	"fmt"
	"iter"
	"log"
	"time"
)

// GenerateWebRTCFrameH265 使用 bytes.Index 实现零分配的高性能拆包
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
		if header.IsKeyFrame {
			fmt.Println("--------------Key Frame -------------------------")
		}

		totalLen := len(payload)

		for pos < totalLen {
			// 1. 查找下一个起始码的位置
			nextStartRelative := bytes.Index(payload[pos:], startCode)

			var end int
			if nextStartRelative == -1 {
				end = totalLen
			} else {
				end = pos + nextStartRelative
			}

			// 2. 获取 Raw NALU (不含起始码，零拷贝切片)
			nal := payload[pos:end]

			// 更新游标到下一个 NALU 的数据开始处
			pos = end + 4

			if len(nal) == 0 {
				continue
			}

			// --- H.265 处理逻辑 ---
			// H.265 NALU Header: F(1) + Type(6) + LayerId(6) + TID(3)
			// Type 在第一个字节的中间 6 位
			nalType := (nal[0] >> 1) & 0x3F
			isConfig := true

			switch nalType {
			case 32: // VPS
				// log.Printf("Debug H265: VPS NALU, Size: %d", len(nal))
				da.keyFrameMutex.Lock()
				da.LastVPS = createCopy(nal) // 必须拷贝
				da.keyFrameMutex.Unlock()
			case 33: // SPS
				// log.Printf("Debug H265: SPS NALU, Size: %d", len(nal))
				// da.updateVideoMetaFromSPS(nal, "h265") // H265 SPS 解析暂略
				da.keyFrameMutex.Lock()
				da.LastSPS = createCopy(nal) // 必须拷贝
				da.keyFrameMutex.Unlock()
			case 34: // PPS
				// log.Printf("Debug H265: PPS NALU, Size: %d", len(nal))
				da.keyFrameMutex.Lock()
				da.LastPPS = createCopy(nal) // 必须拷贝
				da.keyFrameMutex.Unlock()
			case 39, 40: // SEI (Prefix, Suffix)
				continue
			case 19, 20, 21: // IDR (W_RADL, W_LP, CRA)
				da.keyFrameMutex.Lock()
				da.LastIDR = createCopy(nal) // 必须拷贝
				da.LastIDRTime = time.Now()
				da.keyFrameMutex.Unlock()
				isConfig = false
				// log.Fatalln("收到关键帧！！！")

				// 强制注入缓存的 VPS/SPS/PPS，即使包内已经包含了它们
				// 这能解决部分浏览器兼容性问题，并防止 buffer 复用导致的数据损坏
				// da.keyFrameMutex.RLock()
				// vps, sps, pps := da.LastVPS, da.LastSPS, da.LastPPS
				// da.keyFrameMutex.RUnlock()

				// if vps != nil {
				// 	if !yield(WebRTCFrame{Data: createCopy(vps), Timestamp: int64(header.PTS)}) {
				// 		return
				// 	}
				// 	log.Printf("(cached VPS) Sending NALU Type: %d, Size: %d", 32, len(vps))
				// }
				// if sps != nil {
				// 	if !yield(WebRTCFrame{Data: createCopy(sps), Timestamp: int64(header.PTS)}) {
				// 		return
				// 	}
				// 	log.Printf("(cached SPS) Sending NALU Type: %d, Size: %d", 33, len(sps))
				// }
				// if pps != nil {
				// 	if !yield(WebRTCFrame{Data: createCopy(pps), Timestamp: int64(header.PTS)}) {
				// 		return
				// 	}
				// 	log.Printf("(cached PPS) Sending NALU Type: %d, Size: %d", 34, len(pps))
				// }
			case 0, 1:
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
			log.Printf("Sending NALU Type: %d, Size: %d", nalType, len(nal))
		}
		if header.IsKeyFrame {
			fmt.Println("-------------------Key Frame End-----------------------------")
		}
	}
}
