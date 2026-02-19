package webservice

import (
	"log"
	"math/rand"
	"net/http"
	sagent "webscreen/streamAgent"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

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

	config := sagent.AgentConfig{}
	err = conn.ReadJSON(&config)
	if err != nil {
		log.Println("Failed to read connection options:", err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "webrtc_init"})
		conn.Close()
		return
	}

	// Create a unique ID for one abstract device
	deviceIdentifier := config.DeviceType + "_" + config.DeviceID + "_" + config.DeviceIP + "_" + config.DevicePort

	finalSDP, receiptNo, err := wm.WebRTCManager.NewSubscriber(deviceIdentifier, config.SDP, config)
	if err != nil {
		log.Println("Failed to handle new connection:", err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "webrtc_init"})
		conn.Close()
		return
	}
	if finalSDP == "" {
		log.Println("Failed to create WebRTC connection")
		conn.WriteJSON(map[string]any{"status": "error", "message": "Failed to create WebRTC connection", "stage": "webrtc_init"})
		conn.Close()
		return
	}
	log.Println("deviceIdentifier:", deviceIdentifier, "receiptNo:", receiptNo)
	conn.WriteJSON(map[string]any{"status": "ok", "sdp": finalSDP, "stage": "webrtc_init"})
	err = wm.WebRTCManager.Start(deviceIdentifier, receiptNo, config)
	if err != nil {
		log.Printf("Failed to start WebRTC session for device %s: %v", deviceIdentifier, err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "webrtc_start"})
		conn.Close()
		return
	}
	agent, exists := wm.WebRTCManager.GetAgent(deviceIdentifier)
	if !exists {
		log.Printf("Failed to get agent for device %s", deviceIdentifier)
		conn.WriteJSON(map[string]any{"status": "error", "message": "Failed to get agent", "stage": "webrtc_metainfo"})
		conn.Close()
		return
	}
	capabilities := agent.Capabilities()
	log.Printf("Driver Capabilities: %+v", capabilities)
	media_meta := agent.GetMediaMeta()
	conn.WriteJSON(map[string]interface{}{"status": "ok", "capabilities": capabilities, "media_meta": media_meta, "stage": "webrtc_metainfo"})
}

func (wm *WebMaster) removeScreenSession(deviceIdentifier string) {
	log.Printf("Removing screen session: %s", deviceIdentifier)
	if session, exists := wm.ScreenSessions[deviceIdentifier]; exists {
		if session.WSConn != nil {
			session.WSConn.Close()
		}
	}
	delete(wm.ScreenSessions, deviceIdentifier)
}
