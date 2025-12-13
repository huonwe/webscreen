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
	VideoChan        chan WebRTCFrame
	AudioChan        chan WebRTCFrame
	ControlChan      chan WebRTCControlFrame
	VideoPayloadPool sync.Pool
	AudioPayloadPool sync.Pool

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
			tcpConn.SetReadBuffer(512 * 1024)
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
	da.VideoChan = make(chan WebRTCFrame, 20)
	da.AudioChan = make(chan WebRTCFrame, 20)
	da.ControlChan = make(chan WebRTCControlFrame, 10)

	da.VideoPayloadPool = sync.Pool{
		New: func() interface{} {
			// 预分配 512KB (根据你的 H264 码率调整)
			return make([]byte, 512*1024)
		},
	}
	da.AudioPayloadPool = sync.Pool{
		New: func() interface{} {
			// 4KB 足够放下任何 Opus 帧，甚至 AAC 帧
			return make([]byte, 4*1024)
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
	createCopy := func(src []byte) []byte {
		if len(src) == 0 {
			return nil
		}
		// A. 从池子拿（准备做复印件的纸）
		dst := da.VideoPayloadPool.Get().([]byte)

		// B. 容量检查
		// 如果池子里的纸太小（SPS通常很小，这种情况极少发生，但为了健壮性必须写）
		if cap(dst) < len(src) {
			// 把太小的还回去
			da.VideoPayloadPool.Put(dst)
			// 重新造个大的（这次 GC 无法避免，但仅限初始化阶段，无所谓）
			dst = make([]byte, len(src))
			log.Println("resize")
		}

		// C. 设定长度并拷贝
		dst = dst[:len(src)]
		copy(dst, src)
		return dst
	}
	go func() {
		var startCode = []byte{0x00, 0x00, 0x00, 0x01}
		// var lastPTS uint64 = 0
		var sendPTS uint64 = 0
		var cachedSPS []byte
		var cachedPPS []byte

		// lastNilType := uint8(0)

		var headerBuf [12]byte
		frame := &ScrcpyFrame{}
		for {
			// read frame header
			if err := readScrcpyFrameHeader(da.videoConn, headerBuf[:], &frame.Header); err != nil {
				log.Println("Failed to read scrcpy frame header:", err)
				return
			}
			payloadBuf := da.VideoPayloadPool.Get().([]byte)
			if cap(payloadBuf) < int(frame.Header.Size) {
				da.VideoPayloadPool.Put(payloadBuf)
				payloadBuf = make([]byte, frame.Header.Size+1024)
				log.Println("Resized payload buffer for video frame")
				log.Println("size:  ", frame.Header.Size)
			}
			if _, err := io.ReadFull(da.videoConn, payloadBuf[:frame.Header.Size]); err != nil {
				log.Println("Failed to read video frame payload:", err)
				return
			}
			frameData := payloadBuf[:frame.Header.Size]
			nilType := frameData[4] & 0x1F
			// fmt.Printf("Frame Timestamp: %v, Size: %v nilType: %v isKeyFrame: %v isConfig: %v\n", frame.Header.PTS, len(frameData), nilType, frame.Header.IsKeyFrame, frame.Header.IsConfig)
			if nilType == 7 {
				log.Println("SPS Frame Received")
				SPS_PPS_Frame := bytes.Split(frameData, startCode)
				// log.Println("len of SPS_PPS_Frame:", len(SPS_PPS_Frame))
				// for i, data := range SPS_PPS_Frame {
				// 	if len(data) > 0 {
				// 		// 打印 NAL type (data[0] & 0x1F)
				// 		log.Printf("Index %d: Len=%d, NAL Type=%d", i, len(data), data[0]&0x1F)
				// 	} else {
				// 		log.Printf("Index %d: Empty (StartCode at beginning)", i)
				// 	}
				// }
				if cachedSPS != nil {
					if !bytes.Equal(cachedSPS, append(startCode, SPS_PPS_Frame[1]...)) {

						pspInfo, _ := ParseSPS(SPS_PPS_Frame[1], true)
						log.Printf("New SPS Info - Width: %d, Height: %d, FrameRate: %.2f, Profile: %d, Level: %s",
							pspInfo.Width, pspInfo.Height, pspInfo.FrameRate, pspInfo.Profile, pspInfo.Level)
						// log.Fatalln("Video resolution changed, exiting...")
						cachedSPS = append(startCode, SPS_PPS_Frame[1]...)
						cachedPPS = append(startCode, SPS_PPS_Frame[2]...)
						log.Println("New SPS Cached")
					}
				} else {
					cachedSPS = append(startCode, SPS_PPS_Frame[1]...)
					cachedPPS = append(startCode, SPS_PPS_Frame[2]...)
					log.Println("First SPS PPS Cached")
				}
				log.Println("Sending SPS and PPS")
				sendPTS = frame.Header.PTS
				SPSCopy := createCopy(cachedSPS)
				da.VideoChan <- WebRTCFrame{
					Data:      SPSCopy,
					Timestamp: int64(sendPTS),
				}
				PPSCopy := createCopy(cachedPPS)
				da.VideoChan <- WebRTCFrame{
					Data:      PPSCopy,
					Timestamp: int64(sendPTS),
				}

				// 检查后续 NALU (如 IDR 帧)
				for i := 3; i < len(SPS_PPS_Frame); i++ {
					nal := SPS_PPS_Frame[i]
					if len(nal) == 0 {
						continue
					}
					// 检查 NAL Type
					if (nal[0] & 0x1F) == 5 {
						// log.Println("Found IDR in SPS Packet, Sending...")
						// 拼装 StartCode + NAL
						totalLen := 4 + len(nal)
						dst := da.VideoPayloadPool.Get().([]byte)
						if cap(dst) < totalLen {
							da.VideoPayloadPool.Put(dst)
							dst = make([]byte, totalLen)
						}
						dst = dst[:totalLen]
						copy(dst, startCode)
						copy(dst[4:], nal)

						da.VideoChan <- WebRTCFrame{
							Data:      dst,
							Timestamp: int64(sendPTS),
						}
					}
				}

				da.VideoPayloadPool.Put(payloadBuf)
				continue
			}
			if frame.Header.IsKeyFrame {
				SPSCopy := createCopy(cachedSPS)
				PPSCopy := createCopy(cachedPPS)
				// log.Println("is KeyFrame, send cached SPS PPS")
				da.VideoChan <- WebRTCFrame{
					Data:      SPSCopy,
					Timestamp: int64(frame.Header.PTS),
				}
				da.VideoChan <- WebRTCFrame{
					Data:      PPSCopy,
					Timestamp: int64(frame.Header.PTS),
				}

				// log.Println("keyframe's nilType:", nilType)
			}
			// lastPTS = frame.Header.PTS

			// 这里的 Data 引用了 pool 中的内存，消费者用完必须 Put 回去
			webRTCFrame := WebRTCFrame{
				Data:      frameData,
				Timestamp: int64(frame.Header.PTS),
			}
			da.VideoChan <- webRTCFrame
			// lastNilType = nilType
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
			payloadBuf := da.AudioPayloadPool.Get().([]byte)
			if cap(payloadBuf) < int(frame.Header.Size) {
				da.AudioPayloadPool.Put(payloadBuf)
				payloadBuf = make([]byte, frame.Header.Size+1024)
				log.Println("Resized payload buffer for audio frame")
				log.Println("size:  ", frame.Header.Size)
			}
			// read frame payload
			n, _ := io.ReadFull(da.audioConn, payloadBuf[:frame.Header.Size])
			frameData := payloadBuf[:frame.Header.Size]

			if frame.Header.IsConfig {
				log.Println("Audio Config Frame Received")

				// 读取并丢弃配置帧的负载
				buf := new(bytes.Buffer)
				buf.WriteString("AOPUSHD")                        // Magic
				binary.Write(buf, binary.LittleEndian, uint64(n)) // Length
				buf.Write(frameData)
				da.AudioChan <- WebRTCFrame{
					Data:      buf.Bytes(),
					Timestamp: int64(frame.Header.PTS),
				}
				da.AudioPayloadPool.Put(payloadBuf)
				continue
			}

			// 这里的 Data 引用了 pool 中的内存，消费者用完必须 Put 回去
			webRTCFrame := WebRTCFrame{
				Data:      frameData,
				Timestamp: int64(frame.Header.PTS),
			}
			da.AudioChan <- webRTCFrame
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
