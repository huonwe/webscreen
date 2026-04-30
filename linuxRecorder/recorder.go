package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

func main() {
	tcpPort := flag.Int("tcp_port", 27184, "server listen port")
	resolution := flag.String("resolution", "1920x1080", "virtual display resolution")
	bitRate := flag.String("bitrate", "8M", "streaming bitrate, e.g. 4M, 800K, 1000000")
	frameRate := flag.Int("framerate", 60, "frame rate for capturing")
	codec := flag.String("codec", "h264", "video codec: h264 or hevc")
	// cpuSet := flag.String("cpu_set", "", "optional CPU affinity for wf-recorder, for example 0 or 0-1")
	backend := flag.String("backend", "wayland", "capture backend: wayland, xorg, or xvfb")
	flag.Parse()
	log.Printf("Starting %s capturer with resolution %s, bitrate %s, framerate %d, codec %s\n", *backend, *resolution, *bitRate, *frameRate, *codec)

	_width, _height := strings.Split(*resolution, "x")[0], strings.Split(*resolution, "x")[1]
	width, err := strconv.Atoi(_width)
	if err != nil {
		log.Printf("Invalid width: %v", err)
		return
	}
	height, err := strconv.Atoi(_height)
	if err != nil {
		log.Printf("Invalid height: %v", err)
		return
	}

	var session *Session

	parentCtx := context.Background()
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	session, err = NewSession(*backend, ctx)
	if err != nil {
		log.Printf("Failed to create session  %s: %v", *backend, err)
		return
	}
	err = session.LaunchSession(width, height, *frameRate)
	if err != nil {
		log.Fatal("Failed to launch session: ", err)
	}
	err = session.WaitSessionReady(*tcpPort)
	if err != nil {
		log.Fatal("Failed to setup session: ", err)
	}

	err = session.SetupController()
	if err != nil {
		log.Printf("Warning: Failed to setup controller: %v", err)
	}

	// 监听 Ctrl+C，确保退出时执行清理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan // 阻塞直到收到信号
		session.CleanUp()
		os.Exit(0)
	}()

	// 确保 main 函数正常结束时也清理
	defer session.CleanUp()

	log.Printf("Recorder initialized with backend: %s", *backend)

	go session.RunCmd("xterm")

	err = session.StartRecord(*codec, *resolution, *bitRate, *frameRate)
	if err != nil {
		log.Printf("Failed to start recording: %v", err)
		return
	}

	go session.ServePushFrames()
}
