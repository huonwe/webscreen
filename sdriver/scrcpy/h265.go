package scrcpy

import (
	"bytes"
	"fmt"
	"iter"
	"log"
	"time"
	"webcpy/sdriver"
)

// GenerateWebRTCFrameH265 使用 bytes.Index 实现零分配的高性能拆包
func (da *ScrcpyDriver) GenerateWebRTCFrameH265(header ScrcpyFrameHeader, payload []byte) iter.Seq[sdriver.AVBox] {
	return func(yield func(sdriver.AVBox) bool) {
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
				da.keyFrameMutex.RLock()
				vps, sps, pps := da.LastVPS, da.LastSPS, da.LastPPS
				da.keyFrameMutex.RUnlock()

				if vps != nil {
					if !yield(sdriver.AVBox{Data: createCopy(vps), PTS: time.Duration(header.PTS), IsConfig: true}) {
						return
					}
					log.Printf("(cached VPS) Sending NALU Type: %d, Size: %d", 32, len(vps))
				}
				if sps != nil {
					if !yield(sdriver.AVBox{Data: createCopy(sps), PTS: time.Duration(header.PTS), IsConfig: true}) {
						return
					}
					log.Printf("(cached SPS) Sending NALU Type: %d, Size: %d", 33, len(sps))
				}
				if pps != nil {
					if !yield(sdriver.AVBox{Data: createCopy(pps), PTS: time.Duration(header.PTS), IsConfig: true}) {
						return
					}
					log.Printf("(cached PPS) Sending NALU Type: %d, Size: %d", 34, len(pps))
				}
			case 0, 1:
				isConfig = false
			}

			// 发送当前 NALU (零拷贝，直接引用 LinearBuffer)
			if !yield(sdriver.AVBox{
				Data:     nal,
				PTS:      time.Duration(header.PTS),
				IsConfig: isConfig,
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

func (da *ScrcpyDriver) GenerateWebRTCFrameH265_debug(header ScrcpyFrameHeader, payload []byte) iter.Seq[sdriver.AVBox] {
	return func(yield func(sdriver.AVBox) bool) {
		startCode := []byte{0x00, 0x00, 0x00, 0x01}

		// 使用 bytes.Split 进行拆包，这是最稳妥的方式
		parts := bytes.Split(payload, startCode)
		if header.IsKeyFrame {
			fmt.Println("--------------Key Frame -------------------------")
		}
		for _, nal := range parts {
			if len(nal) == 0 {
				continue
			}

			// H.265 NALU Header: F(1) + Type(6) + LayerId(6) + TID(3)
			// Type 在第一个字节的中间 6 位
			nalType := (nal[0] >> 1) & 0x3F
			// log.Printf("Debug H265: Part %d, Type: %d, Size: %d", i, nalType, len(nal))

			isConfig := false

			switch nalType {
			case 32: // VPS
				da.keyFrameMutex.Lock()
				da.LastVPS = createCopy(nal)
				da.keyFrameMutex.Unlock()
				isConfig = true
			case 33: // SPS
				// da.updateVideoMetaFromSPS(nal, "h265") // H265 SPS 解析比较复杂，暂时注释
				da.keyFrameMutex.Lock()
				da.LastSPS = createCopy(nal)
				da.keyFrameMutex.Unlock()
				isConfig = true
			case 34: // PPS
				da.keyFrameMutex.Lock()
				da.LastPPS = createCopy(nal)
				da.keyFrameMutex.Unlock()
				isConfig = true
			case 39: // SEI
				continue
			case 40:
				isConfig = true
			case 19, 20, 21: // IDR
				da.keyFrameMutex.Lock()
				da.LastIDR = createCopy(nal)
				da.LastIDRTime = time.Now()
				da.keyFrameMutex.Unlock()

				da.keyFrameMutex.RLock()
				vps, sps, pps := da.LastVPS, da.LastSPS, da.LastPPS
				da.keyFrameMutex.RUnlock()

				if vps != nil {
					if !yield(sdriver.AVBox{Data: createCopy(vps), PTS: time.Duration(header.PTS), IsConfig: true}) {
						return
					}
				}
				if sps != nil {
					if !yield(sdriver.AVBox{Data: createCopy(sps), PTS: time.Duration(header.PTS), IsConfig: true}) {
						return
					}
				}
				if pps != nil {
					if !yield(sdriver.AVBox{Data: createCopy(pps), PTS: time.Duration(header.PTS), IsConfig: true}) {
						return
					}
				}
			}

			// 如果是 IDR 帧，先发送缓存的 VPS/SPS/PPS
			// if header.IsKeyFrame {
			// 	da.keyFrameMutex.RLock()
			// 	vps, sps, pps := da.LastVPS, da.LastSPS, da.LastPPS
			// 	da.keyFrameMutex.RUnlock()

			// 	if vps != nil {
			// 		if !yield(WebRTCFrame{Data: createCopy(vps), Timestamp: int64(header.PTS)}) {
			// 			return
			// 		}
			// 	}
			// 	if sps != nil {
			// 		if !yield(WebRTCFrame{Data: createCopy(sps), Timestamp: int64(header.PTS)}) {
			// 			return
			// 		}
			// 	}
			// 	if pps != nil {
			// 		if !yield(WebRTCFrame{Data: createCopy(pps), Timestamp: int64(header.PTS)}) {
			// 			return
			// 		}
			// 	}
			// }

			// 发送当前 NALU (Raw NALU)
			if !yield(sdriver.AVBox{
				Data:     nal,
				PTS:      time.Duration(header.PTS),
				IsConfig: isConfig,
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
