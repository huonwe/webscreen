package streamServer

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
	"webcpy/scrcpy"

	"github.com/gin-gonic/gin"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

func HandleStatic(c *gin.Context) {
	http.ServeFile(c.Writer, c.Request, "./public"+c.Request.URL.Path)
}
func HTTPServer(sc *StreamController, port string) {
	r := gin.Default()

	// 提供静态文件服务
	r.Static("/static", "./public/static")
	r.GET("/", HandleStatic)
	r.GET("/index.html", HandleStatic)
	r.GET("/console.html", func(c *gin.Context) {
		http.ServeFile(c.Writer, c.Request, "./public/console.html")
	})

	// 处理 SDP 协商
	r.POST("/sdp", func(c *gin.Context) {
		sc.RLock()
		sm := sc.CurrentStreamManager
		sc.RUnlock()
		if sm == nil {
			c.String(503, "Stream not started")
			return
		}
		sm.HandleSDP(c)
	})
	r.OPTIONS("/sdp", func(c *gin.Context) {
		sc.RLock()
		sm := sc.CurrentStreamManager
		sc.RUnlock()
		if sm == nil {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Status(200)
			return
		}
		sm.HandleSDP(c)
	})

	// WebSocket 路由
	r.GET("/ws", func(c *gin.Context) {
		sc.RLock()
		sm := sc.CurrentStreamManager
		sc.RUnlock()
		if sm == nil {
			c.String(503, "Stream not started")
			return
		}
		sm.HandleWebSocket(c)
	})

	// ADB API Routes
	api := r.Group("/api")
	{
		api.GET("/devices", HandleListDevices)
		api.POST("/connect", HandleConnectDevice)
		api.POST("/pair", HandlePairDevice)
		api.POST("/start_stream", func(c *gin.Context) {
			HandleStartStream(c, sc)
		})
	}

	// 启动服务器
	gin.SetMode(gin.ReleaseMode)
	err := r.Run(":" + port)
	if err != nil {
		panic("启动 HTTP 服务器失败: " + err.Error())
	}
}

func (sm *StreamManager) HandleRTCP() {
	rtcpBuf := make([]byte, 1500)
	lastRTCPTime := time.Now()
	for {
		n, _, err := sm.rtpSenderVideo.Read(rtcpBuf)
		if err != nil {
			return
		}
		packets, err := rtcp.Unmarshal(rtcpBuf[:n])
		if err != nil {
			continue
		}
		for _, p := range packets {
			switch p.(type) {
			case *rtcp.PictureLossIndication:
				now := time.Now()
				if now.Sub(lastRTCPTime) < time.Second*2 {
					continue
				}
				lastRTCPTime = now
				// log.Println("收到 PLI 请求 (Keyframe Request)")
				sm.DataAdapter.RequestKeyFrame()
			}
		}
	}
}

func (sm *StreamManager) HandleSDP(c *gin.Context) {
	// 允许跨域 (方便调试)
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	if c.Request.Method == "OPTIONS" {
		return
	}

	// A. 读取浏览器发来的 Offer SDP
	body, _ := io.ReadAll(c.Request.Body)
	// log.Println("Browser Offer SDP:", string(body)) // <--- 添加这行
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  string(body),
	}

	// B. 创建 PeerConnection
	// 配置 ICE 服务器 (STUN)，用于穿透 NAT
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	// 创建 MediaEngine 并注册默认 Codec
	// 创建 MediaEngine
	m := &webrtc.MediaEngine{}

	// 1. 注册 Opus (音频)
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10;useinbandfec=1"},
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		log.Println("RegisterCodec Opus failed:", err)
	}

	// 2. 注册 H.264 (视频) - 即使我们想用 H.265，注册 H.264 也是个好习惯，作为 fallback
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:     webrtc.MimeTypeH264,
			ClockRate:    90000,
			Channels:     0,
			SDPFmtpLine:  "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
			RTCPFeedback: []webrtc.RTCPFeedback{{"goog-remb", ""}, {"ccm", "fir"}, {"nack", ""}, {"nack", "pli"}}, // 显式禁用 Generic NACK，只保留 PLI
		},
		PayloadType: 102,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		log.Println("RegisterCodec H264 failed:", err)
	}

	// 3. 注册 H.265 (视频)
	// 注意：这里我们尝试使用浏览器 Offer 中的 Payload Type (49) 或者一个安全的动态值 (104)
	// 为了避免冲突，我们先检查 sm.VideoTrack 的类型
	sm.RLock()
	videoMime := sm.VideoTrack.Codec().MimeType
	sm.RUnlock()
	if videoMime == webrtc.MimeTypeH265 {
		log.Printf("Registering %s Codec", videoMime)
		if err := m.RegisterCodec(webrtc.RTPCodecParameters{
			RTPCodecCapability: webrtc.RTPCodecCapability{
				MimeType:  videoMime,
				ClockRate: 90000,
				Channels:  0,
				// SDPFmtpLine: "level-id=180;profile-id=1;tier-flag=0;tx-mode=SRST",
				SDPFmtpLine:  "",                                                       // H.265 通常不需要复杂的 fmtp，或者可以留空让 Pion 处理
				RTCPFeedback: []webrtc.RTCPFeedback{{"goog-remb", ""}, {"ccm", "fir"}}, // 禁用 {"nack", ""} 以关闭重传
			},
			PayloadType: 104, // 使用 104，避开 Offer 中的 49/51 和 H.264 的 102
		}, webrtc.RTPCodecTypeVideo); err != nil {
			log.Println("RegisterCodec H265 failed:", err)
		}
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		log.Println("创建 PeerConnection 失败:", err)
		c.String(500, err.Error())
		return
	}

	// 加读锁，防止读取的时候 track 正在被替换
	sm.RLock()
	currentVideoTrack := sm.VideoTrack
	currentAudioTrack := sm.AudioTrack
	sm.RUnlock()
	if currentVideoTrack == nil || currentAudioTrack == nil {
		log.Println("视频或音频轨道尚未准备好")
		c.String(500, "视频或音频轨道尚未准备好")
		return
	}

	// C. 添加视频轨道 (Video Track)
	// 只要浏览器一连上来，就把我们从 Android 收到的 H.264 流推给它
	sm.rtpSenderVideo, err = peerConnection.AddTrack(currentVideoTrack)
	if err != nil {
		log.Println("添加 Track 失败:", err)
		c.String(500, err.Error())
		return
	}
	// 添加音频轨道 (Audio Track)
	sm.rtpSenderAudio, err = peerConnection.AddTrack(currentAudioTrack)
	if err != nil {
		log.Println("添加 Audio Track 失败:", err)
		c.String(500, err.Error())
		return
	}
	// 启动协程读取 RTCP 包 (如 PLI 请求关键帧)
	go sm.HandleRTCP()
	// D. 设置 Remote Description (浏览器发来的 Offer)
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		log.Println("设置 Remote Description 失败:", err)
		c.String(500, err.Error())
		return
	}

	// E. 创建 Answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Println("创建 Answer 失败:", err)
		c.String(500, err.Error())
		return
	}
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		log.Printf("连接状态改变: %s", s)
		if s == webrtc.PeerConnectionStateFailed || s == webrtc.PeerConnectionStateClosed {
			// 做一些清理工作，比如移除引用
			peerConnection.Close()
		}
	})

	// F. 设置 Local Description 并等待 ICE 收集完成
	// 这一步是为了生成一个包含所有网络路径信息的完整 SDP，
	// 这样我们就不需要写复杂的 Trickle ICE 逻辑了。
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	if err := peerConnection.SetLocalDescription(answer); err != nil {
		log.Println("设置 Local Description 失败:", err)
		c.String(500, err.Error())
		return
	}

	// 阻塞等待 ICE 收集完成 (通常几百毫秒)
	<-gatherComplete
	// log.Println("Final Server SDP:", peerConnection.LocalDescription().SDP)
	// params := sm.rtpSenderVideo.GetParameters()
	// if len(params.Encodings) > 0 {
	// 	log.Printf(">>> [DEBUG] Actual Outbound SSRC: %d", params.Encodings[0].SSRC)
	// }
	// // 1765815565
	// for _, codec := range params.Codecs {
	// 	// 找到 H.265 的那个 Codec
	// 	if strings.Contains(codec.MimeType, "H265") {
	// 		log.Printf(">>> [DEBUG] Actual Outbound PayloadType: %d | Mime: %s | Fmtp: %s",
	// 			codec.PayloadType,
	// 			codec.MimeType,
	// 			codec.SDPFmtpLine,
	// 		)
	// 	}
	// }
	// G. 将最终的 SDP Answer 返回给浏览器
	c.Writer.Header().Set("Content-Type", "application/sdp")

	// 手动修改 SDP 以突破浏览器默认带宽限制 (SDP Munging)
	finalSDP := setSDPBandwidth(peerConnection.LocalDescription().SDP, 20000) // 20 Mbps
	fmt.Fprint(c.Writer, finalSDP)

	// H. 请求关键帧 (IDR)
	// 连接建立后，立即请求一个新的关键帧，确保客户端能马上看到画面
	sm.webrtcConnected.Store(true)
	if sm.DataAdapter != nil {
		go func() {
			// 稍微延迟一下，确保连接完全建立
			time.Sleep(500 * time.Millisecond)
			sm.DataAdapter.RequestKeyFrame()
		}()
	}
}

type ConnectRequest struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
}

type PairRequest struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
	Code string `json:"code"`
}

type StartStreamRequest struct {
	Serial string `json:"serial"`
	// Options can be added here
}

func HandleStartStream(c *gin.Context, sc *StreamController) {
	var req StartStreamRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.Serial == "" {
		c.JSON(400, gin.H{"error": "Serial is required"})
		return
	}

	// Check if already running for this device
	sc.RLock()
	if sc.CurrentDataAdapter != nil && sc.CurrentDataAdapter.DeviceName == req.Serial { // Note: DeviceName might not be serial, check logic
		// Actually DataAdapter.DeviceName is read from device, might not match serial exactly or might be model name.
		// But we can check if we are already streaming.
		// For simplicity, let's just restart if requested, or maybe we should check.
		// Let's just restart to be safe and ensure clean state.
	}
	sc.RUnlock()

	// Use default options for now, or parse from request
	options := scrcpy.ScrcpyOptions{
		Version:      "3.3.3",
		SCID:         "01234567", // Should be random
		MaxFPS:       "60",
		VideoBitRate: "20000000",
		Control:      "true",
		Audio:        "true",
		VideoCodec:   "h264",
	}

	if err := sc.StartStream(req.Serial, options); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "started"})
}

func HandleListDevices(c *gin.Context) {
	devices, err := scrcpy.GetConnectedDevices()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"devices": devices})
}

func HandleConnectDevice(c *gin.Context) {
	var req ConnectRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	addr := req.IP
	if req.Port != "" {
		addr = addr + ":" + req.Port
	}
	if err := scrcpy.ConnectDevice(addr); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "connected"})
}

func HandlePairDevice(c *gin.Context) {
	var req PairRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	addr := req.IP + ":" + req.Port
	if err := scrcpy.PairDevice(addr, req.Code); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "paired"})
}
