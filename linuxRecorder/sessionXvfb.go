package main

import (
	"fmt"
	"os/exec"
)

func (s *Session) launchXVFBSession(width int, height int, frameRate int) error {
	// Xvfb 命令: Xvfb :99 -ac -screen 0 1920x1080x24
	// -nolisten tcp: 为了安全，不监听 TCP 端口，只走 Unix Socket
	xvfbCmd := exec.CommandContext(s.ctx, "Xvfb", s.X11Display, "-ac", "-screen", "0", fmt.Sprintf("%dx%dx%d", width, height, COLOR_DEPTH), "-nolisten", "tcp")

	if err := s.SpawnProcess(xvfbCmd, "Xvfb"); err != nil {
		return err
	}
	return nil
}
