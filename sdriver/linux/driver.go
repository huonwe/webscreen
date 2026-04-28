package linuxDriver

import (
	"embed"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"
	"webscreen/sdriver"
	"webscreen/sdriver/comm"
)

//go:embed bin/capturer_wf-recorder
//go:embed bin/capturer_xorg
//go:embed bin/capturer_xvfb
var capturerXvfbData embed.FS

// sudo killall Xvfb
type LinuxDriver struct {
	videoChan   chan sdriver.AVBox
	videoBuffer *comm.LinearBuffer
	conn        net.Conn

	ip          string
	user        string
	password    string
	resolution  string
	frameRate   string
	bitRate     string
	video_codec string

	lastSPS []byte
	lastPPS []byte
	lastIDR []byte
}

// 简单的 Header 定义，对应发送端的结构
type Header struct {
	PTS  uint64
	Size uint32
}

func New(cfg map[string]string) (*LinuxDriver, error) {
	d := &LinuxDriver{
		videoChan:   make(chan sdriver.AVBox, 10), // 适当增大缓冲防止阻塞
		ip:          cfg["ip"],
		user:        cfg["user"],
		resolution:  cfg["resolution"],
		frameRate:   cfg["frameRate"],
		bitRate:     cfg["bitRate"],
		video_codec: cfg["video_codec"],

		videoBuffer: comm.NewLinearBuffer(16 * 1024 * 1024),
	}

	data, err := capturerXvfbData.ReadFile("bin/capturer_xorg")
	if err != nil {
		log.Printf("[wf-recorder] 读取 capturer_wf-recorder 失败: %v", err)
		return nil, err
	}
	err = os.WriteFile("capturer_xvfb", data, 0755)
	if err != nil {
		log.Printf("[xvfb] 写入本地文件失败: %v", err)
		os.Remove("capturer_xvfb")
		return nil, err
	}
	if d.ip == "127.0.0.1" || d.ip == "localhost" || d.ip == "" {
		d.ip = "127.0.0.1"
		err = LocalStartXvfb("27184", d.resolution, d.bitRate, d.frameRate, d.video_codec)
	} else {
		err = PushAndStartXvfb(d.user, d.ip, "27184", d.resolution, d.bitRate, d.frameRate, d.video_codec)
	}
	if err != nil {
		log.Printf("[xvfb] 启动远程 capturer_xvfb 失败: %v", err)
		os.Remove("capturer_xvfb")
		return nil, err
	}

	var conn net.Conn
	startTime := time.Now()
	for {
		conn, err = net.Dial("tcp", d.ip+":27184")
		if err == nil {
			break
		}
		time.Sleep(time.Second)
		if time.Since(startTime) > 5*time.Second {
			os.Remove("capturer_xvfb")
			return nil, fmt.Errorf("Failed to connect to capturer after 5 seconds: %v", err)
		}
	}
	d.conn = conn
	return d, nil
}

func (d *LinuxDriver) Start() {
	// 启动视频监听
	go d.handleConnection()
	log.Println("LinuxDriver started, listening for connections...")
}

func (d *LinuxDriver) UpdateDriverConfig(config map[string]string) error {
	return nil
}

// Start, GetReceivers 等方法保持不变...
// 仅重写 handleConnection

func (d *LinuxDriver) handleConnection() {
	headerBuf := make([]byte, 12)
	waitForKeyFrame := true
	for {
		// 1. 读取固定长度的 Header (12 bytes)
		if _, err := io.ReadFull(d.conn, headerBuf); err != nil {
			log.Println("Failed to read header:", err)
			return
		}

		pts := binary.BigEndian.Uint64(headerBuf[0:8])
		size := binary.BigEndian.Uint32(headerBuf[8:12])

		// 2. 准备 payload 缓冲区
		// 确保缓冲区够大
		payloadBuf := d.videoBuffer.Get(int(size))

		// 3. 读取完整的 NALU Payload
		if _, err := io.ReadFull(d.conn, payloadBuf); err != nil {
			log.Println("Failed to read payload:", err)
			return
		}

		// 此时 payloadBuf 包含 Annex B 格式数据 (00 00 00 01 XX XX ...)
		// 目标：剥离起始码，只保留 NAL Unit Header + Data

		// 查找起始码结束的位置
		// 起始码通常是 00 00 01 或 00 00 00 01
		startCodeEnd := 0
		if len(payloadBuf) > 4 && payloadBuf[0] == 0 && payloadBuf[1] == 0 && payloadBuf[2] == 0 && payloadBuf[3] == 1 {
			startCodeEnd = 4
		} else if len(payloadBuf) > 3 && payloadBuf[0] == 0 && payloadBuf[1] == 0 && payloadBuf[2] == 1 {
			startCodeEnd = 3
		} else {
			// 异常情况：没有标准起始码，可能数据错乱，或者发送端已经是 AVCC 格式？
			// 这里假设必须有 Annex B 起始码
			log.Printf("Warning: Invalid start code in NALU of size %d", size)
			continue
		}
		// log.Println("startCodeEnd:", startCodeEnd)

		// 真正的 NAL 数据（不含起始码）
		nalData := payloadBuf[startCodeEnd:]

		if len(nalData) == 0 {
			continue
		}

		// 解析 NAL Header (第一个字节)
		nalHeader := nalData[0]
		nalType := nalHeader & 0x1F

		isKeyFrame := false
		// log.Printf("Processing NALU Type: %d", nalType)

		switch nalType {
		case 6, 9: // SEI, AUD
			// log.Printf("Received SEI, PTS=%d, Size=%d bytes", pts, len(nalData))
			log.Println("Received SEI/AUD:", string(nalData))
			continue
		case 7: // SPS
			// log.Printf("Received SPS, PTS=%d, Size=%d bytes", pts, len(nalData))
			d.lastSPS = make([]byte, len(nalData))
			copy(d.lastSPS, nalData)
			continue
		case 8: // PPS
			// log.Printf("Received PPS, PTS=%d, Size=%d bytes", pts, len(nalData))
			d.lastPPS = make([]byte, len(nalData))
			copy(d.lastPPS, nalData)
			// log.Println("PPS Data:", nalData)
			continue
		case 5: // IDR (关键帧)
			// log.Printf("Received IDR frame, PTS=%d, Size=%d bytes", pts, len(nalData))
			d.lastIDR = make([]byte, len(nalData))
			copy(d.lastIDR, nalData)
			isKeyFrame = true
			waitForKeyFrame = false // 【重点新增】成功捕获首个关键帧，解除拦截状态！
		default:
			// log.Printf("Received non-key frame (NALU Type=%d), PTS=%d, Size=%d bytes", nalType, pts, len(nalData))
		}
		if waitForKeyFrame {
			log.Printf("Dropping non-key frame (NALU Type=%d) before first IDR", nalType)
			continue
		}

		var sendData []byte
		if isKeyFrame {
			startCode := []byte{0x00, 0x00, 0x00, 0x01}
			sendData = make([]byte, 0, len(d.lastSPS)+len(d.lastPPS)+len(payloadBuf)+12)
			if len(d.lastSPS) > 0 {
				sendData = append(sendData, startCode...)
				sendData = append(sendData, d.lastSPS...)
			}
			if len(d.lastPPS) > 0 {
				sendData = append(sendData, startCode...)
				sendData = append(sendData, d.lastPPS...)
			}
			sendData = append(sendData, startCode...)
			sendData = append(sendData, nalData...)
		} else {
			// 保留 payloadBuf 原始的 Annex-B Start Code
			sendData = payloadBuf
		}

		// 4. 发送 AVBox
		d.videoChan <- sdriver.AVBox{
			Data:       sendData,
			PTS:        time.Duration(pts) * time.Microsecond,
			IsKeyFrame: isKeyFrame,
			IsConfig:   false,
		}
		// naltype := nalData[0] & 0x1F
		// log.Printf("Sent AVBox: NALU Type=%d, PTS=%d, Size=%d bytes, IsKeyFrame=%v\n", naltype, pts, len(sendData), isKeyFrame)
		// log.Println(nalData)
	}
}

// ... 其他方法保持不变

// 实现 sdriver.SDriver 接口的其他方法
func (d *LinuxDriver) GetReceivers() (<-chan sdriver.AVBox, <-chan sdriver.AVBox, chan sdriver.Event) {
	return d.videoChan, nil, nil
}

func (d *LinuxDriver) Pause() {}

func (d *LinuxDriver) RequestIDR(firstFrame bool) {
	// send cache key frame if available
	// if len(d.lastSPS) > 0 && len(d.lastPPS) > 0 {
	// 	startCode := []byte{0x00, 0x00, 0x00, 0x01}
	// 	keyFrameData := make([]byte, 0, len(d.lastSPS)+len(d.lastPPS)+8)
	// 	keyFrameData = append(keyFrameData, startCode...)
	// 	keyFrameData = append(keyFrameData, d.lastSPS...)
	// 	keyFrameData = append(keyFrameData, startCode...)
	// 	keyFrameData = append(keyFrameData, d.lastPPS...)

	// 	keyFrameData = append(keyFrameData, startCode...)
	// 	if len(d.lastIDR) > 0 {
	// 		keyFrameData = append(keyFrameData, d.lastIDR...)
	// 	} else {
	// 		log.Println("Warning: No cached IDR frame available, sending SPS/PPS only")
	// 	}

	// 	d.videoChan <- sdriver.AVBox{
	// 		Data:       keyFrameData,
	// 		PTS:        time.Now().Sub(time.Unix(0, 0)),
	// 		IsKeyFrame: true,
	// 		IsConfig:   false,
	// 	}
	// 	log.Println("Sent cached key frame (SPS+PPS) in response to IDR request")
	// } else {
	// 	log.Println("No cached SPS/PPS available to send for IDR request")
	// }
}

func (d *LinuxDriver) Capabilities() sdriver.DriverCaps {
	return sdriver.DriverCaps{
		CanAudio:     false,
		CanVideo:     true,
		CanControl:   true,
		CanClipboard: false,
		CanUHID:      false,
		IsLinux:      true,
	}
}

// CodecInfo() (videoCodec string, audioCodec string)
func (d *LinuxDriver) MediaMeta() sdriver.MediaMeta {
	return sdriver.MediaMeta{
		Width:      1920,
		Height:     1080,
		VideoCodec: "h264",
		AudioCodec: "",
	}
}

func (d *LinuxDriver) Stop() {
	if d.conn != nil {
		d.conn.Close()
	}
	os.Remove("capturer_xvfb")
}

func (d *LinuxDriver) ConfigDescription() map[string]string {
	return map[string]string{
		"test": "{String, defaultValue, isOptional, [options...]}Description of the config parameter",
	}
}
