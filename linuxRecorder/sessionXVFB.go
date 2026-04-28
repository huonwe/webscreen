package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func NewXVFBSession(tcpPort string, width int, height int, displayNo int, depth int) (*Session, error) {
	// Xvfb 命令: Xvfb :99 -ac -screen 0 1920x1080x24
	// -nolisten tcp: 为了安全，不监听 TCP 端口，只走 Unix Socket
	xvfbCmd := exec.Command("Xvfb", fmt.Sprintf(":%d", displayNo), "-ac", "-screen", "0", fmt.Sprintf("%dx%dx%d", width, height, depth), "-nolisten", "tcp")
	// 将 Xvfb 的输出重定向到空，或者是 os.Stdout 以便调试
	// xvfbCmd.Stdout = os.Stdout
	xvfbCmd.Stderr = os.Stderr

	if err := xvfbCmd.Start(); err != nil {
		return nil, err
	}
	log.Printf("Started Xvfb on display :%d with PID %d\n", displayNo, xvfbCmd.Process.Pid)
	session := &Session{
		sessionType: "xvfb",
		X11Display:  fmt.Sprintf(":%d", displayNo),
		cmdProcess:  xvfbCmd.Process,
	}
	err := session.waitX11Launch()
	if err != nil {
		return nil, err
	}
	log.Printf("listening at %s...\n", tcpPort)
	conn, err := WaitTCP(tcpPort)

	session.conn = conn
	log.Printf("TCP connection established at %s\n", tcpPort)

	session.controller, err = NewInputController(CONTROLLER_TYPE_X11, session.X11Display, uint16(width), uint16(height))
	if err != nil {
		session.CleanUp()
		return nil, fmt.Errorf("failed to create input controller: %w", err)
	}
	go session.controller.ServeControlConn(conn)
	return session, nil
}

func (s *Session) waitX11Launch() error {
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
	return nil
}

func (s *Session) X11RunXfce4Session() {
	// 等待 1 秒让 Xvfb 初始化完成
	cmd := exec.Command("dbus-run-session", "xfce4-session")
	cmd.Env = append(os.Environ(), "DISPLAY="+s.X11Display)
	if err := cmd.Start(); err != nil {
		log.Println("failed to start Xfce4 session:", err)
	}
}

func (s *Session) X11RunCmd(cmdStr string) int {
	cmd := exec.Command("bash", "-c", cmdStr)
	cmd.Env = append(os.Environ(), "DISPLAY="+s.X11Display)
	if err := cmd.Start(); err != nil {
		log.Println("failed to run command:", err)
		return -1
	}
	cmd.Wait()
	return cmd.ProcessState.ExitCode()
}
