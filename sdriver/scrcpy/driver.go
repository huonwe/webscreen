package scrcpy

import (
	"bytes"
	"context"
	"embed"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
	"webscreen/sdriver"
	"webscreen/sdriver/comm"
)

//go:embed bin/scrcpy-server-master
var scrcpyServerData embed.FS

const (
	SCRCPY_SERVER_LOCAL_PATH  = "./scrcpy-server"
	SCRCPY_SERVER_ANDROID_DST = "/data/local/tmp/scrcpy-server"
	SCRCPY_PROXY_PORT_DEFAULT = "27183"
	SCRCPY_VERSION            = "3.3.4"
)

type ScrcpyDriver struct {
	VideoChan   chan sdriver.AVBox
	AudioChan   chan sdriver.AVBox
	ControlChan chan sdriver.Event

	// LinearBuffer 管理器
	videoBuffer *comm.LinearBuffer
	audioBuffer *comm.LinearBuffer

	mediaMeta  sdriver.MediaMeta
	deviceName string

	videoConn   net.Conn
	audioConn   net.Conn
	controlConn net.Conn

	options map[string]string

	capabilities sdriver.DriverCaps

	ctx       context.Context
	cancel    context.CancelFunc
	adbClient *ADBClient
	scid      string
	// socketName string

	cacheMutex         sync.RWMutex
	LastVPS            []byte
	LastSPS            []byte
	LastPPS            []byte
	LastIDR            []byte
	LastPTS            time.Duration
	LastIDRRequestTime time.Time
}

// 一个ScrcpyDriver对应一个scrcpy实例，通过本地端口建立三个连接：视频、音频、控制
func New(config map[string]string, deviceID string) (*ScrcpyDriver, error) {
	var err error
	da := &ScrcpyDriver{
		VideoChan:   make(chan sdriver.AVBox, 10),
		AudioChan:   make(chan sdriver.AVBox, 10),
		ControlChan: make(chan sdriver.Event, 10),

		videoBuffer: comm.NewLinearBuffer(0),
		audioBuffer: comm.NewLinearBuffer(1 * 1024 * 1024), // 1MB 音频缓冲区

		LastVPS: make([]byte, 32),
		LastSPS: make([]byte, 64),
		LastPPS: make([]byte, 8),
		LastIDR: make([]byte, 1*1024*1024), // 1MB IDR 缓存

		// scid: GenerateSCID(),
		scid: "00000000",

		capabilities: sdriver.DriverCaps{
			IsAndroid: true,
		},
	}
	da.ctx, da.cancel = context.WithCancel(context.Background())
	da.adbClient = NewADBClient(deviceID, da.scid, da.ctx)

	data, err := scrcpyServerData.ReadFile("bin/scrcpy-server-master")
	if err != nil {
		log.Printf("[scrcpy] read scrcpy-server failed: %v", err)
		return nil, err
	}
	err = os.WriteFile(SCRCPY_SERVER_LOCAL_PATH, data, 0755)
	if err != nil {
		log.Printf("[scrcpy] write scrcpy-server to local file failed: %v", err)
		return nil, err
	}
	err = da.adbClient.PushScrcpyServer(SCRCPY_SERVER_LOCAL_PATH, SCRCPY_SERVER_ANDROID_DST)
	if err != nil {
		log.Printf("[scrcpy] Push scrcpy-server failed: %v", err)
		return nil, err
	}

	localPort := SCRCPY_PROXY_PORT_DEFAULT

	da.adbClient.ReverseRemove(fmt.Sprintf("localabstract:scrcpy_%s", da.scid))
	err = da.adbClient.Reverse(fmt.Sprintf("localabstract:scrcpy_%s", da.scid), "tcp:"+localPort)
	if err != nil {
		log.Printf("[scrcpy] Set up reverse tunnel failed: %v", err)
		// listener.Close()
		return nil, err
	}
	log.Printf("[scrcpy] set up reverse tunnel success: localabstract:scrcpy_%s -> tcp:%s", da.scid, localPort)

	if !da.adbClient.SupportOpusAudio(SCRCPY_VERSION, da.scid) {
		config["audio"] = "false"
		log.Println("[scrcpy] Device does not support Opus audio encoding, disabling audio.")
		da.ControlChan <- sdriver.TextMsgEvent{Msg: "[scrcpy] Device does not support Opus audio encoding, disabling audio."}
	} else {
		da.ControlChan <- sdriver.TextMsgEvent{Msg: "[scrcpy] Device supports Opus audio encoding, enabling audio if configured."}
		log.Println("[scrcpy] Device supports Opus audio encoding, enabling audio if configured.")
	}
	da.adbClient.PushScrcpyServer(SCRCPY_SERVER_LOCAL_PATH, SCRCPY_SERVER_ANDROID_DST)
	os.Remove(SCRCPY_SERVER_LOCAL_PATH)
	listener, err := net.Listen("tcp", ":"+localPort)
	if err != nil {
		log.Printf("[scrcpy] Listen port failed: %v", err)
		return nil, err
	}
	// da.adbClient.cancel()
	log.Printf("[scrcpy] driver config: %v", config)
	options := map[string]string{
		"CLASSPATH":      SCRCPY_SERVER_ANDROID_DST,
		"Version":        SCRCPY_VERSION,
		"scid":           da.scid,
		"max_size":       config["max_size"],
		"max_fps":        config["max_fps"],
		"video":          "true",
		"video_bit_rate": config["video_bit_rate"],
		"video_codec":    config["video_codec"],
		"audio":          config["audio"],
		"control":        "true",
		"new_display":    config["new_display"],
		"cleanup":        "true",
		"log_level":      "info",
		// "video_encoder":  "c2.rk.hevc.encoder",
	}

	da.adbClient.StartScrcpyServer(options)
	da.options = options
	// log.Println("Scrcpy server started successfully")
	// conns := make([]net.Conn, 3)
	log.Println("start tcp listening")

	// 设置一个总的超时时间，如果在这个时间内没有建立所有连接，就认为失败
	// scrcpy-server 启动失败通常会很快退出，或者根本连不上
	timeout := time.Second * 5
	listener.(*net.TCPListener).SetDeadline(time.Now().Add(timeout))

	if options["video"] == "true" {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[scrcpy] Accept failed (可能是 scrcpy-server 启动失败): %v", err)
			listener.Close()
			da.adbClient.ReverseRemove(fmt.Sprintf("localabstract:scrcpy_%s", da.scid))
			return nil, fmt.Errorf("failed to accept connection from scrcpy-server: %v", err)
		}
		err = da.readDeviceMeta(conn)
		if err != nil {
			log.Println("Failed to read device metadata:", err)
			return nil, err
		}
		log.Printf("[scrcpy] Connected Device: %s", da.deviceName)

		da.assignConn(conn)
	}
	if options["audio"] == "true" {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[scrcpy] Accept failed (可能是 scrcpy-server 启动失败): %v", err)
			listener.Close()
			da.adbClient.ReverseRemove(fmt.Sprintf("localabstract:scrcpy_%s", da.scid))
			return nil, fmt.Errorf("failed to accept connection from scrcpy-server: %v", err)
		}
		da.assignConn(conn)
	}
	if options["control"] == "true" {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[scrcpy] Accept failed (可能是 scrcpy-server 启动失败): %v", err)
			listener.Close()
			da.adbClient.ReverseRemove(fmt.Sprintf("localabstract:scrcpy_%s", da.scid))
			return nil, fmt.Errorf("failed to accept connection from scrcpy-server: %v", err)
		}
		da.controlConn = conn
		da.capabilities.CanControl = true
		da.capabilities.CanUHID = true
		da.capabilities.CanClipboard = true
		log.Println("Scrcpy Control Connection Established")
	}

	listener.Close()

	// 甜点值
	if da.videoConn != nil {
		da.videoConn.(*net.TCPConn).SetReadBuffer(4 * 1024 * 1024)
	}
	if da.audioConn != nil {
		da.audioConn.(*net.TCPConn).SetReadBuffer(64 * 1024)
	}

	// 设更合理的读缓冲区大小
	// da.videoConn.(*net.TCPConn).SetReadBuffer(2 * 1024 * 1024)
	// da.audioConn.(*net.TCPConn).SetReadBuffer(64 * 1024)

	return da, nil
}

func (da *ScrcpyDriver) ShowDeviceInfo() {
	log.Printf("[scrcpy] Device Name: %s", da.deviceName)
	log.Printf("[scrcpy] media Meta: %v", da.mediaMeta)
}

// Please Ensure the input conn is not Control conn
func (da *ScrcpyDriver) assignConn(conn net.Conn) error {
	codecID := readCodecID(conn)
	switch codecID {
	case "h264", "h265", "av1 ":
		da.videoConn = conn
		da.mediaMeta.VideoCodec = codecID
		err := da.readVideoMeta(conn)
		if err != nil {
			log.Fatalln("Failed to read video metadata:", err)
			return err
		}
		da.capabilities.CanVideo = true
		log.Println("Scrcpy Video Connection Established")
	case "aac ", "opus":
		da.audioConn = conn
		da.mediaMeta.AudioCodec = codecID
		da.capabilities.CanAudio = true
		log.Println("Audio Connection Established")
		// default:
		// 	da.controlConn = conn
		// 	log.Println("Scrcpy Control Connection Established")
	}
	return nil
}

func (da *ScrcpyDriver) readDeviceMeta(conn net.Conn) error {
	// 1. Device Name (64 bytes)
	nameBuf := make([]byte, 64)
	_, err := io.ReadFull(conn, nameBuf)
	if err != nil {
		return err
	}
	da.deviceName = string(nameBuf)
	return nil
}

func (da *ScrcpyDriver) readVideoMeta(conn net.Conn) error {
	// Width (4 bytes)
	// Height (4 bytes)
	// Codec 已经在外面读取过了，用于确认是哪个通道
	metaBuf := make([]byte, 8)
	if _, err := io.ReadFull(conn, metaBuf); err != nil {
		log.Println("Failed to read metadata:", err)
		return err
	}
	// 解析元数据
	da.mediaMeta.Width = binary.BigEndian.Uint32(metaBuf[0:4])
	da.mediaMeta.Height = binary.BigEndian.Uint32(metaBuf[4:8])

	return nil
}

func (da *ScrcpyDriver) updateVideoMetaFromSPS(sps []byte, codec string) {
	if da.LastSPS != nil && bytes.Equal(da.LastSPS, sps) {
		// log.Println("SPS unchanged, no need to update video meta")
		return
	}
	var spsInfo comm.SPSInfo
	var err error
	switch codec {
	case "h264":
		spsInfo, err = comm.ParseSPS_H264(sps, true)
	case "h265":
		spsInfo, err = comm.ParseSPS_H265(sps)
	default:
		log.Println("Unknown codec type for SPS parsing:", codec)
		return
	}

	if err != nil {
		log.Println("Failed to parse SPS for video meta update:", err)
		return
	}
	da.mediaMeta.Width = spsInfo.Width
	da.mediaMeta.Height = spsInfo.Height
	log.Printf("[scrcpy] Updated Video Meta from SPS: Width=%d, Height=%d", da.mediaMeta.Width, da.mediaMeta.Height)
}

// func (da *ScrcpyDriver) cacheFrame(webrtcFrame *WebRTCFrame, frameType string) {
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
	log.Printf("[scrcpy] Frame Header - PTS: %d, Size: %d, IsConfig: %v, IsKeyFrame: %v",
		header.PTS, header.Size, header.IsConfig, header.IsKeyFrame)
}
