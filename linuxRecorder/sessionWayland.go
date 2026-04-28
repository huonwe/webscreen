package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func NewWaylandSession(tcpPort string, width int, height int, frameRate string, cpuSet string) (*Session, error) {
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntimeDir == "" {
		xdgRuntimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
		os.Setenv("XDG_RUNTIME_DIR", xdgRuntimeDir)
	}
	os.MkdirAll(xdgRuntimeDir, 0700)

	files, _ := filepath.Glob(filepath.Join(xdgRuntimeDir, "wayland-[0-9]*"))
	for _, f := range files {
		os.Remove(f)
	}
	files, _ = filepath.Glob(filepath.Join(xdgRuntimeDir, "sway-ipc.*.sock"))
	for _, f := range files {
		os.Remove(f)
	}

	swayConfig := filepath.Join(xdgRuntimeDir, "sway-headless.conf")
	configContent := fmt.Sprintf(`# 基础配置
xwayland enable
output HEADLESS-1 resolution %dx%d@%sHz position 0 0

# === 外观与美化配置 ===

# 1. 设置背景壁纸 (fill 模式会按比例缩放并裁剪填满屏幕)
output HEADLESS-1 bg /home/hiroi/Downloads/124956717_p0.png fill

# 2. 全局字体设置
font pango:sans-serif 11

# 3. 窗口边框与间距 (现代平铺桌面风格)
# 取消默认的粗大标题栏，改为 2 像素的纯色边框
default_border pixel 2
default_floating_border normal

# 设置窗口之间的缝隙，让壁纸能透出来
gaps inner 8
gaps outer 4

# 4. 窗口颜色配置 (基于优雅的 Nord 主题配色)
# 格式：class                 border  backgr. text    indicator child_border
client.focused          #88c0d0 #434c5e #eceff4 #8fbcbb   #88c0d0
client.focused_inactive #3b4252 #2e3440 #d8dee9 #4c566a   #4c566a
client.unfocused        #2e3440 #2e3440 #d8dee9 #2e3440   #2e3440
client.urgent           #bf616a #bf616a #eceff4 #bf616a   #bf616a

# 5. 状态栏配置 (可选)
# 如果你只想要一个纯净的画面（比如为了无干扰地跑特定应用），可以取消下面 bar 的注释来隐藏默认的底部状态栏
# bar {
#     mode invisible
# }
`, width, height, frameRate)
	// configContent := ""
	os.WriteFile(swayConfig, []byte(configContent), 0600)

	swayCmd := exec.Command("sway", "-c", swayConfig)
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
	swayCmd.Stderr = os.Stderr

	if err := swayCmd.Start(); err != nil {
		return nil, err
	}

	session := &Session{
		sessionType: "wayland",
		cmdProcess:  swayCmd.Process,
		width:       width,
		height:      height,
	}

	err := session.waitWaylandLaunch(xdgRuntimeDir)
	if err != nil {
		return nil, err
	}

	log.Printf("listening at %s...\n", tcpPort)
	conn, err := WaitTCP(tcpPort)
	if err != nil {
		return nil, fmt.Errorf("等待 TCP 连接失败: %v", err)
	}
	session.conn = conn
	log.Printf("TCP connection established at %s\n", tcpPort)

	var errController error
	session.controller, errController = NewInputController(CONTROLLER_TYPE_WAYLAND, "", uint16(width), uint16(height))
	if errController != nil {
		log.Printf("创建 Wayland 虚拟外设失败, 请检查 /dev/uinput 权限: %v\n", errController)
	} else {
		log.Println("成功创建 Wayland 虚拟 TouchPad / Keyboard!")
	}

	go func() {
		if err := session.controller.ServeControlConn(conn); err != nil {
			log.Println("控制连接关闭:", err)
		}
	}()
	return session, nil
}

func (s *Session) waitWaylandLaunch(xdgRuntimeDir string) error {
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
	cmd := exec.Command("bash", "-c", cmdStr)

	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntimeDir == "" {
		xdgRuntimeDir = fmt.Sprintf("/run/user/%d", os.Getuid())
	}

	swaySock := s.swaySock
	if swaySock != "" && !filepath.IsAbs(swaySock) {
		swaySock = filepath.Join(xdgRuntimeDir, swaySock)
	}

	// 注入 Wayland/Sway 运行时环境给子进程
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir),
		fmt.Sprintf("WAYLAND_DISPLAY=%s", s.displayName),
	)
	if swaySock != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("SWAYSOCK=%s", swaySock))
	}
	if s.X11Display != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("DISPLAY=%s", s.X11Display))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("RunCmd: %s (WAYLAND_DISPLAY=%s, SWAYSOCK=%s, DISPLAY=%s)", cmdStr, s.displayName, swaySock, s.X11Display)

	if err := cmd.Start(); err != nil {
		log.Println("启动命令失败:", err)
		return -1
	}
	// 不在此处 Wait() 阻塞主线程，因为图形程序可能会一直前台运行
	// cmd.Wait()
	return 0
}

func (s *Session) StartWfRecorder(codec string, resolution string, bitRate string, frameRate string) error {
	var encoder string
	switch codec {
	case "h264":
		encoder = GetBestH264Encoder()
	case "hevc":
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

	ffmpegCmd := exec.Command(commandName, commandArgs...)

	// 3. 设置子进程环境
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	ffmpegCmd.Env = append(os.Environ(),
		fmt.Sprintf("XDG_RUNTIME_DIR=%s", xdgRuntimeDir),
		fmt.Sprintf("WAYLAND_DISPLAY=%s", s.displayName),
	)

	// 将管道写入端交给子进程的 ExtraFiles
	ffmpegCmd.ExtraFiles = []*os.File{pw}

	// 关掉 stdin，防止任何交互提示卡住进程
	ffmpegCmd.Stdin = nil
	ffmpegCmd.Stderr = os.Stderr

	// 4. 启动
	if err := ffmpegCmd.Start(); err != nil {
		return err
	}

	// 【重要】启动后在父进程关闭写入端，否则会导致读取端无法收到 EOF
	pw.Close()

	// 将读取端赋值给 session，后续的 Scanner 会从这里读
	s.processOutput = pr
	log.Printf("Started wf-recorder with PID %d, streaming to session.processOutput\n", ffmpegCmd.Process.Pid)

	return nil
}
