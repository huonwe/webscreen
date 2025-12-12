package main

import (
	"log"
	"net"
	"webcpy/adb"
	"webcpy/httpserver"

	"github.com/pion/webrtc/v4"
)

// 配置部分
const (
	ScrcpyVersion = "3.3.3" // 必须与 jar 包完全一致
	LocalPort     = "6000"
	// 请确保此路径下有 scrcpy-server-v3.3.3.jar
	ServerLocalPath  = "./scrcpy-server-v3.3.3"
	ServerRemotePath = "/data/local/tmp/scrcpy-server-dev"
)

func main() {
	var err error
	// 1. 推送 Server
	adbClient := adb.NewClient("")
	defer adbClient.Stop()
	err = adbClient.Push(ServerLocalPath, ServerRemotePath)
	if err != nil {
		log.Fatalf("设置 推送scrcpy-server失败: %v", err)
	}
	err = adbClient.Reverse("localabstract:scrcpy", "tcp:"+LocalPort)
	defer adbClient.ReverseRemove("localabstract:scrcpy")
	if err != nil {
		log.Fatalf("设置 Reverse 隧道失败: %v", err)
	}

	// 3. 本地监听
	listener, err := net.Listen("tcp", ":"+LocalPort)
	if err != nil {
		log.Fatalf("监听端口失败: %v", err)
	}
	defer listener.Close()

	// 4. 启动 Android 端 Server
	adbClient.StartScrcpyServer()

	const StreamID = "android_live_stream"

	// 创建视频轨
	videoTrack, _ := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
		"video-track-id",
		StreamID, // <--- 关键点
	)

	// 创建音频轨
	audioTrack, _ := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, // 假设音频是 Opus
		"audio-track-id",
		StreamID, // <--- 必须和视频的一样！
	)

	streamManager := &httpserver.StreamManager{}
	streamManager.UpdateTracks(videoTrack, audioTrack)

	select {}

	// 初始化 WebRTC
	// videoTrack, err = webrtc.NewTrackLocalStaticSample(
	// 	webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264},
	// 	"video", "pion",
	// )
	// if err != nil {
	// 	panic(err)
	// }

	// go startHTTPServer()

	// // 5. 等待连接
	// log.Println(">>> 等待设备连接...")
	// conn, err := listener.Accept()
	// if err != nil {
	// 	log.Fatalf("Accept 失败: %v", err)
	// }
	// log.Println(">>> 设备已连接！开始处理流...")

	// handleStream(conn)
}

// func handleStream(conn net.Conn) {
// 	defer conn.Close()

// 	// 缓冲区：分配大一点，避免频繁扩容
// 	// 4MB足够容纳大多数 I 帧
// 	payloadBuf := make([]byte, 4*1024*1024)
// 	headerBuf := make([]byte, 12)

// 	// --- 阶段 1: 读取 Codec Metadata (12 bytes) ---
// 	// 文档：On the video socket, 12 bytes: codec id, width, height
// 	if _, err := io.ReadFull(conn, headerBuf); err != nil {
// 		log.Printf("读取 Codec Meta 失败: %v", err)
// 		return
// 	}

// 	codecId := string(headerBuf[0:4])
// 	width := binary.BigEndian.Uint32(headerBuf[4:8])
// 	height := binary.BigEndian.Uint32(headerBuf[8:12])
// 	log.Printf(">>> 握手成功: Codec=%s, 分辨率=%dx%d", codecId, width, height)

// 	if codecId != "h264" {
// 		log.Printf("警告: 收到非 h264 流 (%s)，WebRTC 可能无法播放", codecId)
// 	}

// 	// --- 阶段 2: 循环读取 Frame ---
// 	var lastPts int64 = 0

// 	for {
// 		// A. 读取 Frame Header (12 bytes)
// 		// 文档：config packet flag(u1), key frame flag(u1), PTS(u62), packet size(u32)
// 		if _, err := io.ReadFull(conn, headerBuf); err != nil {
// 			if err == io.EOF {
// 				log.Println(">>> 设备断开连接")
// 			} else {
// 				log.Printf("读取帧头失败: %v", err)
// 			}
// 			return
// 		}

// 		// 解析 PTS 和 Flags
// 		ptsAndFlags := binary.BigEndian.Uint64(headerBuf[0:8])
// 		packetSize := binary.BigEndian.Uint32(headerBuf[8:12])

// 		// 提取 PTS (低62位)
// 		pts := int64(ptsAndFlags & 0x3FFFFFFFFFFFFFFF)

// 		// 提取关键帧标记 (第62位)
// 		// isKeyFrame := (ptsAndFlags & 0x4000000000000000) != 0

// 		// B. 校验并读取 Payload
// 		if packetSize > uint32(len(payloadBuf)) {
// 			log.Printf("警告: 帧过大 (%d bytes), 扩容缓冲区...", packetSize)
// 			payloadBuf = make([]byte, packetSize+1024*1024)
// 		}

// 		if _, err := io.ReadFull(conn, payloadBuf[:packetSize]); err != nil {
// 			log.Printf("读取 Payload 失败: %v", err)
// 			return
// 		}

// 		// C. 计算 Duration 并发送给 WebRTC
// 		var duration time.Duration
// 		if lastPts == 0 {
// 			duration = time.Millisecond * 16 // 默认第一帧间隔
// 		} else {
// 			// Scrcpy 的 PTS 单位是微秒 (us)
// 			duration = time.Duration(pts-lastPts) * time.Microsecond
// 		}
// 		lastPts = pts

// 		// 简单的防抖动：如果计算出的间隔太离谱（比如由暂停引起），重置为标准值
// 		if duration > time.Second || duration < 0 {
// 			duration = time.Millisecond * 16
// 		}

// 		// D. 写入 Pion Track
// 		if err := videoTrack.WriteSample(media.Sample{
// 			Data:     payloadBuf[:packetSize],
// 			Duration: duration,
// 		}); err != nil {
// 			// 只有当没有浏览器连接 PeerConnection 时这里会报错 "io: read/write on closed pipe"
// 			// 这是正常的，忽略即可
// 		}
// 	}
// }
