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
	"sync"
	"time"
)

type X11Session struct {
	Display int
	Cmd     *os.Process
	Conn    net.Conn

	ffmpegOutput io.ReadCloser

	controller *InputController
	cleanupOnce sync.Once

	xorgConfigPath string
	xorgLogPath    string
}

func NewX11Session(tcpPort string, width int, height int, displayNum int, depth int, xorgDriver string) (*X11Session, error) {
	configPath, logPath, err := writeXorgConfig(width, height, depth, xorgDriver)
	if err != nil {
		return nil, err
	}

	xorgCmd := exec.Command("Xorg", fmt.Sprintf(":%d", displayNum), "-config", configPath, "-noreset", "-nolisten", "tcp", "+extension", "GLX", "+extension", "RANDR", "+extension", "RENDER", "-logfile", logPath)
	xorgCmd.Stderr = os.Stderr

	if err := xorgCmd.Start(); err != nil {
		return nil, err
	}

	session := &X11Session{
		Display:        displayNum,
		Cmd:            xorgCmd.Process,
		xorgConfigPath:  configPath,
		xorgLogPath:     logPath,
	}

	if err := session.waitLaunchFinished(); err != nil {
		session.CleanUp()
		return nil, err
	}

	log.Printf("listening at %s...\n", tcpPort)
	conn := WaitTCP(tcpPort)
	session.Conn = conn
	log.Printf("TCP connection established at %s\n", tcpPort)

	go session.RunDesktopSession()

	session.controller, _ = NewInputController(fmt.Sprintf(":%d", session.Display))
	go session.HandleEvent()
	return session, nil
}

func writeXorgConfig(width, height, depth int, xorgDriver string) (string, string, error) {
	configFile, err := os.CreateTemp("", "webscreen-xorg-*.conf")
	if err != nil {
		return "", "", err
	}
	defer configFile.Close()

	logFile, err := os.CreateTemp("", "webscreen-xorg-*.log")
	if err != nil {
		os.Remove(configFile.Name())
		return "", "", err
	}
	logFile.Close()

	driver := strings.ToLower(strings.TrimSpace(xorgDriver))
	if driver == "" {
		driver = "auto"
	}

	if driver == "auto" {
		if fileExists("/dev/dri/card0") {
			driver = "modesetting"
		} else {
			driver = "dummy"
		}
	}

	var builder strings.Builder
	builder.WriteString("Section \"ServerFlags\"\n")
	builder.WriteString("    Option \"AutoAddGPU\" \"false\"\n")
	builder.WriteString("    Option \"AutoBindGPU\" \"false\"\n")
	builder.WriteString("EndSection\n\n")

	builder.WriteString("Section \"Monitor\"\n")
	builder.WriteString("    Identifier \"Monitor0\"\n")
	builder.WriteString("    Option \"DPMS\" \"false\"\n")
	builder.WriteString("EndSection\n\n")

	builder.WriteString("Section \"Device\"\n")
	builder.WriteString("    Identifier \"Device0\"\n")
	switch driver {
	case "nvidia":
		builder.WriteString("    Driver \"nvidia\"\n")
		builder.WriteString("    Option \"AllowEmptyInitialConfiguration\" \"True\"\n")
		builder.WriteString("    Option \"UseDisplayDevice\" \"None\"\n")
	case "modesetting":
		builder.WriteString("    Driver \"modesetting\"\n")
		builder.WriteString("    Option \"AccelMethod\" \"glamor\"\n")
		builder.WriteString("    Option \"DRI\" \"3\"\n")
		builder.WriteString("    Option \"AllowEmptyInitialConfiguration\" \"True\"\n")
	case "dummy":
		builder.WriteString("    Driver \"dummy\"\n")
		builder.WriteString("    VideoRam 256000\n")
	default:
		builder.WriteString("    Driver \"dummy\"\n")
		builder.WriteString("    VideoRam 256000\n")
	}
	builder.WriteString("EndSection\n\n")

	builder.WriteString("Section \"Screen\"\n")
	builder.WriteString("    Identifier \"Screen0\"\n")
	builder.WriteString("    Device \"Device0\"\n")
	builder.WriteString("    Monitor \"Monitor0\"\n")
	builder.WriteString("    DefaultDepth ")
	builder.WriteString(fmt.Sprintf("%d\n", depth))
	builder.WriteString("    SubSection \"Display\"\n")
	builder.WriteString(fmt.Sprintf("        Depth %d\n", depth))
	builder.WriteString(fmt.Sprintf("        Modes \"%dx%d\"\n", width, height))
	builder.WriteString(fmt.Sprintf("        Virtual %d %d\n", width, height))
	builder.WriteString("    EndSubSection\n")
	builder.WriteString("EndSection\n\n")

	if _, err := configFile.WriteString(builder.String()); err != nil {
		os.Remove(configFile.Name())
		os.Remove(logFile.Name())
		return "", "", err
	}

	return configFile.Name(), logFile.Name(), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (s *X11Session) CleanUp() {
	s.cleanupOnce.Do(func() {
		log.Println("正在清理资源，关闭原生 X11 虚拟显示器...")
		if s.Conn != nil {
			s.Conn.Close()
		}
		if s.controller != nil {
			s.controller.Close()
		}
		if s.ffmpegOutput != nil {
			s.ffmpegOutput.Close()
		}
		if s.Cmd != nil {
			s.Cmd.Kill()
			s.Cmd.Wait()
		}
		if s.xorgConfigPath != "" {
			os.Remove(s.xorgConfigPath)
		}
		if s.xorgLogPath != "" {
			os.Remove(s.xorgLogPath)
		}
		tmpDir := os.Getenv("TMPDIR")
		if tmpDir == "" {
			tmpDir = "/tmp"
		}
		socketFile := filepath.Join(tmpDir, ".X11-unix", fmt.Sprintf("X%d", s.Display))
		lockFile := filepath.Join(tmpDir, fmt.Sprintf(".X%d-lock", s.Display))
		os.Remove(socketFile)
		os.Remove(lockFile)
		log.Println("清理完成，程序退出。")
	})
}

func (s *X11Session) RunCmd(cmdStr string) int {
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Env = append(os.Environ(), fmt.Sprintf("DISPLAY=:%d", s.Display))
	if err := cmd.Start(); err != nil {
		log.Println("启动命令失败:", err)
		return -1
	}
	cmd.Wait()
	return cmd.ProcessState.ExitCode()
}

func (s *X11Session) RunDesktopSession() {
	cmd := exec.Command("dbus-run-session", "xfce4-session")
	cmd.Env = append(os.Environ(), "DISPLAY="+fmt.Sprintf(":%d", s.Display))
	if err := cmd.Start(); err != nil {
		log.Println("启动桌面失败:", err)
	}
}

func (s *X11Session) waitLaunchFinished() error {
	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = "/tmp"
	}
	socketFile := filepath.Join(tmpDir, ".X11-unix", fmt.Sprintf("X%d", s.Display))
	ready := false
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(socketFile); err == nil {
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !ready {
		return fmt.Errorf("Xorg timeout! Socket file not found: %s", socketFile)
	}
	return nil
}

func (s *X11Session) HandleEvent() {
	const (
		eventTypeKeyboard = 0x00
		EventTypeMouse    = 0x01
	)

	head := make([]byte, 1)
	for {
		_, err := io.ReadFull(s.Conn, head)
		if err != nil {
			log.Println("控制连接断开或读取错误:", err)
			return
		}
		switch head[0] {
		case EventTypeMouse:
			payload := make([]byte, 17)
			if _, err := io.ReadFull(s.Conn, payload); err != nil {
				log.Println("读取鼠标数据包失败:", err)
				return
			}

			action := payload[0]
			x := binary.BigEndian.Uint32(payload[1:5])
			y := binary.BigEndian.Uint32(payload[5:9])
			buttons := binary.BigEndian.Uint32(payload[9:13])
			deltaX := int16(binary.BigEndian.Uint16(payload[13:15]))
			deltaY := int16(binary.BigEndian.Uint16(payload[15:]))

			if s.controller == nil {
				log.Println("输入控制器未初始化，无法处理鼠标事件")
				continue
			}

			s.controller.HandleMouseEvent(action, int16(x), int16(y), buttons, deltaX, deltaY)
		case eventTypeKeyboard:
			payload := make([]byte, 5)
			if _, err := io.ReadFull(s.Conn, payload); err != nil {
				return
			}

			action := payload[0]
			webKeyCode := binary.BigEndian.Uint32(payload[1:5])
			x11Code := byte(webKeyCode)

			if s.controller != nil {
				s.controller.HandleKeyboardEvent(action, x11Code)
			}
		default:
			log.Printf("收到未知事件类型: 0x%X", head[0])
		}
	}
}

func (s *X11Session) StartFFmpeg(codec string, resolution string, bitRate string, frameRate string) error {
	var bestEncoder string
	switch codec {
	case "h264":
		bestEncoder = GetBestH264Encoder()
	case "hevc":
		bestEncoder = GetBestHEVCEncoder()
	default:
		return fmt.Errorf("不支持的编码格式: %s", codec)
	}

	log.Printf("Encoder: %s\n", bestEncoder)
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
		"-video_size", resolution,
		"-i", fmt.Sprintf(":%d", s.Display),
		"-c:v", bestEncoder,
		"-b:v", bitRate,
		"-maxrate", bitRate,
		"-g", "60",
		"-bf", "0",
		"-preset", _preset,
		"-x", "yuv420p",
		"-f", codec,
		"-",
	)
	ffmpegCmd.Env = append(os.Environ(), fmt.Sprintf("DISPLAY=:%d", s.Display))
	ffmpegCmd.Stderr = os.Stderr

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