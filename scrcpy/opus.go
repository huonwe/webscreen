package scrcpy

import (
	"bytes"
	"encoding/binary"
	"iter"
	"log"
)

// GenerateWebRTCFrameOpus 处理 Opus 音频帧
// Opus 帧通常不需要像 H.264/H.265 那样拆包，但需要处理 Config 帧
func (da *DataAdapter) GenerateWebRTCFrameOpus(header ScrcpyFrameHeader, payload []byte) iter.Seq[WebRTCFrame] {
	return func(yield func(WebRTCFrame) bool) {
		if header.IsConfig {
			log.Println("Audio Config Frame Received")

			n := len(payload)
			totalLen := 7 + 8 + n
			configBuf := make([]byte, totalLen) // Config 帧很少，直接分配

			copy(configBuf[0:7], []byte("AOPUSHC"))                   // Magic
			binary.LittleEndian.PutUint64(configBuf[7:15], uint64(n)) // Length
			copy(configBuf[15:], payload)

			yield(WebRTCFrame{
				Data:      configBuf,
				Timestamp: int64(header.PTS),
			})
			return
		}

		// 普通音频帧，直接透传 (零拷贝)
		yield(WebRTCFrame{
			Data:      payload,
			Timestamp: int64(header.PTS),
			NotConfig: true,
		})
	}
}

func ParseOpusHead(data []byte) *OpusHead {
	var head OpusHead
	r := bytes.NewReader(data)

	binary.Read(r, binary.LittleEndian, &head.Magic)
	binary.Read(r, binary.LittleEndian, &head.Version)
	binary.Read(r, binary.LittleEndian, &head.Channels)
	binary.Read(r, binary.LittleEndian, &head.PreSkip)
	binary.Read(r, binary.LittleEndian, &head.SampleRate)
	binary.Read(r, binary.LittleEndian, &head.OutputGain)
	binary.Read(r, binary.LittleEndian, &head.Mapping)
	return &head
}
