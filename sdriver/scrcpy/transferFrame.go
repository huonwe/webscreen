package scrcpy

import (
	"encoding/binary"
	"io"
	"log"
	"time"
	// "bytes"
	"webscreen/sdriver"
)

func (da *ScrcpyDriver) convertVideoFrame() {
	var headerBuf [12]byte
	header := ScrcpyFrameHeader{}
	var nalTypeF func(byte) byte
	var nalType byte
	for {
		// read frame header
		if _, err := io.ReadFull(da.videoConn, headerBuf[:]); err != nil {
			log.Println("Failed to read scrcpy frame header:", err)
			return
		}
		if err := readScrcpyFrameHeader(headerBuf[:], &header); err != nil {
			log.Println("Failed to read scrcpy frame header:", err)
			return
		}
		da.LastPTS = time.Duration(header.PTS) * time.Microsecond
		// showFrameHeaderInfo(frame.Header)
		frameSize := int(header.Size)

		// 从 LinearBuffer 获取内存
		payloadBuf := da.videoBuffer.Get(frameSize)

		if _, err := io.ReadFull(da.videoConn, payloadBuf); err != nil {
			log.Println("Failed to read video frame payload:", err)
			return
		}
		switch da.mediaMeta.VideoCodec {
			case "h265":
				nalTypeF = func(payloadBuf byte) byte { return (payloadBuf >> 1) & 0x3F }
			case "h264":
				nalTypeF = func(payloadBuf byte) byte { return payloadBuf & 0x1F }
			default:
				log.Println("Unknown codec type for NALU parsing:", da.mediaMeta.VideoCodec)
				continue
		}
		nalType = nalTypeF(payloadBuf[4]) // 注意：payloadBuf 前 4 字节是起始码
		// log.Printf("ScrcpyDriver: isKeyFrame=%v, nal Type=%v, Size=%d bytes\n", header.IsKeyFrame, nalType, len(payloadBuf))
		// parts := bytes.Split(payloadBuf, []byte{0x00, 0x00, 0x00, 0x01})
		// for _, part := range parts {
		// 	if len(part) == 0 {
		// 		continue
		// 	}
		// 	log.Printf("NALU Part: nalType=%v, Size=%d bytes\n", nalTypeF(part[0]), len(part))
		// }
		if header.IsKeyFrame {
			switch nalType {
			case 5, 19, 20, 21: // H.264 IDR / H.265 IDR_W_RADL
				da.sendWithCachedConfigFrame(da.LastPTS, payloadBuf)
				da.LastIDR = createCopy(payloadBuf[4:]) // 去掉起始码
				continue
			case 6, 39, 40: // H.264 SEI / H.265 Prefix/Suffix SEI
				payloadBuf = PruneSEI(payloadBuf, da.mediaMeta.VideoCodec)
				da.sendWithCachedConfigFrame(da.LastPTS, payloadBuf)
				continue
			case 7, 32: // H.264 SPS / H.265 VPS
				go da.updateCache(payloadBuf, da.mediaMeta.VideoCodec)
				da.VideoChan <- sdriver.AVBox{
					Data:       payloadBuf,
					PTS:        da.LastPTS,
					IsKeyFrame: true,
					IsConfig:   false,
				}
				continue
			default:
				continue
			}
		}
		switch nalType {
		case 7, 32: // H.264 SPS / H.265 VPS
			go da.updateCache(payloadBuf, da.mediaMeta.VideoCodec)
			continue
		}

		select {
		case da.VideoChan <- sdriver.AVBox{
			Data:       payloadBuf[4:],
			PTS:        da.LastPTS,
			IsConfig:   false,
			IsKeyFrame: false,
		}:
		default:
			log.Println("Video channel full, skip...")
		}
	}
}

func (da *ScrcpyDriver) convertAudioFrame() {
	var headerBuf [12]byte
	header := ScrcpyFrameHeader{}
	for {
		// read frame header
		if _, err := io.ReadFull(da.audioConn, headerBuf[:]); err != nil {
			log.Println("Failed to read scrcpy frame header:", err)
			return
		}
		if err := readScrcpyFrameHeader(headerBuf[:], &header); err != nil {
			log.Println("Failed to read scrcpy audio frame header:", err)
			return
		}
		frameSize := int(header.Size)
		payloadBuf := da.audioBuffer.Get(frameSize)

		// read frame payload
		_, _ = io.ReadFull(da.audioConn, payloadBuf)
		// if header.IsConfig {
		// 	log.Println("[scrcpy driver]Received audio config frame, skipping...")
		// 	continue
		// }

		da.AudioChan <- sdriver.AVBox{
			Data:     payloadBuf,
			PTS:      time.Duration(header.PTS) * time.Microsecond,
			IsConfig: false,
		}

	}
}

func (da *ScrcpyDriver) transferControlMsg() {
	header := make([]byte, 5) // Type (1) + Length (4)
	for {
		_, err := io.ReadFull(da.controlConn, header)
		if err != nil {
			log.Println("Control connection read error:", err)
			return
		}

		msgType := header[0]
		length := binary.BigEndian.Uint32(header[1:])

		switch msgType {
		case DEVICE_MSG_TYPE_CLIPBOARD:
			content := make([]byte, length)
			_, err := io.ReadFull(da.controlConn, content)
			if err != nil {
				log.Println("Control connection read content error:", err)
				return
			}
			da.ControlChan <- sdriver.ReceiveClipboardEvent{
				Content: content,
			}
		default:
			// Skip unknown message
			if length > 0 {
				io.CopyN(io.Discard, da.controlConn, int64(length))
			}
		}
	}
}
