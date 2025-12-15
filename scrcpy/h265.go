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

		if (payload[4]>>1)&0x3F == 32 {
			VPSData := []byte{}
			SPSData := []byte{}
			PPSData := []byte{}
			IDRData := []byte{}
			parts := bytes.Split(payload, startCode)
			for _, nal := range parts {
				if len(nal) == 0 {
					continue
				}
				nalType := (nal[0] >> 1) & 0x3F
				log.Printf("NALU Type: %d, size: %d", nalType, len(nal))
				switch nalType {
				case 32: // VPS
					VPSData = nal
					da.keyFrameMutex.Lock()
					da.LastVPS = VPSData
					da.keyFrameMutex.Unlock()
					// log.Println("VPS NALU processed, size:", len(VPSData))
				case 33: // SPS
					// da.updateVideoMetaFromSPS(nal)
					SPSData = nal
					da.keyFrameMutex.Lock()
					da.LastSPS = SPSData
					da.keyFrameMutex.Unlock()
					// log.Println("SPS NALU processed, size:", len(SPSData))
				case 34: // PPS
					PPSData = nal
					da.keyFrameMutex.Lock()
					da.LastPPS = PPSData
					da.keyFrameMutex.Unlock()
					// log.Println("PPS NALU processed, size:", len(PPSData))
				case 19, 20, 21: // IDR
					da.keyFrameMutex.Lock()
					IDRData = append(startCode, nal...)
					da.LastIDR = IDRData
					da.LastIDRTime = time.Now()
					da.keyFrameMutex.Unlock()
					// log.Println("IDR NALU processed, size:", len(IDRData))
				}
			}
			// Yield Packets
			if len(VPSData) > 0 {
				if !yield(WebRTCFrame{Data: createCopy(VPSData, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
					return
				}
			}
			if len(SPSData) > 0 {
				if !yield(WebRTCFrame{Data: createCopy(SPSData, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
					return
				}
			}
			if len(PPSData) > 0 {
				if !yield(WebRTCFrame{Data: createCopy(PPSData, &da.PayloadPoolSmall), Timestamp: int64(header.PTS)}) {
					return
				}
			}
			if len(IDRData) > 0 {
				if !yield(WebRTCFrame{Data: createCopy(IDRData, &da.PayloadPoolLarge), Timestamp: int64(header.PTS), NotConfig: true}) {
					return
				}
			}
			log.Println("Sent H265 keyframe NALUs: VPS, SPS, PPS, IDR")
			return // 已经处理完所有NALU，返回
		}

		// If it's a keyframe, send cached config first
		if header.IsKeyFrame {
			da.keyFrameMutex.RLock()
			da.LastIDR = payload
			da.LastIDRTime = time.Now()
			da.keyFrameMutex.RUnlock()

			if da.LastVPS != nil {
				if !yield(WebRTCFrame{Data: createCopy(da.LastVPS, &da.PayloadPoolLarge), Timestamp: int64(header.PTS)}) {
					return
				}
			}
			if da.LastSPS != nil {
				if !yield(WebRTCFrame{Data: createCopy(da.LastSPS, &da.PayloadPoolLarge), Timestamp: int64(header.PTS)}) {
					return
				}
			}
			if da.LastPPS != nil {
				if !yield(WebRTCFrame{Data: createCopy(da.LastPPS, &da.PayloadPoolLarge), Timestamp: int64(header.PTS)}) {
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
