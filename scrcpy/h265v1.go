package scrcpy

import (
	"bytes"
	"iter"
	"time"
)

func (da *DataAdapter) GenerateWebRTCFrameH265_v1(header ScrcpyFrameHeader, payload []byte) iter.Seq[WebRTCFrame] {
	return func(yield func(WebRTCFrame) bool) {
		var startCode = []byte{0x00, 0x00, 0x00, 0x01}

		// Parse NALUs to update cache
		parts := bytes.Split(payload, startCode)
		for _, part := range parts {
			if len(part) == 0 {
				continue
			}

			nalType := (part[0] >> 1) & 0x3F

			switch nalType {
			case 32: // VPS
				vpsData := append(startCode, part...)
				da.keyFrameMutex.Lock()
				da.LastVPS = createCopy(vpsData, &da.PayloadPoolLarge)
				da.keyFrameMutex.Unlock()
			case 33: // SPS
				// da.updateVideoMetaFromSPS(part)
				spsData := append(startCode, part...)
				da.keyFrameMutex.Lock()
				da.LastSPS = createCopy(spsData, &da.PayloadPoolLarge)
				da.keyFrameMutex.Unlock()
			case 34: // PPS
				ppsData := append(startCode, part...)
				da.keyFrameMutex.Lock()
				da.LastPPS = createCopy(ppsData, &da.PayloadPoolLarge)
				da.keyFrameMutex.Unlock()
			case 19, 20, 21: // IDR
				da.keyFrameMutex.Lock()
				idrData := append(startCode, part...)
				da.LastIDR = createCopy(idrData, &da.PayloadPoolLarge)
				da.LastIDRTime = time.Now()
				da.keyFrameMutex.Unlock()
			}
		}

		// If it's a keyframe (but not a config frame itself), send cached config first
		if header.IsKeyFrame && !header.IsConfig {
			da.keyFrameMutex.RLock()
			lastVPS := da.LastVPS
			lastSPS := da.LastSPS
			lastPPS := da.LastPPS
			da.keyFrameMutex.RUnlock()

			if lastVPS != nil {
				if !yield(WebRTCFrame{Data: createCopy(lastVPS, &da.PayloadPoolLarge), Timestamp: int64(header.PTS)}) {
					return
				}
			}
			if lastSPS != nil {
				if !yield(WebRTCFrame{Data: createCopy(lastSPS, &da.PayloadPoolLarge), Timestamp: int64(header.PTS)}) {
					return
				}
			}
			if lastPPS != nil {
				if !yield(WebRTCFrame{Data: createCopy(lastPPS, &da.PayloadPoolLarge), Timestamp: int64(header.PTS)}) {
					return
				}
			}
		}

		if !yield(WebRTCFrame{
			Data:      payload,
			Timestamp: int64(header.PTS),
		}) {
			return
		}
	}
}
