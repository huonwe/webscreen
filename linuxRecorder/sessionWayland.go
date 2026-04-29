package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"webscreen/linuxRecorder/config"
)

func (s *Session) initWaylandEnv() error {
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntimeDir == "" {
		xdgRuntimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
		err := os.MkdirAll(xdgRuntimeDir, 0700)
		if err != nil {
			return fmt.Errorf("Create XDG_RUNTIME_DIR Failed: %v", err)
		}
		os.Setenv("XDG_RUNTIME_DIR", xdgRuntimeDir)
	}

	s.xdgRuntimeDir = xdgRuntimeDir

	// 启动前清理残留的 Wayland Socket 和 Sway IPC Socket，避免冲突
	files, _ := filepath.Glob(filepath.Join(xdgRuntimeDir, "wayland-[0-9]*"))
	for _, f := range files {
		os.Remove(f)
	}
	files, _ = filepath.Glob(filepath.Join(xdgRuntimeDir, "sway-ipc.*.sock"))
	for _, f := range files {
		os.Remove(f)
	}
	return nil
}

func (s *Session) launchWaylandSession(width int, height int, frameRate int) error {
	if s.xdgRuntimeDir == "" {
		return fmt.Errorf("XDG_RUNTIME_DIR is not set")
	}
	swayConfig := filepath.Join(s.xdgRuntimeDir, "sway-headless.conf")
	configContent := config.GetSwayHeadlessConf(width, height, fmt.Sprintf("%d", frameRate))
	os.WriteFile(swayConfig, []byte(configContent), 0600)

	swayCmd := exec.CommandContext(s.ctx, "sway", "-c", swayConfig)
	// swayEnv := envWithoutKey(os.Environ(), "WLR_LIBINPUT_NO_DEVICES")
	swayCmd.Env = append(os.Environ(),
		// 必须同时开启 headless 和 libinput
		"WLR_BACKENDS=headless,libinput",
		// 告诉 libseat 去找 seatd 代理，不要自己动 tty
		"SEATD_SOCK=/run/seatd.sock",
		// 确保 libinput 被允许扫描设备
		"WLR_LIBINPUT_NO_DEVICES=0",
		// 其他你原有的环境变量...
		"WLR_RENDERER=pixman", // 强制使用软件渲染，避免某些 GPU 驱动的兼容性问题
	)
	swayCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := swayCmd.Start(); err != nil {
		return err
	}
	s.processes = append(s.processes, swayCmd.Process)
	return nil
}

func (s *Session) waitWaylandReady() error {
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntimeDir == "" {
		xdgRuntimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
		log.Printf("Warn: XDG_RUNTIME_DIR not set, defaulting to %s\n", xdgRuntimeDir)
	}
	for i := 0; i < 50; i++ {
		if s.displayName == "" {
			files, _ := filepath.Glob(filepath.Join(xdgRuntimeDir, "wayland-[0-9]*"))
			if len(files) > 0 {
				s.displayName = filepath.Base(files[0])
			}
		}
		if s.swaySock == "" {
			files, _ := filepath.Glob(filepath.Join(xdgRuntimeDir, "sway-ipc.*.sock"))
			if len(files) > 0 {
				s.swaySock = filepath.Base(files[0])
			}
		}
		if s.displayName != "" && s.swaySock != "" {
			log.Printf("Wayland session ready: DISPLAY=%s, SWAYSOCK=%s\n", s.displayName, s.swaySock)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("Sway headless Timeout")
}

// 补充类似于 XvfbSession 的 RunCmd 命令
func (s *Session) WaylandRunCmd(cmdStr string) int {
	cmd := exec.CommandContext(s.ctx, "swaymsg", "exec", cmdStr)

	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntimeDir == "" {
		xdgRuntimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}

	swaySock := s.swaySock
	if swaySock != "" && !filepath.IsAbs(swaySock) {
		swaySock = filepath.Join(xdgRuntimeDir, swaySock)
	}

	cmd.Env = append(os.Environ(),
		fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir),
	)
	if swaySock != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SWAYSOCK=%s", swaySock))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		log.Println("启动命令失败:", err)
		return -1
	}
	// swaymsg 本身是个执行完就会马上退出的短命命令，不要把它放入 Session 统一监控中
	// 否则 Wait 收尸时它早就因为先早退出而变成僵尸。
	// 我们直接启动 goroutine 来 Wait 它
	go func() {
		cmd.Wait()
	}()
	return 0
}

func (s *Session) StartWfRecorder(codec string, resolution string, bitRate string, frameRate int) error {
	var encoder string
	switch codec {
	case "h264":
		encoder = GetBestH264Encoder()
	case "hevc", "h265":
		codec = "hevc" // wf-recorder 里统一叫 hevc
		encoder = GetBestHEVCEncoder()
	default:
		return fmt.Errorf("不支持的编码格式: %s", codec)
	}

	// 1. 创建一对匿名管道
	// pr: Go 程序读取端 (Read), pw: wf-recorder 写入端 (Write)
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}

	_preset := "ultrafast"
	bitRate = "1M"

	// 2. 构造参数
	args := []string{
		"--output", "HEADLESS-1",
		"-g", fmt.Sprintf("0,0 %dx%d", s.width, s.height),
		"--muxer", codec,
		"--codec", encoder,
		"--file", "/dev/fd/3",
		"-x", "yuv420p",
		"-D",
		"-p", "preset=" + _preset,
		"-p", "tune=zerolatency",
	}
	args = append(args,
		// --- 以下是针对 WebRTC 的关键优化 ---
		// "-p", "profile=baseline", // 强制使用 Baseline Profile，这是 WebRTC 的最爱
		// "-p", "level=3.1", // 限制 Level，避免超出浏览器硬解能力上限
		// "-p", "g=60", // 缩短 GOP，强制每 30 帧（0.5秒）出一个关键帧
		"-p", "x264-params=sliced-threads=0:slices=1", // 【关键修复】禁用多线程切片，确保每帧只有一个 VCL NALU
		"-p", "slices=1", // 【关键修复】禁用多 slice 编码，确保每帧只有一个 VCL NALU
		// "-p", "keyint_min=60", // 最小关键帧间隔
		// "-p", "scenecut=0", // 关闭场景切换检测，确保 GOP 长度绝对固定
		// "-p", "intra-refresh=1", // 【重点】开启周期内帧刷新，WebRTC 最爱，能消除 I 帧带来的瞬间带宽波动
		"-p", "bf=0", // 禁用 B 帧
		"-p", "b="+bitRate,
	)

	commandName := "wf-recorder"
	commandArgs := args
	cmd := exec.CommandContext(s.ctx, commandName, commandArgs...)
	// 3. 设置子进程环境
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir),
		fmt.Sprintf("WAYLAND_DISPLAY=%s", s.displayName),
	)

	// 将管道写入端交给子进程的 ExtraFiles
	cmd.ExtraFiles = []*os.File{pw}

	// 关掉 stdin，防止任何交互提示卡住进程
	cmd.Stdin = nil
	cmd.Stderr = os.Stderr

	// 4. 启动
	if err := cmd.Start(); err != nil {
		return err
	}

	// 【重要】启动后在父进程关闭写入端，否则会导致读取端无法收到 EOF
	pw.Close()

	s.recorderOutput = pr
	s.processes = append(s.processes, cmd.Process)
	return nil
}
