package scrcpy

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// type StreamChan struct {
// 	VideoChan        chan WebRTCVideoFrame
// 	AudioChan        chan WebRTCAudioFrame
// 	ControlChan      chan WebRTCControlFrame
// 	PayloadPoolLarge sync.Pool
// }

type DataAdapter struct {
	// VideoChanMutex   sync.RWMutex
	// AudioChanMutex   sync.RWMutex
	VideoChan        chan WebRTCFrame
	AudioChan        chan WebRTCFrame
	ControlChan      chan WebRTCControlFrame
	PayloadPoolLarge sync.Pool
	PayloadPoolSmall sync.Pool

	DeviceName string
	VideoMeta  ScrcpyVideoMeta
	AudioMeta  ScrcpyAudioMeta

	// convertVideoPaused bool
	// convertAudioPaused bool

	videoConn   net.Conn
	audioConn   net.Conn
	controlConn net.Conn

	adbClient *ADBClient

	// keyFrameRequestMutex    sync.Mutex
	lastIDRRequestTime time.Time

	keyFrameMutex sync.RWMutex // 保护 LastSPS, LastPPS, LastIDR
	LastVPS       []byte       // 新增：H.265 VPS
	LastSPS       []byte
	LastPPS       []byte
	LastIDR       []byte
	LastIDRTime   time.Time
	// LastPFrames   [][]byte
}

// 一个DataAdapter对应一个scrcpy实例，通过本地端口建立三个连接：视频、音频、控制
func NewDataAdapter(config map[string]string) (*DataAdapter, error) {
	var err error
	da := &DataAdapter{
		adbClient: NewADBClient(config["device_serial"]),
		// convertVideoPaused: false,
		// convertAudioPaused: false,

		VideoChan:   make(chan WebRTCFrame, 10),
		AudioChan:   make(chan WebRTCFrame, 10),
		ControlChan: make(chan WebRTCControlFrame, 10),
		PayloadPoolLarge: sync.Pool{
			New: func() interface{} {
				// 预分配 512KB (根据你的 码率调整)
				return make([]byte, 512*1024)
			},
		},
		PayloadPoolSmall: sync.Pool{
			New: func() interface{} {
				// 1KB 足够放下大多数 Opus 帧
				return make([]byte, 1024)
			},
		},
	}
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
	// 甜点值 64KB ~ 128KB
	da.videoConn.(*net.TCPConn).SetReadBuffer(1 * 1024 * 1024)
	da.audioConn.(*net.TCPConn).SetReadBuffer(64 * 1024)

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
		var headerBuf [12]byte
		frame := &ScrcpyFrame{}

		isH265 := da.VideoMeta.CodecID == "h265"

		for {
			// read frame header
			if err := readScrcpyFrameHeader(da.videoConn, headerBuf[:], &frame.Header); err != nil {
				log.Println("Failed to read scrcpy frame header:", err)
				return
			}
			// showFrameHeaderInfo(frame.Header)
			payloadBuf := da.PayloadPoolLarge.Get().([]byte)
			// log.Printf("payload size: %v,PayloadPoolLarge capacity: %v", frame.Header.Size, cap(payloadBuf))
			if cap(payloadBuf) < int(frame.Header.Size) {
				log.Printf("resize video payload buf, current cap: %v payloadbuf: %v", cap(payloadBuf), cap(payloadBuf))
				da.PayloadPoolLarge.Put(payloadBuf)
				newSize := int(frame.Header.Size) + 1024
				if newSize < 512*1024 {
					newSize = 512 * 1024
				}
				payloadBuf = make([]byte, newSize)
			}
			if _, err := io.ReadFull(da.videoConn, payloadBuf[:frame.Header.Size]); err != nil {
				log.Println("Failed to read video frame payload:", err)
				return
			}
			frameData := payloadBuf[:frame.Header.Size]

			var iter func(func(WebRTCFrame) bool)
			if isH265 {
				iter = da.GenerateWebRTCFrameH265(frame.Header, frameData)
			} else {
				iter = da.GenerateWebRTCFrameH264v1(frame.Header, frameData)
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
	}()
}

func (da *DataAdapter) StartConvertAudioFrame() {
	go func() {
		var headerBuf [12]byte
		frame := &ScrcpyFrame{}
		for {
			// read frame header
			if err := readScrcpyFrameHeader(da.audioConn, headerBuf[:], &frame.Header); err != nil {
				log.Println("Failed to read scrcpy audio frame header:", err)
				return
			}
			// log.Printf("Audio Frame Timestamp: %v, Size: %v isConfig: %v\n", frame.Header.PTS, frame.Header.Size, frame.Header.IsConfig)
			payloadBuf := da.PayloadPoolSmall.Get().([]byte)
			if cap(payloadBuf) < int(frame.Header.Size) {
				log.Println("current buf cap:", cap(payloadBuf))
				if cap(payloadBuf) >= 1024 {
					da.PayloadPoolSmall.Put(payloadBuf)
				}
				payloadBuf = make([]byte, frame.Header.Size+1024)
				log.Println("Resized payload buffer for audio frame")
				log.Println("size:  ", frame.Header.Size)
			}
			// read frame payload
			n, _ := io.ReadFull(da.audioConn, payloadBuf[:frame.Header.Size])
			frameData := payloadBuf[:frame.Header.Size]

			if frame.Header.IsConfig {
				log.Println("Audio Config Frame Received")

				totalLen := 7 + 8 + int(n)
				configBuf := da.PayloadPoolSmall.Get().([]byte)
				if cap(configBuf) < totalLen {
					if cap(configBuf) >= 1024 {
						da.PayloadPoolSmall.Put(configBuf)
					}
					configBuf = make([]byte, 1024)
					if cap(configBuf) < totalLen {
						configBuf = make([]byte, totalLen+512)
					}
					log.Println("Resized config buffer for audio config frame")
				}
				configBuf = configBuf[:totalLen]
				copy(configBuf[0:7], []byte("AOPUSHC"))                   // Magic
				binary.LittleEndian.PutUint64(configBuf[7:15], uint64(n)) // Length
				copy(configBuf[15:], frameData)
				da.AudioChan <- WebRTCFrame{
					Data:      configBuf,
					Timestamp: int64(frame.Header.PTS),
				}
				da.PayloadPoolSmall.Put(payloadBuf)
				continue
			}

			// 这里的 Data 引用了 pool 中的内存，消费者用完必须 Put 回去
			webRTCFrame := WebRTCFrame{
				Data:      frameData,
				Timestamp: int64(frame.Header.PTS),
			}
			select {
			case da.AudioChan <- webRTCFrame:

			default:
				log.Println("Audio channel full, waiting to send frame...")
				da.AudioChan <- webRTCFrame
			}
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

func (da *DataAdapter) updateVideoMetaFromSPS(sps []byte) {
	if da.LastSPS != nil && bytes.Equal(da.LastSPS[4:], sps) {
		log.Println("SPS unchanged, no need to update video meta")
		return
	}
	spsInfo, err := ParseSPS(sps, true)
	if err != nil {
		log.Println("Failed to parse SPS for video meta update:", err)
		return
	}
	da.VideoMeta.Width = spsInfo.Width
	da.VideoMeta.Height = spsInfo.Height
	log.Printf("Updated Video Meta from SPS: Width=%d, Height=%d", da.VideoMeta.Width, da.VideoMeta.Height)
}

func readScrcpyFrameHeader(conn net.Conn, headerBuf []byte, header *ScrcpyFrameHeader) error {
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

func showFrameHeaderInfo(header ScrcpyFrameHeader) {
	log.Printf("Frame Header - PTS: %d, Size: %d, IsConfig: %v, IsKeyFrame: %v",
		header.PTS, header.Size, header.IsConfig, header.IsKeyFrame)
}

func createCopy(src []byte, pool *sync.Pool) []byte {
	if len(src) == 0 {
		log.Println("createCopy called with empty src")
		return nil
	}
	dst := pool.Get().([]byte)
	// log.Printf("current pool cap: %d, src size: %d", cap(dst), len(src))
	if cap(dst) < len(src) {
		log.Printf("resize pool buffer, current cap:%v < src size:%v", cap(dst), len(src))
		if len(src) <= 1024 {
			log.Fatalln("!!!!!!!")
		}
		// pool.Put(dst)
		dst = make([]byte, len(src)+1024)
	}
	copy(dst, src)
	dst = dst[:len(src)]
	return dst
}
