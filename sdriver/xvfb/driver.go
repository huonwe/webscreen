package linuxXvfbDriver

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
	data, err := capturerXvfbData.ReadFile("bin/capturer_xvfb")
	if err != nil {
		log.Printf("[xvfb] 读取 capturer_xvfb 失败: %v", err)
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

		// 真正的 NAL 数据（不含起始码）
		nalData := payloadBuf[startCodeEnd:]

		if len(nalData) == 0 {
			continue
		}

		// 解析 NAL Header (第一个字节)
		nalHeader := nalData[0]
		nalType := nalHeader & 0x1F

		isKeyFrame := false
		isConfig := false

		switch nalType {
		case 7: // SPS
			isConfig = true
		case 8: // PPS
			isConfig = true
		case 5: // IDR (关键帧)
			isKeyFrame = true
		}

		// log.Printf("Recv NAL: type=%d, len=%d, isKey=%v", nalType, len(nalData), isKeyFrame)

		// 4. 发送 AVBox
		d.videoChan <- sdriver.AVBox{
			Data:       nalData, // 这里的切片引用的是 videoBuffer 的底层数组，注意生命周期
			PTS:        time.Duration(pts) * time.Microsecond,
			IsKeyFrame: isKeyFrame,
			IsConfig:   isConfig,
		}
	}
}

// ... 其他方法保持不变

// 实现 sdriver.SDriver 接口的其他方法
func (d *LinuxDriver) GetReceivers() (<-chan sdriver.AVBox, <-chan sdriver.AVBox, chan sdriver.Event) {
	return d.videoChan, nil, nil
}

func (d *LinuxDriver) Pause() {}

func (d *LinuxDriver) RequestIDR(firstFrame bool) {
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
