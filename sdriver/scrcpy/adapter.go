package scrcpy

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"sync"
	"time"
	"webcpy/adb"
)

// defaultScrcpyOptions := ScrcpyOptions{
// 	Version:      "3.3.3",
// 	SCID:         GenerateSCID(),
// 	MaxFPS:       "60",
// 	VideoBitRate: "20000000",
// 	Control:      "true",
// 	Audio:        "true",
// 	VideoCodec:   "h264",
// 	NewDisplay:   "",
// 	// VideoCodecOptions: "i-frame-interval=1",
// 	LogLevel: "info",
// }

type DataAdapter struct {
	VideoChan   chan WebRTCFrame
	AudioChan   chan WebRTCFrame
	ControlChan chan WebRTCControlFrame

	// LinearBuffer 管理器
	videoBuffer *LinearBuffer
	audioBuffer *LinearBuffer

	DeviceName string
	VideoMeta  ScrcpyVideoMeta
	AudioMeta  ScrcpyAudioMeta

	videoConn   net.Conn
	audioConn   net.Conn
	controlConn net.Conn

	adbClient *adb.ADBClient

	lastIDRRequestTime time.Time

	keyFrameMutex sync.RWMutex // 保护 LastSPS, LastPPS, LastIDR
	LastVPS       []byte       // 新增：H.265 VPS
	LastSPS       []byte
	LastPPS       []byte
	LastIDR       []byte
	LastIDRTime   time.Time
}

// TODO: Adapter和Stream Manager 解耦
// 一个DataAdapter对应一个scrcpy实例，通过本地端口建立三个连接：视频、音频、控制
func NewDataAdapter(config map[string]string) (*DataAdapter, error) {
	var err error
	// serial := config["device_serial"]
	da := &DataAdapter{
		// adbClient:   adb.NewADBClient(serial),
		VideoChan:   make(chan WebRTCFrame, 10),
		AudioChan:   make(chan WebRTCFrame, 10),
		ControlChan: make(chan WebRTCControlFrame, 10),

		// 4MB 足够存放几秒的高清视频数据
		// 当这 4MB 用完后，我们会分配新的，旧的由 GC 自动回收
		videoBuffer: NewLinearBuffer(0),
		audioBuffer: NewLinearBuffer(1 * 1024 * 1024), // 1MB 音频缓冲区
	}
	err = da.adbClient.Push(config["server_local_path"], config["server_remote_path"])
	if err != nil {
		log.Printf("设置 推送scrcpy-server失败: %v", err)
		return nil, err
	}
	err = da.adbClient.Reverse("localabstract:scrcpy", "tcp:"+config["local_port"])
	if err != nil {
		log.Printf("设置 Reverse 隧道失败: %v", err)
		return nil, err
	}
	// TODO: 使用 ScrcpyOptions 配置参数
	da.adbClient.StartScrcpyServer(config)
	listener, err := net.Listen("tcp", ":"+config["local_port"])
	if err != nil {
		log.Printf("监听端口失败: %v", err)
		return nil, err
	}
	defer listener.Close()
	conns := make([]net.Conn, 3)
	for i := 0; i < 3; i++ {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept 失败: %v", err)
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
				log.Println("Failed to read device metadata:", err)
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
			go da.startControlReader()
		}
	}
	// 甜点值
	da.videoConn.(*net.TCPConn).SetReadBuffer(2 * 1024 * 1024)
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
	go da.convertVideoFrame()
}

func (da *DataAdapter) StartConvertAudioFrame() {
	go da.convertAudioFrame()
}

func (da *DataAdapter) startControlReader() {
	go da.transferControlMsg()
}

func (da *DataAdapter) assignConn(conn net.Conn) error {
	codecID := readCodecID(conn)
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

func (da *DataAdapter) updateVideoMetaFromSPS(sps []byte, codec string) {
	if da.LastSPS != nil && bytes.Equal(da.LastSPS, sps) {
		// log.Println("SPS unchanged, no need to update video meta")
		return
	}
	var spsInfo SPSInfo
	var err error
	switch codec {
	case "h264":
		spsInfo, err = ParseSPS_H264(sps, true)
	case "h265":
		spsInfo, err = ParseSPS_H265(sps)
	default:
		log.Println("Unknown codec type for SPS parsing:", codec)
		return
	}

	if err != nil {
		log.Println("Failed to parse SPS for video meta update:", err)
		return
	}
	da.VideoMeta.Width = spsInfo.Width
	da.VideoMeta.Height = spsInfo.Height
	log.Printf("Updated Video Meta from SPS: Width=%d, Height=%d", da.VideoMeta.Width, da.VideoMeta.Height)
}

// func (da *DataAdapter) cacheFrame(webrtcFrame *WebRTCFrame, frameType string) {
// 	switch frameType {
// 	case "SPS":
// 		da.keyFrameMutex.Lock()

// }

func readScrcpyFrameHeader(headerBuf []byte, header *ScrcpyFrameHeader) error {

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

func readCodecID(conn net.Conn) string {
	// Codec ID (4 bytes)
	codecBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, codecBuf); err != nil {
		log.Println("Failed to read codec ID:", err)
		return ""
	}

	return string(codecBuf)
}

func createCopy(src []byte) []byte {
	if len(src) == 0 {
		log.Println("createCopy called with empty src")
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

func ShowFrameHeaderInfo(header ScrcpyFrameHeader) {
	log.Printf("Frame Header - PTS: %d, Size: %d, IsConfig: %v, IsKeyFrame: %v",
		header.PTS, header.Size, header.IsConfig, header.IsKeyFrame)
}
