package scrcpy

import (
	"bytes"
	"iter"
	"sync"
	"time"
)

func (da *DataAdapter) GenerateWebRTCFrameH264_v2(header ScrcpyFrameHeader, payload []byte) iter.Seq[WebRTCFrame] {
	return func(yield func(WebRTCFrame) bool) {
		startCode := []byte{0x00, 0x00, 0x00, 0x01}

		// 核心修复：无条件拆分所有包，解决 SEI+IDR 粘包问题
		parts := bytes.Split(payload, startCode)

		for _, nal := range parts {
			if len(nal) == 0 {
				continue
			}

			nalType := nal[0] & 0x1F
			// log.Printf("Debug H264_v2: Part %d, Type: %d, Size: %d", i, nalType, len(nal))

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
					// 直接发送 Raw NALU (不带起始码)
					// streamManager 会检查前4字节，如果不是 00000001，就会直接发送数据，这正是我们想要的
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
