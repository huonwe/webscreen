package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"webscreen/linuxRecorder/config"
)

func (s *Session) launchXorgSession(width int, height int, frameRate int) error {
	configPath, logPath, err := writeXorgConfig(width, height, 24)
	if err != nil {
		return fmt.Errorf("failed to write Xorg config: %w", err)
	}
	s.xorgConfigPath = configPath
	s.xorgLogPath = logPath

	// 👇 将 "Xorg" 改为 "X" 或者是绝对路径 "/usr/bin/X"
	xorgCmd := exec.Command("X",
		s.X11Display, // 使用我们指定的显示器 :99
		"-config", configPath,
		"-noreset",
		"-nolisten", "tcp",
		// "-keeptty",  // 之前加的这个可以先注释掉
		"+extension", "GLX",
		"+extension", "RANDR",
		"+extension", "RENDER",
		// "vt7", // 👈 强制告诉 Xorg 去使用 7 号控制台
		"-logfile", logPath,
	)
	xorgCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	xorgCmd.Stderr = os.Stderr
	// Patch for Mali GPU
	// If you have a Mali GPU, you might need to set LD_LIBRARY_PATH to find the driver libraries. Adjust the path as needed for your system.
	if fileExists("/usr/lib/aarch64-linux-gnu/mali") {
		xorgCmd.Env = append(os.Environ(),
			"LD_LIBRARY_PATH=/usr/lib/aarch64-linux-gnu/mali:$LD_LIBRARY_PATH",
		)
	}

	if err := s.SpawnProcess(xorgCmd, "Xorg"); err != nil {
		return err
	}
	return nil

}

func writeXorgConfig(width, height, depth int) (string, string, error) {
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

	var driver string
	if fileExists("/dev/dri/card0") {
		driver = "modesetting"
	} else {
		driver = "dummy"
	}
	configContent := config.GetXorgConfig(driver, width, height, depth)

	if _, err := configFile.WriteString(configContent); err != nil {
		os.Remove(configFile.Name())
		os.Remove(logFile.Name())
		return "", "", fmt.Errorf("failed to write Xorg config: %w", err)
	}

	return configFile.Name(), logFile.Name(), nil
}

func (s *Session) StartFFmpeg(codec string, resolution string, bitRate string, frameRate int) error {

	var bestEncoder string
	switch codec {
	case "h264":
		bestEncoder = GetBestH264Encoder()
	case "h265", "hevc":
		codec = "hevc" // 统一格式名，供 ffmpeg 的 -f 参数使用
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
	cmdArgs := []string{
		"-f", "x11grab",
		"-framerate", strconv.Itoa(frameRate),
		"-video_size", resolution, // 使用定义的变量
		"-i", s.X11Display, // 连到我们刚创建的 :99

		// 编码参数
		"-c:v", bestEncoder,
		"-b:v", bitRate,
		"-maxrate", bitRate,
		"-g", "120", // GOP 长度 120 帧（2秒@60fps）
		"-bf", "0", // 禁用 B 帧
		"-preset", _preset,
		"-pix_fmt", "yuv420p", // 注意 FFmpeg 是 -pix_fmt 而不是 -x
	}

	if codec == "h264" {
		cmdArgs = append(cmdArgs,
			"-profile:v", "baseline",
			"-level", "4.1",
		)
	} else if codec == "hevc" {
		cmdArgs = append(cmdArgs,
			"-profile:v", "main",
		)
	}

	cmdArgs = append(cmdArgs,
		"-f", codec, // 强制输出裸流 (h264/hevc)
		"pipe:3",
	)

	cmd := exec.CommandContext(s.ctx, "ffmpeg", cmdArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = append(os.Environ(), fmt.Sprintf("DISPLAY=%s", s.X11Display))
	cmd.Stderr = os.Stderr
	log.Printf("Running FFmpeg command: %s\n", strings.Join(cmd.Args, " "))

	// 创建匿名管道
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}

	cmd.ExtraFiles = []*os.File{pw}
	cmd.Stdin = nil

	if err := s.SpawnProcess(cmd, "FFmpeg"); err != nil {
		log.Printf("FFmpeg 启动失败: %v", err)
		pw.Close()
		pr.Close()
		return err
	}

	// 【重要】启动后在父进程关闭写入端，否则会导致读取端无法收到 EOF
	pw.Close()

	s.recorderOutput = pr
	return nil
}

func (s *Session) X11RunXfce4Session() {
	// 等待 1 秒让 Xvfb 初始化完成
	cmd := exec.CommandContext(s.ctx, "dbus-run-session", "xfce4-session")
	cmd.Env = append(os.Environ(), "DISPLAY="+s.X11Display)
	if err := s.SpawnProcess(cmd, "xfce4-session"); err != nil {
		log.Println("failed to start Xfce4 session:", err)
	}
}

func (s *Session) X11RunCmd(cmdStr string) {
	cmd := exec.CommandContext(s.ctx, "bash", "-c", cmdStr)
	cmd.Env = append(os.Environ(), "DISPLAY="+s.X11Display)

	// 对于直接通过 runCmd 运行的随意短命令，我们也走统一后台防僵尸处理即可
	if err := s.SpawnProcess(cmd, "x11-runcmd"); err != nil {
		log.Println("failed to run command:", err)
	}
}

func (s *Session) waitX11Ready() error {
	// 等待 Xvfb 的 Socket 文件生成，最多等 5 秒
	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = "/tmp"
	}
	socketFile := filepath.Join(tmpDir, ".X11-unix", fmt.Sprintf("X%s", strings.TrimPrefix(s.X11Display, ":")))
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
	s.X11RunXfce4Session()
	return nil
}

func (s *Session) findX11Display() {
	tmpDir := "/tmp/.X11-unix"
	for i := 0; i < 20; i++ {
		files, _ := filepath.Glob(filepath.Join(tmpDir, "X*"))
		if len(files) > 0 {
			for _, f := range files {
				displayNum := strings.TrimPrefix(filepath.Base(f), "X")
				s.X11Display = ":" + displayNum
			}
			log.Printf("Found Xwayland display: %s\n", s.X11Display)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}
