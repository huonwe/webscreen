package webservice

import (
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
	sagent "webscreen/streamAgent"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"

	"webscreen/webservice/android"
)

// sdpOffersH265 reports whether the browser's SDP offer contains H.265/HEVC.
// Only browsers that advertise it in the offer can decode it (Safari does,
// Chrome/Chromium do not).
func sdpOffersH265(sdp string) bool {
	lower := strings.ToLower(sdp)
	return strings.Contains(lower, "h265") || strings.Contains(lower, "hevc")
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// /:id/ws
func (wm *WebMaster) handleScreenWS(c *gin.Context) {
	// Implement WebSocket handling for screen here
	// Parse URL parameters
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Failed to upgrade to websocket:", err)
		return
	}
	defer conn.Close()

	config := sagent.AgentConfig{}
	err = conn.ReadJSON(&config)
	if err != nil {
		log.Println("Failed to read connection options:", err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "webrtc_init"})
		return
	}
	log.Printf("Received connection driver config: %+v", config.DriverConfig)

	// H.265 (HEVC) over WebRTC only works with a browser that offers it in its
	// SDP. Asking for it from a browser that does not (Chrome/Chromium) leaves
	// the media section impossible to populate ("RTPSender created with no
	// codecs") and the whole connection fails, so use H.264 for this session
	// instead of failing and telling the user nothing.
	codecFallback := false
	if strings.EqualFold(config.DriverConfig["video_codec"], "h265") && !sdpOffersH265(config.SDP) {
		log.Printf("[webscreen] browser SDP does not offer H.265 (HEVC), using H.264 for this session")
		if config.DriverConfig == nil {
			config.DriverConfig = map[string]string{}
		}
		config.DriverConfig["video_codec"] = "h264"
		codecFallback = true
	}

	// Create a unique ID for one abstract device
	deviceIdentifier := config.DeviceType + "_" + config.DeviceID + "_" + config.DeviceIP + "_" + config.DevicePort
	// Hash the identifier to ensure it's a valid filename and not too long
	// h := sha256.New()
	// h.Write([]byte(deviceIdentifier))
	// deviceIdentifier = fmt.Sprintf("%x", h.Sum(nil))

	finalSDP, receiptNo, err := wm.WebRTCManager.NewSubscriber(deviceIdentifier, config.SDP, config)
	if err != nil {
		log.Println("Failed to handle new connection:", err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "webrtc_init"})
		return
	}
	if finalSDP == "" {
		log.Println("Failed to create WebRTC connection")
		conn.WriteJSON(map[string]any{"status": "error", "message": "Failed to create WebRTC connection", "stage": "webrtc_init"})
		return
	}
	log.Println("deviceIdentifier:", deviceIdentifier, "receiptNo:", receiptNo)
	conn.WriteJSON(map[string]any{"status": "ok", "sdp": finalSDP, "stage": "webrtc_init"})

	sub, exists := wm.WebRTCManager.GetSubscriber(deviceIdentifier, receiptNo)
	if !exists {
		log.Printf("Failed to get subscriber for device %s", deviceIdentifier)
		conn.WriteJSON(map[string]any{"status": "error", "message": "Failed to get subscriber", "stage": "webrtc_init"})
		return
	}
Loop:
	for {
		switch sub.PeerConnection.ConnectionState() {
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			log.Printf("Peer connection for device %s is in state %s, closing WebSocket", deviceIdentifier, sub.PeerConnection.ConnectionState())
			conn.WriteJSON(map[string]any{"status": "error", "message": "Peer connection failed or closed", "stage": "webrtc_connection"})
			return
		case webrtc.PeerConnectionStateConnected:
			break Loop
		default:
			time.Sleep(1 * time.Second)
		}
	}

	err = wm.WebRTCManager.Start(deviceIdentifier, receiptNo, config)
	if err != nil {
		log.Printf("Failed to start WebRTC session for device %s: %v", deviceIdentifier, err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "webrtc_start"})
		return
	}
	agent, exists := wm.WebRTCManager.GetAgent(deviceIdentifier)
	if !exists {
		log.Printf("Failed to get agent for device %s", deviceIdentifier)
		conn.WriteJSON(map[string]any{"status": "error", "message": "Failed to get agent", "stage": "webrtc_metainfo"})
		return
	}
	if codecFallback {
		agent.Notify("This browser does not support H.265 (HEVC); streaming with H.264 instead. (H.265 needs a browser that offers HEVC, e.g. Safari.)")
	}
	// 记录投屏状态，供设备管理界面显示
	android.GetManager().MarkStreaming(config.DeviceID, true)
	defer android.GetManager().MarkStreaming(config.DeviceID, false)
	capabilities := agent.Capabilities()
	log.Printf("Driver Capabilities: %+v", capabilities)
	media_meta := agent.GetMediaMeta()
	conn.WriteJSON(map[string]interface{}{"status": "ok", "capabilities": capabilities, "media_meta": media_meta, "stage": "webrtc_metainfo"})

	// Keep consuming the signaling connection after SDP negotiation. Gorilla
	// processes close, ping and pong control frames while reading, and the
	// browser also uses this socket for explicit key-frame requests.
	const (
		requestKeyFrameType = byte(0x63)
		pongWait            = 50 * time.Second
		pingPeriod          = 20 * time.Second
	)

	conn.SetReadLimit(64 * 1024)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	pingDone := make(chan struct{})
	defer close(pingDone)
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
					return
				}
			case <-pingDone:
				return
			}
		}
	}()

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket signaling channel closed for device %s: %v", deviceIdentifier, err)
			return
		}
		if messageType == websocket.BinaryMessage && len(payload) > 0 && payload[0] == requestKeyFrameType {
			agent.PLIRequest()
		}
	}
}

// func (wm *WebMaster) removeScreenSession(deviceIdentifier string) {
// 	log.Printf("Removing screen session: %s", deviceIdentifier)
// 	if session, exists := wm.ScreenSessions[deviceIdentifier]; exists {
// 		if session.WSConn != nil {
// 			session.WSConn.Close()
// 		}
// 	}
// 	delete(wm.ScreenSessions, deviceIdentifier)
// }
