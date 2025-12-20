package scrcpy

import (
	"encoding/binary"
	"io"
	"log"
	"webcpy/sdriver"
)

func (da *ScrcpyDriver) convertVideoFrame() {
	var headerBuf [12]byte
	frame := &ScrcpyFrame{}

	isH265 := da.mediaMeta.VideoCodecID == "h265"

	for {
		// read frame header
		if _, err := io.ReadFull(da.videoConn, headerBuf[:]); err != nil {
			log.Println("Failed to read scrcpy frame header:", err)
			return
		}
		if err := readScrcpyFrameHeader(headerBuf[:], &frame.Header); err != nil {
			log.Println("Failed to read scrcpy frame header:", err)
			return
		}
		// showFrameHeaderInfo(frame.Header)
		frameSize := int(frame.Header.Size)

		// 从 LinearBuffer 获取内存
		payloadBuf := da.videoBuffer.Get(frameSize)

		if _, err := io.ReadFull(da.videoConn, payloadBuf); err != nil {
			log.Println("Failed to read video frame payload:", err)
			return
		}

		var iter func(func(sdriver.AVBox) bool)
		if isH265 {
			iter = da.GenerateWebRTCFrameH265(frame.Header, payloadBuf)
		} else {
			// 	niltype := (frameData[4] >> 1) & 0x3F
			// 	log.Printf("(h265) NALU Type of first NALU in frame: %d; total size: %d", niltype, len(frameData))
			iter = da.GenerateWebRTCFrameH264(frame.Header, payloadBuf)
		}

		for webRTCFrame := range iter {
			select {
			case da.VideoChan <- webRTCFrame:
			default:
				log.Println("Video channel full, waiting to send frame...")
				da.VideoChan <- webRTCFrame
			}
		}
	}
}

func (da *ScrcpyDriver) convertAudioFrame() {
	var headerBuf [12]byte
	frame := &ScrcpyFrame{}
	for {
		// read frame header
		if _, err := io.ReadFull(da.audioConn, headerBuf[:]); err != nil {
			log.Println("Failed to read scrcpy frame header:", err)
			return
		}
		if err := readScrcpyFrameHeader(headerBuf[:], &frame.Header); err != nil {
			log.Println("Failed to read scrcpy audio frame header:", err)
			return
		}
		// log.Printf("Audio Frame Timestamp: %v, Size: %v isConfig: %v\n", frame.Header.PTS, frame.Header.Size, frame.Header.IsConfig)
		frameSize := int(frame.Header.Size)
		payloadBuf := da.audioBuffer.Get(frameSize)

		// read frame payload
		_, _ = io.ReadFull(da.audioConn, payloadBuf)

		for webRTCFrame := range da.GenerateWebRTCFrameOpus(frame.Header, payloadBuf) {
			select {
			case da.AudioChan <- webRTCFrame:
			default:
				log.Println("Audio channel full, waiting to send frame...")
				da.AudioChan <- webRTCFrame
			}
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
