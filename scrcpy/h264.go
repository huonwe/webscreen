package scrcpy

import (
	"bytes"
	"iter"
	"log"
	"time"
)

func (da *DataAdapter) GenerateWebRTCFrameH264(header ScrcpyFrameHeader, payload []byte) iter.Seq[WebRTCFrame] {
	return func(yield func(WebRTCFrame) bool) {
		var startCode = []byte{0x00, 0x00, 0x00, 0x01}

		if payload[4]&0x1F == 7 {
			SPSData := []byte{}
			PPSData := []byte{}
			IDRData := []byte{}
			parts := bytes.Split(payload, startCode)
			for _, nal := range parts {
				if len(nal) == 0 {
					continue
				}
				nalType := nal[0] & 0x1F
				log.Printf("NALU Type: %d, size: %d", nalType, len(nal))
				switch nalType {
				case 7: // SPS
					da.updateVideoMetaFromSPS(nal)
					SPSData = nal
					da.keyFrameMutex.Lock()
					da.LastSPS = SPSData
					da.keyFrameMutex.Unlock()
					log.Println("SPS NALU processed, size:", len(SPSData))
				case 8: // PPS
					PPSData = nal
					da.keyFrameMutex.Lock()
					da.LastPPS = PPSData
					da.keyFrameMutex.Unlock()
					log.Println("PPS NALU processed, size:", len(PPSData))
				case 5: // IDR
					da.keyFrameMutex.Lock()
					IDRData = append(startCode, nal...)
					da.LastIDR = IDRData
					da.LastIDRTime = time.Now()
					da.keyFrameMutex.Unlock()
					log.Println("IDR NALU processed, size:", len(IDRData))
				}
			}
			// Yield Packets
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
			log.Println("Sent H264 keyframe NALUs: SPS, PPS, IDR")
			return // 已经处理完所有NALU，返回
		}

		// If it's a keyframe, send cached config first
		if header.IsKeyFrame {
			da.keyFrameMutex.RLock()
			da.LastIDR = payload
			da.LastIDRTime = time.Now()
			da.keyFrameMutex.RUnlock()

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
