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
)

// // 配置参数
// const (
// 	DisplayNum = 99
// 	// Resolution = "1920x1080"
// 	BitDepth = 24 // 24位色深
// )

func main() {
	tcpPort := flag.String("tcp_port", "27184", "server listen port")
	resolution := flag.String("resolution", "1920x1080", "virtual display resolution")
	bitRate := flag.String("bitrate", "8M", "streaming bitrate in Mbps")
	frameRate := flag.String("framerate", "60", "frame rate for capturing")
	codec := flag.String("codec", "h264", "video codec: h264 or hevc")
	flag.Parse()
	log.Printf("Starting Xvfb capturer with resolution %s, bitrate %s, framerate %s, codec %s\n", *resolution, *bitRate, *frameRate, *codec)

	// 启动 Xvfb 虚拟显示器
	_width, _height := strings.Split(*resolution, "x")[0], strings.Split(*resolution, "x")[1]
	// 定义清理函数：用于杀死 Xvfb 进程
	width, err := strconv.Atoi(_width)
	height, err := strconv.Atoi(_height)
	session, err := NewXvfbSession(*tcpPort, width, height, 99, 24)
	if err != nil {
		log.Printf("无法启动 Xvfb: %v", err)
		return
	}
	// 2. 监听 Ctrl+C，确保退出时执行清理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan // 阻塞直到收到信号
		session.CleanUp()
		os.Exit(0)
	}()

	// 确保 main 函数正常结束时也清理
	defer session.CleanUp()

	log.Println("连接成功，开始 FFmpeg 推流...")

	// FFmpeg 抓取该虚拟屏幕
	session.StartFFmpeg(*codec, *resolution, *bitRate, *frameRate)

	// 数据发送循环
	scanner := bufio.NewScanner(session.ffmpegOutput)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)
	scanner.Split(splitNALU)

	header := make([]byte, 12)

	var currentPts uint64 = uint64(time.Now().UnixNano() / 1e3)
	var frameStarted bool = false

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

		pts := currentPts
		binary.BigEndian.PutUint64(header[0:8], pts)
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

	// 循环结束后（通常是 FFmpeg 退出或网络断开），由 defer cleanup() 负责收尾
}
