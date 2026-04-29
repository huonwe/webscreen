package main

import (
	"fmt"
	"os/exec"
	"syscall"
)

func (s *Session) launchXVFBSession(width int, height int, frameRate int) error {
	// Xvfb 命令: Xvfb :99 -ac -screen 0 1920x1080x24
	// -nolisten tcp: 为了安全，不监听 TCP 端口，只走 Unix Socket
	xvfbCmd := exec.CommandContext(s.ctx, "Xvfb", s.X11Display, "-ac", "-screen", "0", fmt.Sprintf("%dx%dx%d", width, height, COLOR_DEPTH), "-nolisten", "tcp")
	xvfbCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// 将 Xvfb 的输出重定向到空，或者是 os.Stdout 以便调试
	if err := xvfbCmd.Start(); err != nil {
		return err
	}
	s.processes = append(s.processes, xvfbCmd.Process)
	return nil
}
