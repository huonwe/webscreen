package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Session struct {
	sessionType string
	ctx         context.Context
	// XVFB/Xorg
	X11Display     string
	xorgConfigPath string
	xorgLogPath    string

	// Sway
	displayName   string
	xdgRuntimeDir string
	swaySock      string
	width, height int

	frameRate string // session or recorder

	// Input Event Controller
	controller *InputController
	// Connect to the webscreen server
	conn net.Conn
	// FFmpeg/wf-recorder process
	// FFmpeg/wf-recorder output (for logging/debugging)
	recorderOutput io.ReadCloser

	// Others
	cleanupOnce sync.Once
	processes   []*os.Process
}

func NewSession(sessionType string, ctx context.Context) (*Session, error) {
	s := &Session{
		sessionType: sessionType,
		ctx:         ctx,
	}
	switch sessionType {
	case SESSION_TYPE_WAYLAND:
		err := s.initWaylandEnv()
		if err != nil {
			return nil, fmt.Errorf("初始化 Wayland 环境失败: %v", err)
		}
	case SESSION_TYPE_XORG, SESSION_TYPE_XVFB:
		s.X11Display = XORG_DISPLAY // 默认使用 :99 显示器
		// 不需要额外的环境准备

	default:
		return nil, fmt.Errorf("未知的 session 类型: %s", sessionType)
	}

	go func() {
		<-s.ctx.Done()
		log.Println("Session context done, running automatic cleanup...")
		s.CleanUp()
	}()

	return s, nil
}

func (s *Session) LaunchSession(width, height, frame_rate int) error {
	s.width = width
	s.height = height
	s.frameRate = strconv.Itoa(frame_rate)
	switch s.sessionType {
	case SESSION_TYPE_WAYLAND:
		return s.launchWaylandSession(width, height, frame_rate)
	case SESSION_TYPE_XORG:
		return s.launchXorgSession(width, height, frame_rate)
	case SESSION_TYPE_XVFB:
		return s.launchXVFBSession(width, height, frame_rate)
	default:
		return fmt.Errorf("未知的 session 类型: %s", s.sessionType)
	}
}

func (s *Session) WaitSessionReady(tcpPort int) error {
	err := s.WaitTCP(tcpPort)
	if err != nil {
		return err
	}
	switch s.sessionType {
	case SESSION_TYPE_WAYLAND:
		return s.waitWaylandReady()
	case SESSION_TYPE_XORG, SESSION_TYPE_XVFB:
		return s.waitX11Ready()
	default:
		return fmt.Errorf("未知的 session 类型: %s", s.sessionType)
	}
}

func (s *Session) WaitTCP(port int) error {
	var err error
	var conn net.Conn
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return fmt.Errorf("Failed to start TCP listener on port %d: %v", port, err)
	}
	listener.(*net.TCPListener).SetDeadline(time.Now().Add(10 * time.Second)) // 设置 10 秒超时
	conn, err = listener.Accept()
	if err != nil {
		return fmt.Errorf("Failed to accept TCP connection on port %d: %v", port, err)
	}
	listener.Close()
	log.Println("TCP connection established:", port)
	s.conn = conn
	return nil
}

func (s *Session) SetupController() error {
	var err error
	switch s.sessionType {
	case SESSION_TYPE_WAYLAND:
		s.controller, err = NewInputController(CONTROLLER_TYPE_WAYLAND, "", uint16(s.width), uint16(s.height))
		if err != nil {
			return fmt.Errorf("创建 Wayland 虚拟外设失败, 请检查 /dev/uinput 权限: %v", err)
		} else {
			log.Println("成功创建 Wayland 虚拟 TouchPad / Keyboard!")
			go func() {
				if err := s.controller.ServeControlConn(s.conn); err != nil {
					log.Println("控制连接关闭:", err)
				}
			}()
		}
	case SESSION_TYPE_XORG, SESSION_TYPE_XVFB:
		s.controller, err = NewInputController(CONTROLLER_TYPE_X11, s.X11Display, uint16(s.width), uint16(s.height))
		if err != nil {
			return fmt.Errorf("创建 X11 虚拟外设失败: %v", err)
		} else {
			log.Println("成功创建 X11 虚拟 TouchPad / Keyboard!")
			go func() {
				if err := s.controller.ServeControlConn(s.conn); err != nil {
					log.Println("控制连接关闭:", err)
				}
			}()
		}
	}
	return nil
}

func (s *Session) CleanUp() {
	s.cleanupOnce.Do(func() {
		if s.processes != nil {
			// **反向遍历**，优先杀后启动的进程（或者分别发信号后异步 Wait，避免因为某个阻塞导致僵尸进程堆积）
			for i := len(s.processes) - 1; i >= 0; i-- {
				proc := s.processes[i]
				if proc == nil {
					continue
				}
				log.Printf("Killing process group PID %d...\n", proc.Pid)
				// 对此进程及其子进程发送 SIGKILL
				syscall.Kill(-proc.Pid, syscall.SIGKILL)
				// 防止有些由于 Setpgid 尚未生效，也对本身发一份 SIGKILL 以防万一
				syscall.Kill(proc.Pid, syscall.SIGKILL)
			}

			// 单独去 Wait，确认资源回收，防僵尸
			for i := len(s.processes) - 1; i >= 0; i-- {
				proc := s.processes[i]
				if proc == nil {
					continue
				}
				log.Printf("Waiting for process PID %d...\n", proc.Pid)
				// 防止 wait 无限阻塞，特别是 wf-recorder 内部线程不干净的问题
				done := make(chan struct{})
				go func(p *os.Process) {
					p.Wait()
					close(done)
				}(proc)

				select {
				case <-done:
					log.Printf("Process PID %d reaped.\n", proc.Pid)
				case <-time.After(3 * time.Second): // 给 3 秒时间收尸
					log.Printf("Warning: Waiting for process PID %d timed out.\n", proc.Pid)
				}
			}
			log.Println("All child processes killed and reaped.")
		}
		if s.conn != nil {
			s.conn.Close()
		}
		if s.controller != nil {
			s.controller.Close()
		}
		if s.recorderOutput != nil {
			s.recorderOutput.Close()
		}
		// sway Cleanup
		xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if xdgRuntimeDir != "" && s.swaySock != "" {
			os.Remove(filepath.Join(xdgRuntimeDir, s.swaySock))
		}
		if xdgRuntimeDir != "" && s.X11Display != "" {
			x11displayNo, err := strconv.Atoi(strings.TrimPrefix(s.X11Display, ":"))
			if err == nil {
				os.Remove(filepath.Join(xdgRuntimeDir, fmt.Sprintf("X%d", x11displayNo)))
			}
		}
		// Xorg/Xvfb Cleanup
		if s.xorgConfigPath != "" {
			os.Remove(s.xorgConfigPath)
		}
		if s.xorgLogPath != "" {
			os.Remove(s.xorgLogPath)
		}
		if s.X11Display == "" {
			return
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
	})
}

func (s *Session) RunCmd(cmdStr string) {
	// 根据 sessionType 分发到不同的 RunCmd 实现
	switch s.sessionType {
	case SESSION_TYPE_XVFB, SESSION_TYPE_XORG:
		s.X11RunCmd(cmdStr)
	case SESSION_TYPE_WAYLAND:
		s.WaylandRunCmd(cmdStr)
	default:
		log.Printf("未知的 session 类型: %s\n", s.sessionType)
	}
}

// func (s *Session) RunXterm() {
// 	switch s.sessionType {
// 	case SESSION_TYPE_WAYLAND:
// 		// 通过 swaymsg 让 Sway 进程拉起应用，避免直接启动 xterm 时缺少 DISPLAY
// 		s.RunCmd("xterm")
// 	case SESSION_TYPE_XORG, SESSION_TYPE_XVFB:
// 		s.RunCmd("xterm")
// 	}
// }

func (s *Session) StartRecord(codec string, resolution string, bitRate string, frameRate int) error {
	log.Printf("Try Starting recording sessionType %s, resolution %s, bitrate %s, framerate %d\n", s.sessionType, resolution, bitRate, frameRate)
	switch s.sessionType {
	case SESSION_TYPE_WAYLAND:
		err := s.StartWfRecorder(codec, resolution, bitRate, frameRate)
		log.Printf("Started wf-recorder with codec %s, resolution %s, bitrate %s, framerate %d\n", codec, resolution, bitRate, frameRate)
		if err != nil {
			log.Printf("启动 wf-recorder 失败: %v\n", err)
			return fmt.Errorf("启动 wf-recorder 失败: %v", err)
		}
	case SESSION_TYPE_XORG, SESSION_TYPE_XVFB:
		err := s.StartFFmpeg(codec, resolution, bitRate, frameRate)
		log.Printf("Started FFmpeg with codec %s, resolution %s, bitrate %s, framerate %d\n", codec, resolution, bitRate, frameRate)
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
