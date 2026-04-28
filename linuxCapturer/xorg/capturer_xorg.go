package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	lc "webscreen/linuxCapturer"
)

func main() {
	tcpPort := flag.String("tcp_port", "27184", "server listen port")
	resolution := flag.String("resolution", "1920x1080", "virtual display resolution")
	bitRate := flag.String("bitrate", "8M", "streaming bitrate in Mbps")
	frameRate := flag.String("framerate", "60", "frame rate for capturing")
	codec := flag.String("codec", "h264", "video codec: h264 or hevc")
	xorgDriver := flag.String("xorg_driver", "modesetting", "Xorg driver: auto, nvidia, modesetting, dummy")
	flag.Parse()
	log.Printf("Starting X11 capturer with resolution %s, bitrate %s, framerate %s, codec %s, driver %s\n", *resolution, *bitRate, *frameRate, *codec, *xorgDriver)

	parts := strings.Split(*resolution, "x")
	if len(parts) != 2 {
		log.Printf("Invalid resolution: %s", *resolution)
		return
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Printf("Invalid width: %v", err)
		return
	}
	height, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Printf("Invalid height: %v", err)
		return
	}

	session, err := NewX11Session(*tcpPort, width, height, 99, 24, *xorgDriver)
	if err != nil {
		log.Printf("无法启动原生 X11 虚拟显示器: %v", err)
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		session.CleanUp()
		os.Exit(0)
	}()

	defer session.CleanUp()

	log.Println("连接成功，开始 FFmpeg 推流...")

	if err := session.StartFFmpeg(*codec, *resolution, *bitRate, *frameRate); err != nil {
		log.Printf("推流启动失败: %v", err)
		return
	}

	scanner := bufio.NewScanner(session.ffmpegOutput)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)
	scanner.Split(lc.SplitNALU)

	header := make([]byte, 12)
	var currentPts uint64 = uint64(time.Now().UnixNano() / 1e3)
	var frameStarted bool

	for scanner.Scan() {
		nalData := scanner.Bytes()
		if len(nalData) == 0 {
			continue
		}

		isVCL := false
		isFirstSlice := false
		isNonVCLFrameStart := false

		if *codec == "h265" {
			nalTypeHevc := (nalData[0] >> 1) & 0x3F
			if nalTypeHevc == 35 || nalTypeHevc == 32 || nalTypeHevc == 33 || nalTypeHevc == 34 || nalTypeHevc == 39 || nalTypeHevc == 40 {
				isNonVCLFrameStart = true
			} else if nalTypeHevc <= 31 {
				isVCL = true
				if len(nalData) > 2 && (nalData[2]&0x80) != 0 {
					isFirstSlice = true
				}
			}
		} else {
			nalTypeH264 := nalData[0] & 0x1F
			if nalTypeH264 == 9 || nalTypeH264 == 6 || nalTypeH264 == 7 || nalTypeH264 == 8 {
				isNonVCLFrameStart = true
			} else if nalTypeH264 >= 1 && nalTypeH264 <= 5 {
				isVCL = true
				if len(nalData) > 1 && (nalData[1]&0x80) != 0 {
					isFirstSlice = true
				}
			}
		}

		if isNonVCLFrameStart {
			if !frameStarted {
				currentPts = uint64(time.Now().UnixNano() / 1e3)
				frameStarted = true
			}
		} else if isVCL {
			if isFirstSlice && !frameStarted {
				currentPts = uint64(time.Now().UnixNano() / 1e3)
			}
			frameStarted = false
		}

		binary.BigEndian.PutUint64(header[0:8], currentPts)
		binary.BigEndian.PutUint32(header[8:12], uint32(len(nalData)))

		if _, err := session.Conn.Write(header); err != nil {
			log.Println("网络发送错误:", err)
			break
		}
		if _, err := session.Conn.Write(nalData); err != nil {
			log.Println("网络发送错误:", err)
			break
		}
	}
}
