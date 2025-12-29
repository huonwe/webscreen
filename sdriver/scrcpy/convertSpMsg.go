package scrcpy

import (
	"encoding/binary"
	"io"
	"log"
	"time"
	"webscreen/sdriver"
)

func (da *ScrcpyDriver) convertVideoFrame() {
	var headerBuf [12]byte
	header := ScrcpyFrameHeader{}

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
		// showFrameHeaderInfo(frame.Header)
		frameSize := int(header.Size)

		// 从 LinearBuffer 获取内存
		payloadBuf := da.videoBuffer.Get(frameSize)

		if _, err := io.ReadFull(da.videoConn, payloadBuf); err != nil {
			log.Println("Failed to read video frame payload:", err)
			return
		}
		if header.IsKeyFrame {
			go da.updateCache(payloadBuf, da.mediaMeta.VideoCodec)
		}
		webRTCFrame := sdriver.AVBox{
			Data:     payloadBuf,
			PTS:      time.Duration(header.PTS) * time.Microsecond,
			IsConfig: false,
		}
		da.LastPTS = webRTCFrame.PTS
		select {
		case da.VideoChan <- webRTCFrame:
		default:
			log.Println("Video channel full, waiting to send frame...")
			da.VideoChan <- webRTCFrame
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
		if header.IsConfig {
			log.Println("[scrcpy driver]Received audio config frame, skipping...")
			continue
		}

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
