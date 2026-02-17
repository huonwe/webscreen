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
	"strconv"
	"strings"
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
func New(config map[string]string) (*ScrcpyDriver, error) {
	var err error
	da := &ScrcpyDriver{
		VideoChan:   make(chan sdriver.AVBox, 10),
		AudioChan:   make(chan sdriver.AVBox, 10),
		ControlChan: make(chan sdriver.Event, 10),

		videoBuffer: comm.NewLinearBuffer(0),
		audioBuffer: comm.NewLinearBuffer(4 * 1024 * 1024), // 4MB 音频缓冲区

		// scid: GenerateSCID(),
		scid: "00000000",

		capabilities: sdriver.DriverCaps{
			IsAndroid: true,
		},
	}
	da.ctx, da.cancel = context.WithCancel(context.Background())
	da.adbClient = NewADBClient(config["deviceID"], da.scid, da.ctx)

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
	video_codec_options := ""
	max_size, err := strconv.Atoi(config["max_size"])
	if err != nil {
		max_size = 3840
	}
	max_fps, err := strconv.Atoi(config["max_fps"])
	if err != nil {
		max_fps = 120
	}
	video_bit_rate, err := strconv.Atoi(config["video_bit_rate"])
	if err != nil {
		video_bit_rate = 8000000
	}
	codecConfigStr := config["webrtc_codec"]
	if codecConfigStr != "" {
		parts := strings.Split(codecConfigStr, "||")
		mimeType := parts[1]
		sdpFmtpLine := parts[2]
		if strings.EqualFold(mimeType, "video/AV1") {
			log.Println("AV1 Main 5.1")
			// --- AV1 ---
			// PAYLOAD_TYPE_AV1_PROFILE_MAIN_5_1
			video_codec_options += "profile=1"
			kv := strings.Split(sdpFmtpLine, ";")
			for _, item := range kv {
				item = strings.TrimSpace(item)
				if strings.HasPrefix(item, "level-idx=") {
					levelStr := strings.TrimPrefix(item, "level-idx=")
					levelID64, err := strconv.ParseUint(levelStr, 10, 32)
					if err != nil {
						log.Printf("Failed to parse level-idx: %v", err)
						return nil, err
					}
					levelID := uint(levelID64)
					switch levelID {
					case 8:
						log.Println("AV1 Level 4.0")
						max_size = min(max_size, 1920)
						video_bit_rate = min(video_bit_rate, 12_000_000)
					case 9:
						log.Println("AV1 Level 4.1")
						max_size = min(max_size, 1920)
						max_fps = min(max_fps, 60)
						video_bit_rate = min(video_bit_rate, 20_000_000)
					case 12:
						log.Println("AV1 Level 5.0")
						video_bit_rate = min(video_bit_rate, 30_000_000)
					case 13:
						log.Println("AV1 Level 5.1")
						video_bit_rate = min(video_bit_rate, 40_000_000)
					case 14:
						log.Println("AV1 Level 5.2")
						video_bit_rate = min(video_bit_rate, 60_000_000)
					case 15:
						log.Println("AV1 Level 5.3")
						video_bit_rate = min(video_bit_rate, 80_000_000)
					default:
						log.Println("AV1 Unexpected Level ID:", levelID)
						if levelID < 8 {
							video_bit_rate = min(video_bit_rate, 10_000_000)
						}
						if levelID > 15 {
							video_bit_rate = min(video_bit_rate, 80_000_000)
						}
					}
				}
			}
		} else if strings.EqualFold(mimeType, "video/H265") || strings.EqualFold(mimeType, "video/HEVC") {
			// --- H.265 (HEVC) ---
			// Main Profile
			video_codec_options += "profile=1"
			// FMTP example: "profile-id=1;tier-flag=0;level-id=123"
			// level-id=123 (Level 4.1), level-id=153 (Level 5.1)
			var levelID uint
			kv := strings.Split(sdpFmtpLine, ";")
			for _, item := range kv {
				item = strings.TrimSpace(item)
				if strings.HasPrefix(item, "level-id=") {
					levelStr := strings.TrimPrefix(item, "level-id=")
					levelID64, err := strconv.ParseUint(levelStr, 10, 32)
					if err != nil {
						log.Printf("Failed to parse level-id: %v", err)
						return nil, err
					}
					levelID = uint(levelID64)
					break
				}
			}
			switch levelID {
			case 123:
				log.Println("H.265 Level 4.1")
				max_size = min(max_size, 1920)
				max_fps = min(max_fps, 60)
				video_bit_rate = min(video_bit_rate, 20_000_000)
			case 150:
				log.Println("H.265 Level 5.0")
				video_bit_rate = min(video_bit_rate, 25_000_000)
			case 153:
				log.Println("H.265 Level 5.1")
				video_bit_rate = min(video_bit_rate, 40_000_000)
			case 156:
				log.Println("H.265 Level 5.2")
				video_bit_rate = min(video_bit_rate, 50_000_000)
			case 180:
				log.Println("H.265 Level 6.0")
				video_bit_rate = min(video_bit_rate, 60_000_000)
			default:
				log.Println("H.265 Unexpected Level ID:", levelID)
				if levelID < 123 {
					video_bit_rate = min(video_bit_rate, 10_000_000)
				}
				if levelID > 180 {
					video_bit_rate = min(video_bit_rate, 60_000_000)
				}
			}
		} else if strings.EqualFold(mimeType, "video/H264") {
			// --- H.264 (AVC) ---
			// profile-level-id : Baseline (42), Main (4d), High (64)
			// level-asymmetry-allowed=1 is supposed to be always set
			video_bit_rate = min(video_bit_rate, 300_000_000)
			if strings.Contains(sdpFmtpLine, "profile-level-id=42") {
				log.Println("H.264 Baseline Profile")
				video_codec_options += "profile=1"
			} else if strings.Contains(sdpFmtpLine, "profile-level-id=4d") {
				log.Println("H.264 Main Profile")
				video_codec_options += "profile=2"
			} else if strings.Contains(sdpFmtpLine, "profile-level-id=64") {
				log.Println("H.264 High Profile")
				video_codec_options += "profile=8"
			}
		}

	}
	options := map[string]string{
		"CLASSPATH":           SCRCPY_SERVER_ANDROID_DST,
		"Version":             SCRCPY_VERSION,
		"scid":                da.scid,
		"max_size":            strconv.Itoa(max_size),
		"max_fps":             strconv.Itoa(max_fps),
		"video":               "true",
		"video_codec_options": video_codec_options, // bitrate-mode=2 to enable CBR
		"video_bit_rate":      strconv.Itoa(video_bit_rate),
		"video_codec":         config["video_codec"],
		"audio":               config["audio"],
		"audio_bit_rate":      config["audio_bit_rate"],
		// "audio_codec_options": "durationUs=10000", // 10ms
		"control":     "true",
		"new_display": config["new_display"],
		"cleanup":     "true",
		"log_level":   "info",

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
	// log.Printf("last SPS len=%d, new SPS len=%d", len(da.LastSPS), len(sps))
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
