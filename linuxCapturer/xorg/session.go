package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	lc "webscreen/linuxCapturer"
)

type X11Session struct {
	Display int
	Cmd     *os.Process
	Conn    net.Conn

	ffmpegOutput io.ReadCloser

	controller  *lc.InputController
	cleanupOnce sync.Once

	xorgConfigPath string
	xorgLogPath    string
}

func NewX11Session(tcpPort string, width int, height int, displayNum int, depth int, xorgDriver string) (*X11Session, error) {
	configPath, logPath, err := writeXorgConfig(width, height, depth, xorgDriver)
	if err != nil {
		return nil, err
	}

	// 👇 将 "Xorg" 改为 "X" 或者是绝对路径 "/usr/bin/X"
	xorgCmd := exec.Command("X",
		fmt.Sprintf(":%d", displayNum),
		"-config", configPath,
		"-noreset",
		"-nolisten", "tcp",
		// "-keeptty",  // 之前加的这个可以先注释掉
		"+extension", "GLX",
		"+extension", "RANDR",
		"+extension", "RENDER",
		"vt7", // 👈 强制告诉 Xorg 去使用 7 号控制台
		"-logfile", logPath,
	)
	xorgCmd.Stderr = os.Stderr
	xorgCmd.Env = append(os.Environ(),
		"LD_LIBRARY_PATH=/usr/lib/aarch64-linux-gnu/mali:$LD_LIBRARY_PATH",
	)
	if err := xorgCmd.Start(); err != nil {
		return nil, err
	}

	session := &X11Session{
		Display:        displayNum,
		Cmd:            xorgCmd.Process,
		xorgConfigPath: configPath,
		xorgLogPath:    logPath,
	}

	if err := session.waitLaunchFinished(); err != nil {
		session.CleanUp()
		return nil, err
	}

	log.Printf("listening at %s...\n", tcpPort)
	conn := lc.WaitTCP(tcpPort)
	session.Conn = conn
	log.Printf("TCP connection established at %s\n", tcpPort)

	go session.RunDesktopSession()

	session.controller, err = lc.NewInputController(lc.CONTROLLER_TYPE_X11, fmt.Sprintf(":%d", session.Display), uint16(width), uint16(height))
	if err != nil {
		session.CleanUp()
		return nil, fmt.Errorf("failed to create input controller: %w", err)
	}
	go session.controller.ServeControlConn(conn)
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
		// 你的代码里写了 DRI 3，由于之前我们分析过 Vendor 驱动更稳的是 DRI 2，建议改回 2 试试
		builder.WriteString("    Option \"DRI\" \"3\"\n")
		builder.WriteString("    Option \"AccelMethod\" \"glamor\"\n")
		builder.WriteString("    Option \"kmsdev\" \"/dev/dri/card0\"\n")

		// 👇 核心修改 1：使用 SWcursor 明确强制软件渲染
		builder.WriteString("    Option \"SWcursor\" \"on\"\n")
		// 注释掉 HWCursor，防止参数解析冲突
		// builder.WriteString("    Option \"HWCursor\" \"off\"\n")

		builder.WriteString("    Option \"AllowEmptyInitialConfiguration\" \"True\"\n")

		// 👇 核心修改 2：千万不要开 ShadowPrimary，它会导致软件鼠标无法输出到 DRM Plane
		// builder.WriteString("    Option \"ShadowPrimary\" \"true\"\n")
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

func (s *X11Session) StartFFmpeg(codec string, resolution string, bitRate string, frameRate string) error {
	var bestEncoder string
	switch codec {
	case "h264":
		bestEncoder = lc.GetBestH264Encoder()
	case "hevc":
		bestEncoder = lc.GetBestHEVCEncoder()
	default:
		return fmt.Errorf("不支持的编码格式: %s", codec)
	}

	log.Printf("Encoder: %s\n", bestEncoder)
	// _preset := "ultrafast"
	// if strings.Contains(bestEncoder, "nvenc") {
	// 	_preset = "p1"
	// }
	// if strings.Contains(bestEncoder, "qsv") {
	// 	_preset = "veryfast"
	// }
	// if strings.Contains(bestEncoder, "amf") {
	// 	_preset = "speed"
	// }
	// 动态生成滤镜链，处理 4K 到目标分辨率的缩放，并修复红蓝反转
	parts := strings.Split(resolution, "x")
	width, _ := strconv.Atoi(parts[0])
	height, _ := strconv.Atoi(parts[1])
	filterStr := fmt.Sprintf("hwdownload,format=bgr0,colorchannelmixer=rr=0:rb=1:br=1:bb=0,scale=%d:%d,format=nv12", width, height)
	ffmpegCmd := exec.Command("ffmpeg",
		"-device", "/dev/dri/card0",
		"-f", "kmsgrab",
		"-framerate", frameRate,
		"-i", "-",

		// 核心滤镜链：下载 -> 翻转红蓝通道 -> 缩放 -> 转换格式
		"-vf", filterStr,

		"-c:v", "h264_rkmpp",
		"-b:v", bitRate,
		"-maxrate", bitRate,
		"-g", "60",
		"-bf", "0",
		"-f", codec,
		"pipe:3",
	)
	ffmpegCmd.Env = append(os.Environ(), fmt.Sprintf("DISPLAY=:%d", s.Display))
	ffmpegCmd.Stderr = os.Stderr

	// 创建匿名管道
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}

	ffmpegCmd.ExtraFiles = []*os.File{pw}
	ffmpegCmd.Stdin = nil

	if err := ffmpegCmd.Start(); err != nil {
		log.Printf("FFmpeg 启动失败: %v", err)
		pw.Close()
		pr.Close()
		return err
	}

	// 【重要】启动后在父进程关闭写入端，否则会导致读取端无法收到 EOF
	pw.Close()

	s.ffmpegOutput = pr
	return nil
}
