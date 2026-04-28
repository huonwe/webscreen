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

func NewXorgSession(tcpPort string, width int, height int, displayNo int, depth int, xorgDriver string) (*Session, error) {
	configPath, logPath, err := writeXorgConfig(width, height, depth, xorgDriver)
	if err != nil {
		return nil, err
	}

	// 👇 将 "Xorg" 改为 "X" 或者是绝对路径 "/usr/bin/X"
	xorgCmd := exec.Command("X",
		fmt.Sprintf(":%d", displayNo),
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
	xorgCmd.Stderr = os.Stderr
	xorgCmd.Env = append(os.Environ(),
		"LD_LIBRARY_PATH=/usr/lib/aarch64-linux-gnu/mali:$LD_LIBRARY_PATH",
	)
	if err := xorgCmd.Start(); err != nil {
		return nil, err
	}

	session := &Session{
		sessionType:    "x11",
		X11Display:     fmt.Sprintf(":%d", displayNo),
		cmdProcess:     xorgCmd.Process,
		xorgConfigPath: configPath,
		xorgLogPath:    logPath,
	}

	if err := session.waitX11Launch(); err != nil {
		session.CleanUp()
		return nil, err
	}

	log.Printf("listening at %s...\n", tcpPort)
	conn, err := WaitTCP(tcpPort)
	if err != nil {
		session.CleanUp()
		return nil, fmt.Errorf("failed to wait for TCP connection: %w", err)
	}
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
