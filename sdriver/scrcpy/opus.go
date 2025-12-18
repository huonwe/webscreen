package scrcpy

import (
	"bytes"
	"encoding/binary"
	"iter"
	"log"
	"time"
	"webcpy/sdriver"
)

// GenerateWebRTCFrameOpus 处理 Opus 音频帧
// Opus 帧通常不需要像 H.264/H.265 那样拆包，但需要处理 Config 帧
func (da *ScrcpyDriver) GenerateWebRTCFrameOpus(header ScrcpyFrameHeader, payload []byte) iter.Seq[sdriver.AVBox] {
	return func(yield func(sdriver.AVBox) bool) {
		if header.IsConfig {
			log.Println("Audio Config Frame Received")

			n := len(payload)
			totalLen := 7 + 8 + n
			configBuf := make([]byte, totalLen) // Config 帧很少，直接分配

			copy(configBuf[0:7], []byte("AOPUSHC"))                   // Magic
			binary.LittleEndian.PutUint64(configBuf[7:15], uint64(n)) // Length
			copy(configBuf[15:], payload)

			yield(sdriver.AVBox{
				Data:     configBuf,
				PTS:      time.Duration(header.PTS),
				IsConfig: true,
			})
			return
		}

		// 普通音频帧，直接透传 (零拷贝)
		yield(sdriver.AVBox{
			Data:     payload,
			PTS:      time.Duration(header.PTS),
			IsConfig: false,
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
