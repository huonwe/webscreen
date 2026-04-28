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
)

type Session struct {
	sessionType string // "xvfb", "xorg", "sway"
	// XVFB
	X11Display string

	// Xorg
	xorgConfigPath string
	xorgLogPath    string

	// Sway
	displayName   string
	swaySock      string
	width, height int

	// Input Event Controller
	controller *InputController
	// Connect to the webscreen server
	conn net.Conn
	// FFmpeg/wf-recorder process
	cmdProcess *os.Process
	// FFmpeg/wf-recorder output (for logging/debugging)
	processOutput io.ReadCloser

	// Others
	cleanupOnce sync.Once
}

func (s *Session) StartFFmpeg(codec string, resolution string, bitRate string, frameRate string) error {

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
	// If want to use kmsgrab, the command would be like this:
	// ffmpeg -f kmsgrab -framerate 30 -i - -vf "hwdownload,format=bgr0,colorchannelmixer=rr=0:rb=1:br=1:bb=0,scale=1280:720,format=nv12" -c:v h264_nvenc -b:v 4M -maxrate 4M -g 60 -bf 0 -preset p1 -x yuv420p -f h264 -
	// filterStr := fmt.Sprintf("hwdownload,format=bgr0,colorchannelmixer=rr=0:rb=1:br=1:bb=0,scale=%d:%d,format=nv12", width, height)
	ffmpegCmd := exec.Command("ffmpeg",
		"-f", "x11grab",
		"-framerate", frameRate,
		"-video_size", resolution, // 使用定义的变量
		"-i", s.X11Display, // 连到我们刚创建的 :99

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
		"pipe:3",
	)
	ffmpegCmd.Env = append(os.Environ(), fmt.Sprintf("DISPLAY=%s", s.X11Display))
	ffmpegCmd.Stderr = os.Stderr
	log.Printf("Running FFmpeg command: %s\n", strings.Join(ffmpegCmd.Args, " "))

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

	s.processOutput = pr
	return nil
}

func (s *Session) CleanUp() {
	s.cleanupOnce.Do(func() {
		if s.cmdProcess != nil {
			s.cmdProcess.Kill()
			s.cmdProcess.Wait()
		}
		if s.conn != nil {
			s.conn.Close()
		}
		if s.controller != nil {
			s.controller.Close()
		}
		if s.processOutput != nil {
			s.processOutput.Close()
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

		x11displayNo, err := strconv.Atoi(strings.TrimPrefix(s.X11Display, ":"))
		if err != nil {
			log.Printf("无法解析 DISPLAY 号码: %s\n", s.X11Display)
			return
		}
		socketFile := filepath.Join(tmpDir, ".X11-unix", fmt.Sprintf("X%d", x11displayNo))
		lockFile := filepath.Join(tmpDir, fmt.Sprintf(".X%d-lock", x11displayNo))
		os.Remove(socketFile)
		os.Remove(lockFile)
		xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if xdgRuntimeDir != "" && s.X11Display != "" {
			os.Remove(filepath.Join(xdgRuntimeDir, fmt.Sprintf("X%d", x11displayNo)))
		}
		if xdgRuntimeDir != "" && s.swaySock != "" {
			os.Remove(filepath.Join(xdgRuntimeDir, s.swaySock))
		}
	})
}

func (s *Session) RunCmd(cmdStr string) int {
	// 根据 sessionType 分发到不同的 RunCmd 实现
	switch s.sessionType {
	case "xvfb", "xorg":
		return s.X11RunCmd(cmdStr)
	case "wayland":
		return s.WaylandRunCmd(cmdStr)
	default:
		log.Printf("未知的 session 类型: %s\n", s.sessionType)
		return -1
	}
}

func (s *Session) RunXterm() {
	switch s.sessionType {
	case "wayland":
		// 通过 swaymsg 让 Sway 进程拉起应用，避免直接启动 xterm 时缺少 DISPLAY
		s.RunCmd("swaymsg exec 'xterm'")
	case "xorg", "xvfb":
		s.RunCmd("xterm")
	}
}

func (s *Session) ServeRecord(codec string, resolution string, bitRate string, frameRate string) error {
	switch s.sessionType {
	case "wayland":
		err := s.StartWfRecorder(codec, resolution, bitRate, frameRate)
		if err != nil {
			log.Printf("启动 wf-recorder 失败: %v\n", err)
			return fmt.Errorf("启动 wf-recorder 失败: %v", err)
		}
	case "xorg", "xvfb":
		err := s.StartFFmpeg(codec, resolution, bitRate, frameRate)
		if err != nil {
			log.Printf("启动 FFmpeg 失败: %v\n", err)
			return fmt.Errorf("启动 FFmpeg 失败: %v", err)
		}
	}
	return nil
}

func (s *Session) Stop() {
	s.CleanUp()
}
