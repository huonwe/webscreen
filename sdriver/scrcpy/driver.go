package scrcpy

import (
	"bytes"
	"context"
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
	"webscreen/utils"
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
	LastPTS            uint64
	LastIDRRequestTime time.Time
}

func setDefault(config map[string]string, key string, value string) {
	if strings.TrimSpace(config[key]) == "" {
		config[key] = value
	}
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

	data, err := scrcpyServerData.ReadFile(SCRCPY_EMBED_PATH)
	if err != nil {
		log.Printf("[scrcpy] read scrcpy-server failed: %v", err)
		return nil, err
	}
	SCRCPY_SERVER_LOCAL_PATH := os.TempDir() + "/scrcpy-server"
	err = os.WriteFile(SCRCPY_SERVER_LOCAL_PATH, data, 0755)
	if err != nil {
		log.Printf("[scrcpy] write scrcpy-server to local file failed: %v", err)
		return nil, err
	}

	localPort := SCRCPY_PROXY_PORT_DEFAULT
	remoteADBDirect := da.adbClient.transportErr == nil &&
		da.adbClient.transport != nil &&
		da.adbClient.transport.Capabilities().SupportsDirectAbs

	if remoteADBDirect {
		log.Printf("[scrcpy] using remote adb direct socket mode for localabstract:scrcpy_%s", da.scid)
	} else {
		da.adbClient.ReverseRemove(fmt.Sprintf("localabstract:scrcpy_%s", da.scid))
		err = da.adbClient.Reverse(fmt.Sprintf("localabstract:scrcpy_%s", da.scid), "tcp:"+localPort)
		if err != nil {
			log.Printf("[scrcpy] Set up reverse tunnel failed: %v", err)
			// listener.Close()
			return nil, err
		}
		log.Printf("[scrcpy] set up reverse tunnel success: localabstract:scrcpy_%s -> tcp:%s", da.scid, localPort)
	}

	if !da.adbClient.SupportOpusAudio() {
		config["audio"] = "false"
		log.Println("[scrcpy] Device does not support Opus audio encoding, disabling audio.")
		da.ControlChan <- sdriver.TextMsgEvent{Msg: "[scrcpy] Device does not support Opus audio encoding, disabling audio."}
	}
	err = da.adbClient.PushScrcpyServer(SCRCPY_SERVER_LOCAL_PATH, SCRCPY_SERVER_ANDROID_DST)
	if err != nil {
		log.Printf("[scrcpy] Push scrcpy-server failed: %v", err)
		return nil, err
	}
	os.Remove(SCRCPY_SERVER_LOCAL_PATH)
	var listener net.Listener
	if !remoteADBDirect {
		listener, err = net.Listen("tcp", ":"+localPort)
		if err != nil {
			log.Printf("[scrcpy] Listen port failed: %v", err)
			return nil, err
		}
	}
	// da.adbClient.cancel()
	log.Printf("[scrcpy] driver config: %v", config)
	setDefault(config, "audio", "false")
	setDefault(config, "control", "true")
	setDefault(config, "video_codec", "h264")
	setDefault(config, "video_bit_rate", "8M")
	setDefault(config, "max_size", "1280")
	setDefault(config, "max_fps", "60")

	video_codec_options := ""
	max_size, err := strconv.Atoi(config["max_size"])
	if err != nil {
		max_size = 3840
	}
	max_fps, err := strconv.Atoi(config["max_fps"])
	if err != nil {
		max_fps = 120
	}
	video_bit_rate_str := config["video_bit_rate"]
	video_bit_rate, err := utils.ParseBitrate(video_bit_rate_str)
	if err != nil {
		return nil, fmt.Errorf("invalid video bit rate: %v", err)
	}
	codecConfigStr := config["webrtc_codec_level"]
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

	video_codec_options_user := config["video_codec_options"]
	if video_codec_options_user != "" {
		video_codec_options = video_codec_options_user
		log.Printf("Using user-provided video codec options: %s", video_codec_options)
	}

	options := map[string]string{
		"CLASSPATH":           SCRCPY_SERVER_ANDROID_DST,
		"Version":             SCRCPY_VERSION,
		"scid":                da.scid,
		"max_size":            strconv.Itoa(max_size),
		"max_fps":             strconv.Itoa(max_fps),
		"video":               "true",
		"video_bit_rate":      strconv.Itoa(video_bit_rate),
		"video_codec":         config["video_codec"],
		"video_codec_options": video_codec_options, // bitrate-mode=2 to enable CBR
		"audio":               config["audio"],
		// "audio_bit_rate":      config["audio_bit_rate"],
		// "audio_codec_options": "durationUs=10000", // 10ms
		"control":   config["control"],
		"cleanup":   "true",
		"log_level": "info",

		"video_encoder": config["video_encoder"],
	}
	if remoteADBDirect {
		options["tunnel_forward"] = "true"
	}
	if config["video_encoder"] != "" {
		options["video_encoder"] = config["video_encoder"]
		log.Printf("Using user-specified video encoder: %s", config["video_encoder"])
	}
	if config["no_video_codec_options"] == "true" {
		delete(options, "video_codec_options")
		log.Println("User requested to disable video codec options, ignoring all codec options.")
	}
	if config["new_display"] == "true" {
		options["new_display"] = fmt.Sprintf("%s/%d", config["resolution"], max_fps)
		// options["start_app"] = config["start_app"]
		log.Printf("Using new virtual display with resolution %s and max_fps %d", config["resolution"], max_fps)
		// log.Printf("start_app: %s", config["start_app"])
	}

	da.adbClient.StartScrcpyServer(options)
	da.options = options
	// log.Println("Scrcpy server started successfully")
	// conns := make([]net.Conn, 3)

	if remoteADBDirect {
		if err := da.connectRemoteADBSockets(options); err != nil {
			return nil, err
		}
		return da, nil
	}

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

func (da *ScrcpyDriver) connectRemoteADBSockets(options map[string]string) error {
	socketName := fmt.Sprintf("scrcpy_%s", da.scid)
	connect := func(label string) (net.Conn, error) {
		conn, err := da.adbClient.ConnectLocalAbstractWithRetry(socketName, 12*time.Second)
		if err != nil {
			return nil, fmt.Errorf("failed to connect remote adb %s socket: %v", label, err)
		}
		return conn, nil
	}

	var firstConn net.Conn
	var videoConn net.Conn
	var audioConn net.Conn
	var controlConn net.Conn

	if options["video"] == "true" {
		conn, err := connect("video")
		if err != nil {
			return err
		}
		videoConn = conn
		if firstConn == nil {
			firstConn = conn
		}
	}
	if options["audio"] == "true" {
		conn, err := connect("audio")
		if err != nil {
			return err
		}
		audioConn = conn
		if firstConn == nil {
			firstConn = conn
		}
	}
	if options["control"] == "true" {
		conn, err := connect("control")
		if err != nil {
			return err
		}
		controlConn = conn
		if firstConn == nil {
			firstConn = conn
		}
	}

	if firstConn == nil {
		return fmt.Errorf("no scrcpy sockets enabled")
	}

	dummy := make([]byte, 1)
	if _, err := io.ReadFull(firstConn, dummy); err != nil {
		return fmt.Errorf("failed to read scrcpy forward dummy byte: %v", err)
	}
	if err := da.readDeviceMeta(firstConn); err != nil {
		return fmt.Errorf("failed to read device metadata: %v", err)
	}
	log.Printf("[scrcpy] Connected Device: %s", da.deviceName)

	if videoConn != nil {
		if err := da.assignConn(videoConn); err != nil {
			videoConn.Close()
			return err
		}
	}
	if audioConn != nil {
		if err := da.assignConn(audioConn); err != nil {
			audioConn.Close()
			return err
		}
	}
	if controlConn != nil {
		da.controlConn = controlConn
		da.capabilities.CanControl = true
		da.capabilities.CanUHID = true
		da.capabilities.CanClipboard = true
		log.Println("Scrcpy Control Connection Established")
	}
	return nil
}

func (da *ScrcpyDriver) EncoderList() []string {
	return da.adbClient.SupportedVideoEncoderList()
}

// Please Ensure the input conn is not Control conn
func (da *ScrcpyDriver) assignConn(conn net.Conn) error {
	codecID := readCodecID(conn)
	switch codecID {
	case "h264", "h265", "av1":
		da.videoConn = conn
		da.mediaMeta.VideoCodec = codecID
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
	log.Println("read nameBuf from scrcpy: ", string(nameBuf))
	da.deviceName = string(nameBuf)
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
	da.updateSize(spsInfo.Width, spsInfo.Height)
}

func (da *ScrcpyDriver) updateSize(width, height uint32) {
	da.mediaMeta.Width = width
	da.mediaMeta.Height = height
	log.Printf("[scrcpy] Updated Video Meta from session packet: Width=%d, Height=%d", da.mediaMeta.Width, da.mediaMeta.Height)
}

func readScrcpyFrameHeader(headerBuf []byte, header *ScrcpyFrameHeader) error {

	ptsAndFlags := binary.BigEndian.Uint64(headerBuf[0:8])
	packetSize := binary.BigEndian.Uint32(headerBuf[8:12])

	// 提取标志位
	isConfig := (headerBuf[0] & 0x40) != 0
	isKeyFrame := (headerBuf[0] & 0x20) != 0

	// 提取PTS (低61位)
	pts := ptsAndFlags & 0x1FFFFFFFFFFFFFFF
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

func createCopy(src []byte, context string) []byte {
	if len(src) == 0 {
		log.Printf("createCopy called with src length: %d, context: %s", len(src), context)
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
