package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	lc "webscreen/linuxCapturer"
)

type XvfbSession struct {
	Display int
	Cmd     *os.Process
	Conn    net.Conn

	ffmpegOutput io.ReadCloser

	controller *lc.InputController
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
	conn := lc.WaitTCP(tcpPort)
	session.Conn = conn
	log.Printf("TCP connection established at %s\n", tcpPort)
	go session.RunXfce4Session()

	session.controller, err = lc.NewInputController(lc.CONTROLLER_TYPE_X11, fmt.Sprintf(":%d", session.Display), uint16(width), uint16(height))
	if err != nil {
		session.CleanUp()
		return nil, fmt.Errorf("failed to create input controller: %w", err)
	}
	go session.controller.ServeControlConn(conn)
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

func (s *XvfbSession) StartFFmpeg(codec string, resolution string, bitRate string, frameRate string) error {

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
		// "-tune", "zerolatency",
		// "-p", "x264-params=sliced-threads=0:slices=1", // 【关键修复】禁用多线程切片，确保每帧只有一个 VCL NALU
		// "-p", "slices=1", // 【关键修复】禁用多 slice 编码，确保每帧只有一个 VCL NALU
		"-x", "yuv420p",
		// "-D",

		"-f", codec,
		"-",
	)
	// 注入 DISPLAY 变量
	ffmpegCmd.Env = append(os.Environ(), fmt.Sprintf("DISPLAY=:%d", s.Display))
	ffmpegCmd.Stderr = os.Stderr // 错误日志打印出来
	log.Printf("Running FFmpeg command: %s\n", strings.Join(ffmpegCmd.Args, " "))

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
