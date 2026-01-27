package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type XvfbSession struct {
	Display int
	Cmd     *os.Process
	Conn    net.Conn

	ffmpegOutput io.ReadCloser

	controller *InputController
}

func NewXvfbSession(tcpPort string, width int, height int, DisplayNum int, depth int) (*XvfbSession, error) {
	// Xvfb 命令: Xvfb :99 -ac -screen 0 1920x1080x24
	// -nolisten tcp: 为了安全，不监听 TCP 端口，只走 Unix Socket
	xvfbCmd := exec.Command("Xvfb", fmt.Sprintf(":%d", DisplayNum), "-ac", "-screen", "0", fmt.Sprintf("%dx%dx%d", width, height, depth), "-nolisten", "tcp")
	// 将 Xvfb 的输出重定向到空，或者是 os.Stdout 以便调试
	// xvfbCmd.Stdout = os.Stdout
	xvfbCmd.Stderr = os.Stderr

	if err := xvfbCmd.Start(); err != nil {
		return nil, err
	}
	session := &XvfbSession{
		Display: DisplayNum,
		Cmd:     xvfbCmd.Process,
	}
	err := session.waitLaunchFinished()
	if err != nil {
		return nil, err
	}
	log.Printf("listening at %s...\n", tcpPort)
	conn := WaitTCP(tcpPort)
	session.Conn = conn
	log.Printf("TCP connection established at %s\n", tcpPort)
	go session.RunXfce4Session()

	session.controller, _ = NewInputController(fmt.Sprintf(":%d", session.Display))
	go session.HandleEvent()
	return session, nil

}

func (s *XvfbSession) CleanUp() {
	log.Println("正在清理资源，关闭虚拟显示器...")
	s.Conn.Close()
	if s.Cmd != nil {
		s.Cmd.Kill()
		s.Cmd.Wait() // 等待进程彻底结束
	}
	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = "/tmp"
	}
	socketFile := filepath.Join(tmpDir, ".X11-unix", fmt.Sprintf("X%d", s.Display))
	lockFile := filepath.Join(tmpDir, fmt.Sprintf(".X%d-lock", s.Display))
	// 清理锁文件（防止下次启动报错），虽然 Xvfb 正常退出会自动清理，但为了保险
	os.Remove(socketFile)
	os.Remove(lockFile)
	log.Println("清理完成，程序退出。")
}

func (s *XvfbSession) RunCmd(cmdStr string) int {
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Env = append(os.Environ(), fmt.Sprintf("DISPLAY=:%d", s.Display))
	if err := cmd.Start(); err != nil {
		log.Println("启动命令失败:", err)
		return -1
	}
	cmd.Wait()
	return cmd.ProcessState.ExitCode()
}

func (s *XvfbSession) RunXfce4Session() {
	// 等待 1 秒让 Xvfb 初始化完成
	cmd := exec.Command("dbus-run-session", "xfce4-session")
	cmd.Env = append(os.Environ(), "DISPLAY="+fmt.Sprintf(":%d", s.Display))
	if err := cmd.Start(); err != nil {
		log.Println("启动桌面失败:", err)
	}
}

func (s *XvfbSession) waitLaunchFinished() error {
	// 等待 Xvfb 的 Socket 文件生成，最多等 5 秒
	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = "/tmp"
	}
	socketFile := filepath.Join(tmpDir, ".X11-unix", fmt.Sprintf("X%d", s.Display))
	xvfbReady := false
	for i := 0; i < 50; i++ { // 50 * 100ms = 5秒
		if _, err := os.Stat(socketFile); err == nil {
			xvfbReady = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !xvfbReady {
		return fmt.Errorf("Xvfb Timeout! Socket file not found: %s", socketFile)
	}
	return nil
}

func (s *XvfbSession) HandleEvent() {
	const (
		eventTypeKeyboard = 0x00
		EventTypeMouse    = 0x01
	)

	// 预分配一个小 buffer 用于读取头部或完整包

	head := make([]byte, 1)
	for {
		_, err := io.ReadFull(s.Conn, head)
		if err != nil {
			log.Println("控制连接断开或读取错误:", err)
			return
		}
		eventType := head[0]
		switch eventType {
		case EventTypeMouse:
			// 2. 如果是鼠标事件，读取剩余的 17 字节
			payload := make([]byte, 17)
			_, err := io.ReadFull(s.Conn, payload)
			if err != nil {
				log.Println("读取鼠标数据包失败:", err)
				return
			}

			// 3. 解析数据 (BigEndian)
			action := payload[0]
			x := binary.BigEndian.Uint32(payload[1:5])
			y := binary.BigEndian.Uint32(payload[5:9])
			buttons := binary.BigEndian.Uint32(payload[9:13])
			deltaX := int16(binary.BigEndian.Uint16(payload[13:15]))
			deltaY := int16(binary.BigEndian.Uint16(payload[15:]))
			// log.Printf("收到鼠标事件: Action=%d, X=%d, Y=%d, Buttons=0x%X, DeltaX=%d, DeltaY=%d\n", action, x, y, buttons, deltaX, deltaY)

			if s.controller == nil {
				log.Println("输入控制器未初始化，无法处理鼠标事件")
				continue
			}

			// 4. 执行控制逻辑
			// 注意：这里需要把 uint32 转为 int16 传给 InputController
			s.controller.HandleMouseEvent(action, int16(x), int16(y), buttons, deltaX, deltaY)
		case eventTypeKeyboard:
			payload := make([]byte, 5)
			if _, err := io.ReadFull(s.Conn, payload); err != nil {
				return
			}

			action := payload[0] // 0=Down, 1=Up
			// Web 端传来的 KeyCode 往往是 DOM Code，这里收到的是 uint32
			// 注意：这里可能需要做 Web KeyCode -> X11 KeyCode 的映射表
			webKeyCode := binary.BigEndian.Uint32(payload[1:5])

			// 简单的映射示例 (需完善 Map)
			// x11Code := mapWebToX11(webKeyCode)
			// 暂时直接透传 (假设前端已经发了 X11 Code)
			x11Code := byte(webKeyCode)

			if s.controller != nil {
				s.controller.HandleKeyboardEvent(action, x11Code)
			}
		default:
			log.Printf("收到未知事件类型: 0x%X", eventType)
			// 如果有变长包，这里如果不处理会导致后续数据错乱
			// 建议实现一个通用的长度头，但在目前定长场景下，这样够用了
		}
	}
}

func (s *XvfbSession) StartFFmpeg(codec string, resolution string, bitRate string, frameRate string) error {

	var bestEncoder string
	switch codec {
	case "h264":
		bestEncoder = GetBestH264Encoder()
	case "hevc":
		bestEncoder = GetBestHEVCEncoder()
	default:
		return fmt.Errorf("不支持的编码格式: %s", codec)
	}

	log.Printf("使用的 H.264 编码器: %s\n", bestEncoder)
	_preset := "ultrafast"
	if strings.Contains(bestEncoder, "nvenc") {
		_preset = "p1"
	}
	if strings.Contains(bestEncoder, "qsv") {
		_preset = "veryfast"
	}
	if strings.Contains(bestEncoder, "amf") {
		_preset = "speed"
	}
	ffmpegCmd := exec.Command("ffmpeg",
		"-f", "x11grab",
		"-framerate", frameRate,
		"-video_size", resolution, // 使用定义的变量
		"-i", fmt.Sprintf(":%d", s.Display), // 连到我们刚创建的 :99

		// 编码参数
		"-c:v", bestEncoder, // 如果在 PC 上跑，改成 libx264
		"-b:v", bitRate,
		"-maxrate", bitRate,
		"-g", "60",
		"-bf", "0",
		"-preset", _preset,
		"-tune", "zerolatency",

		"-f", "h264",
		"-",
	)
	// 注入 DISPLAY 变量
	ffmpegCmd.Env = append(os.Environ(), fmt.Sprintf("DISPLAY=:%d", s.Display))
	ffmpegCmd.Stderr = os.Stderr // 错误日志打印出来

	var err error
	s.ffmpegOutput, err = ffmpegCmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := ffmpegCmd.Start(); err != nil {
		log.Printf("FFmpeg 启动失败: %v", err)
		return err
	}
	return nil
}
