package scrcpy

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"sync"
)

// type StreamChan struct {
// 	VideoChan        chan WebRTCVideoFrame
// 	AudioChan        chan WebRTCAudioFrame
// 	ControlChan      chan WebRTCControlFrame
// 	VideoPayloadPool sync.Pool
// }

type DataAdapter struct {
	VideoChan        chan WebRTCVideoFrame
	AudioChan        chan WebRTCAudioFrame
	ControlChan      chan WebRTCControlFrame
	VideoPayloadPool sync.Pool

	DeviceName string
	VideoMeta  ScrcpyVideoMeta
	AudioMeta  ScrcpyAudioMeta

	videoConn   net.Conn
	audioConn   net.Conn
	controlConn net.Conn

	adbClient *ADBClient
}

// 一个DataAdapter对应一个scrcpy实例，通过本地端口建立三个连接：视频、音频、控制
func NewDataAdapter(config map[string]string) (*DataAdapter, error) {
	var err error
	da := &DataAdapter{}
	da.adbClient = NewADBClient(config["device_serial"])
	err = da.adbClient.Push(config["server_local_path"], config["server_remote_path"])
	if err != nil {
		log.Fatalf("设置 推送scrcpy-server失败: %v", err)
	}
	err = da.adbClient.Reverse("localabstract:scrcpy", "tcp:"+config["local_port"])
	if err != nil {
		log.Fatalf("设置 Reverse 隧道失败: %v", err)
		return nil, err
	}
	da.adbClient.StartScrcpyServer()
	listener, err := net.Listen("tcp", ":"+config["local_port"])
	if err != nil {
		log.Fatalf("监听端口失败: %v", err)
		return nil, err
	}
	defer listener.Close()
	conns := make([]net.Conn, 3)
	for i := 0; i < 3; i++ {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("Accept 失败: %v", err)
			return nil, err
		}
		log.Println("Accept Connection", i)
		conns[i] = conn
	}

	for i, conn := range conns {
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			tcpConn.SetReadBuffer(2 * 1024 * 1024)
		}
		// The target environment is ARM devices, read directly without buffering could be faster because of less memory copy
		// conn := comm.NewBufferedReadWriteCloser(_conn, 4096)
		switch i {
		case 0:
			// Device Metadata and Video
			err = da.readDeviceMeta(conn)
			if err != nil {
				log.Fatalln("Failed to read device metadata:", err)
				return nil, err
			}
			log.Printf("Connected Device: %s", da.DeviceName)

			da.assignConn(conn)
		case 1:
			da.assignConn(conn)
		case 2:
			// The third connection is always Control
			da.controlConn = conn
			log.Println("Scrcpy Control Connection Established")
		}
	}
	da.VideoChan = make(chan WebRTCVideoFrame, 100)
	da.AudioChan = make(chan WebRTCAudioFrame, 100)
	da.ControlChan = make(chan WebRTCControlFrame, 100)

	da.VideoPayloadPool = sync.Pool{
		New: func() interface{} {
			// 预分配 1MB (根据你的 H264 码率调整)
			return make([]byte, 1024*1024)
		},
	}
	return da, nil
}

func (da *DataAdapter) Close() {
	if da.videoConn != nil {
		da.videoConn.Close()
	}
	if da.audioConn != nil {
		da.audioConn.Close()
	}
	if da.controlConn != nil {
		da.controlConn.Close()
	}
	da.adbClient.ReverseRemove("localabstract:scrcpy")
	da.adbClient.Stop()
	// close(da.VideoChan)
	// close(da.AudioChan)
}

func (da *DataAdapter) ShowDeviceInfo() {
	log.Printf("Device Name: %s", da.DeviceName)
	log.Printf("Video Codec: %s, Width: %d, Height: %d", da.VideoMeta.CodecID, da.VideoMeta.Width, da.VideoMeta.Height)
	log.Printf("Audio Codec: %s", da.AudioMeta.CodecID)
}

func (da *DataAdapter) StartConvertVideoFrame() {
	go func() {
		var startCode = []byte{0x00, 0x00, 0x00, 0x01}
		// var lastPTS uint64 = 0
		var sendPTS uint64 = 0
		var cachedSPS []byte
		var cachedPPS []byte

		var headerBuf [12]byte
		frame := &ScrcpyVideoFrame{}
		for {
			// read frame header
			if err := readScrcpyFrameHeader(da.videoConn, headerBuf[:], &frame.Header); err != nil {
				log.Println("Failed to read scrcpy frame header:", err)
				return
			}
			payloadBuf := da.VideoPayloadPool.Get().([]byte)
			if cap(payloadBuf) < int(frame.Header.Size) {
				da.VideoPayloadPool.Put(&payloadBuf)
				payloadBuf = make([]byte, frame.Header.Size+1024)
			}
			if _, err := io.ReadFull(da.videoConn, payloadBuf[:frame.Header.Size]); err != nil {
				log.Println("Failed to read video frame payload:", err)
				return
			}
			frameData := payloadBuf[:frame.Header.Size]
			nilType := frameData[4] & 0x1F
			if nilType == 7 {
				SPS_PPS_Frame := bytes.Split(frameData, startCode)
				if cachedSPS != nil {
					if !bytes.Equal(cachedSPS, SPS_PPS_Frame[1]) {
						log.Println("SPS Changed")
						pspInfo, _ := ParseSPS(SPS_PPS_Frame[1], true)
						log.Printf("New SPS Info - Width: %d, Height: %d, FrameRate: %.2f, Profile: %d, Level: %s",
							pspInfo.Width, pspInfo.Height, pspInfo.FrameRate, pspInfo.Profile, pspInfo.Level)
						// log.Fatalln("Video resolution changed, exiting...")
					}
				}
				cachedSPS = append(startCode, SPS_PPS_Frame[1]...)
				cachedPPS = append(startCode, SPS_PPS_Frame[2]...)
				log.Println("Cached SPS and PPS")
				pspInfo, _ := ParseSPS(SPS_PPS_Frame[1], true)
				log.Printf("SPS Info - Width: %d, Height: %d, FrameRate: %.2f, Profile: %d, Level: %s",
					pspInfo.Width, pspInfo.Height, pspInfo.FrameRate, pspInfo.Profile, pspInfo.Level)
				//
				sendPTS = frame.Header.PTS
				da.VideoChan <- WebRTCVideoFrame{
					Data:      cachedSPS,
					Timestamp: int64(sendPTS),
				}
				da.VideoChan <- WebRTCVideoFrame{
					Data:      cachedPPS,
					Timestamp: int64(sendPTS),
				}
				da.VideoPayloadPool.Put(payloadBuf)
				continue
			}
			if frame.Header.IsKeyFrame {
				da.VideoChan <- WebRTCVideoFrame{
					Data:      cachedSPS,
					Timestamp: int64(frame.Header.PTS),
				}
				da.VideoChan <- WebRTCVideoFrame{
					Data:      cachedPPS,
					Timestamp: int64(frame.Header.PTS),
				}
			}
			// lastPTS = frame.Header.PTS

			// SPS/PPS 粘包处理
			// 00 00 00 01 [SPS数据] 00 00 00 01 [PPS数据]

			// 这里的 Data 引用了 pool 中的内存，消费者用完必须 Put 回去
			webRTCFrame := WebRTCVideoFrame{
				Data:      frameData,
				Timestamp: int64(frame.Header.PTS),
			}
			da.VideoChan <- webRTCFrame
		}
	}()
}

func (da *DataAdapter) assignConn(conn net.Conn) error {
	codecID := ReadCodecID(conn)
	switch codecID {
	case "h264", "h265", "av1 ":
		da.videoConn = conn
		da.VideoMeta.CodecID = codecID
		err := da.readVideoMeta(conn)
		if err != nil {
			log.Fatalln("Failed to read video metadata:", err)
			return err
		}
		log.Println("Scrcpy Video Connection Established")
	case "aac ", "opus":
		da.audioConn = conn
		da.AudioMeta.CodecID = codecID
		log.Println("Audio Connection Established")
	default:
		da.controlConn = conn
		log.Println("Scrcpy Control Connection Established")
	}
	return nil
}

func (da *DataAdapter) readDeviceMeta(conn net.Conn) error {
	// 1. Device Name (64 bytes)
	nameBuf := make([]byte, 64)
	_, err := io.ReadFull(conn, nameBuf)
	if err != nil {
		return err
	}
	da.DeviceName = string(nameBuf)
	return nil
}

func ReadCodecID(conn net.Conn) string {
	// Codec ID (4 bytes)
	codecBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, codecBuf); err != nil {
		log.Println("Failed to read codec ID:", err)
		return ""
	}

	return string(codecBuf)
}

func (da *DataAdapter) readVideoMeta(conn net.Conn) error {
	// Width (4 bytes)
	// Height (4 bytes)
	// Codec 已经在外面读取过了，用于确认是哪个通道
	metaBuf := make([]byte, 8)
	if _, err := io.ReadFull(conn, metaBuf); err != nil {
		log.Println("Failed to read metadata:", err)
		return err
	}
	// 解析元数据
	da.VideoMeta.Width = binary.BigEndian.Uint32(metaBuf[0:4])
	da.VideoMeta.Height = binary.BigEndian.Uint32(metaBuf[4:8])

	return nil
}

func readScrcpyFrameHeader(conn net.Conn, headerBuf []byte, header *ScrcpyVideoFrameHeader) error {
	if _, err := io.ReadFull(conn, headerBuf); err != nil {
		return err
	}

	ptsAndFlags := binary.BigEndian.Uint64(headerBuf[0:8])
	packetSize := binary.BigEndian.Uint32(headerBuf[8:12])

	// 提取标志位
	isConfig := (ptsAndFlags & 0x8000000000000000) != 0
	isKeyFrame := (ptsAndFlags & 0x4000000000000000) != 0

	// 提取PTS (低62位)
	pts := uint64(ptsAndFlags & 0x3FFFFFFFFFFFFFFF)
	header.IsConfig = isConfig
	header.IsKeyFrame = isKeyFrame
	header.PTS = pts
	header.Size = packetSize
	return nil
}
