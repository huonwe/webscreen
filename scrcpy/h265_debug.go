package scrcpy

import (
	"bytes"
	"iter"
	"log"
	"sync"
	"time"
)

func (da *DataAdapter) GenerateWebRTCFrameH265_debug(header ScrcpyFrameHeader, payload []byte) iter.Seq[WebRTCFrame] {
	return func(yield func(WebRTCFrame) bool) {
		startCode := []byte{0x00, 0x00, 0x00, 0x01}

		// 使用 bytes.Split 进行拆包，这是最稳妥的方式
		parts := bytes.Split(payload, startCode)

		for i, nal := range parts {
			if len(nal) == 0 {
				continue
			}

			// H.265 NALU Header: F(1) + Type(6) + LayerId(6) + TID(3)
			// Type 在第一个字节的中间 6 位
			nalType := (nal[0] >> 1) & 0x3F
			log.Printf("Debug H265: Part %d, Type: %d, Size: %d", i, nalType, len(nal))

			var pool *sync.Pool
			isConfig := false

			switch nalType {
			case 32: // VPS
				da.keyFrameMutex.Lock()
				da.LastVPS = nal
				da.keyFrameMutex.Unlock()
				isConfig = true
			case 33: // SPS
				// da.updateVideoMetaFromSPS(nal, "h265") // H265 SPS 解析比较复杂，暂时注释
				da.keyFrameMutex.Lock()
				da.LastSPS = nal
				da.keyFrameMutex.Unlock()
				isConfig = true
			case 34: // PPS
				da.keyFrameMutex.Lock()
				da.LastPPS = nal
				da.keyFrameMutex.Unlock()
				isConfig = true
			// case 39, 40: // SEI
			// 	isConfig = true
			case 19, 20, 21: // IDR
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

			// 发送当前 NALU (Raw NALU)
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
