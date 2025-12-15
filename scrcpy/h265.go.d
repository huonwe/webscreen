package scrcpy

import (
	"bytes"
	"iter"
	"log"
	"time"
)

func (da *DataAdapter) GenerateWebRTCFrameH265(header ScrcpyFrameHeader, payload []byte) iter.Seq[WebRTCFrame] {
	return func(yield func(WebRTCFrame) bool) {
		var startCode = []byte{0x00, 0x00, 0x00, 0x01}

		if (payload[4]>>1)&0x3F == 32 || (payload[4]>>1)&0x3F == 39 {
			parts := bytes.Split(payload, startCode)
			for i, nal := range parts {
				if len(nal) == 0 {
					continue
				}
				nalType := (nal[0] >> 1) & 0x3F
				log.Printf("%v NALU Type: %d, size: %d", i, nalType, len(nal))
				switch nalType {
				case 32: // VPS
					da.keyFrameMutex.Lock()
					da.LastVPS = nal
					da.keyFrameMutex.Unlock()
					if !yield(WebRTCFrame{Data: createCopy(nal, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
						return
					}
					// log.Println("VPS NALU processed, size:", len(VPSData))
				case 33: // SPS
					// da.updateVideoMetaFromSPS(nal, "h265")
					da.keyFrameMutex.Lock()
					da.LastSPS = nal
					da.keyFrameMutex.Unlock()
					// log.Println("SPS NALU processed, size:", len(SPSData))
					if len(nal) > 0 {
						if !yield(WebRTCFrame{Data: createCopy(nal, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
							return
						}
					}
				case 34: // PPS
					da.keyFrameMutex.Lock()
					da.LastPPS = nal
					da.keyFrameMutex.Unlock()
					// log.Println("PPS NALU processed, size:", len(PPSData))
					if len(nal) > 0 {
						if !yield(WebRTCFrame{Data: createCopy(nal, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
							return
						}
					}
				case 19, 20, 21: // IDR
					da.keyFrameMutex.Lock()
					da.LastIDR = nal
					da.LastIDRTime = time.Now()
					da.keyFrameMutex.Unlock()
					// log.Println("IDR NALU processed, size:", len(IDRData))
					if len(nal) > 0 {
						if !yield(WebRTCFrame{Data: createCopy(nal, &da.PayloadPoolLarge), Timestamp: int64(header.PTS), NotConfig: true}) {
							return
						}
					}
				default:
					if !yield(WebRTCFrame{Data: createCopy(nal, &da.PayloadPoolLarge), Timestamp: int64(header.PTS), NotConfig: true}) {
						return
					}
				}
			}
			return // 已经处理完所有NALU，返回
		}

		// If it's a keyframe, send cached config first
		if header.IsKeyFrame {
			da.keyFrameMutex.Lock()
			da.LastIDR = payload
			da.LastIDRTime = time.Now()
			da.keyFrameMutex.Unlock()

			if da.LastVPS != nil {
				if !yield(WebRTCFrame{Data: createCopy(da.LastVPS, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
					return
				}
			}
			if da.LastSPS != nil {
				if !yield(WebRTCFrame{Data: createCopy(da.LastSPS, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
					return
				}
			}
			if da.LastPPS != nil {
				if !yield(WebRTCFrame{Data: createCopy(da.LastPPS, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
					return
				}
			}
		}

		if !yield(WebRTCFrame{
			Data:      payload,
			Timestamp: int64(header.PTS),
			NotConfig: true,
		}) {
			return
		}
	}
}
