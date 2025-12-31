package webservice

import (
	"log"
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

// /:device_type/:device_id/:device_ip/:device_port/ws
func (wm *WebMaster) handleScreenWS(c *gin.Context) {
	// Implement WebSocket handling for screen here
	// Parse URL parameters
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Failed to upgrade to websocket:", err)
		return
	}
	// deviceType := c.Param("device_type")
	// deviceID := c.Param("device_id")
	// deviceIP := c.Param("device_ip")
	// devicePort := c.Param("device_port")
	config := sagent.AgentConfig{}
	err = conn.ReadJSON(&config)
	if err != nil {
		log.Println("Failed to read connection options:", err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "webrtc_init"})
		conn.Close()
		return
	}
	// if config.DeviceType != deviceType || config.DeviceID != deviceID || config.DeviceIP != deviceIP || config.DevicePort != devicePort {
	// 	log.Println("Connection options do not match URL parameters")
	// 	// conn.Close()
	// 	// return
	// }
	// Create a unique session ID
	sessionID := config.DeviceType + "_" + config.DeviceID + "_" + config.DeviceIP + "_" + config.DevicePort
	if _, exists := wm.ScreenSessions[sessionID]; exists {
		wm.removeScreenSession(sessionID)
	}
	log.Printf("New WebSocket connection for session: %s", sessionID)
	session := wm.ScreenSessions[sessionID]
	session.WSConn = conn
	agent, err := sagent.NewAgent(config)
	if err != nil {
		log.Println("Failed to create agent:", err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "webrtc_init"})
		conn.Close()
		return
	}
	session.Agent = agent
	wm.ScreenSessions[sessionID] = session
	finalSDP := agent.CreateWebRTCConnection(string(config.SDP))
	// log.Println("Final SDP generated", finalSDP)
	if finalSDP == "" {
		log.Println("Failed to create WebRTC connection")
		conn.WriteJSON(map[string]any{"status": "error", "message": "Failed to create WebRTC connection", "stage": "webrtc_init"})
		conn.Close()
		return
	}
	conn.WriteJSON(map[string]any{"status": "ok", "sdp": finalSDP, "stage": "webrtc_init"})
	// bitrateInt, err := strconv.Atoi(config.DriverConfig["video_bit_rate"])
	// if err != nil {
	// 	bitrateInt = 8000000 // default to 8Mbps
	// }
	// if bitrateInt > 0 {
	// 	finalSDP = sagent.SetSDPBandwidth(finalSDP, bitrateInt)
	// }
	// finalSDP = webrtcHelper.SetSDPBandwidth(finalSDP, 20_000_000)
	// conn.WriteMessage(websocket.TextMessage, []byte(finalSDP))
	err = agent.InitDriver()
	if err != nil {
		log.Println("Failed to initialize driver:", err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "webrtc_init"})
		conn.Close()
		return
	}
	capabilities := agent.Capabilities()
	log.Printf("Driver Capabilities: %+v", capabilities)
	media_meta := agent.GetMediaMeta()
	conn.WriteJSON(map[string]interface{}{"status": "ok", "capabilities": capabilities, "media_meta": media_meta, "stage": "webrtc_metainfo"})
	go wm.listenScreenWS(conn, agent, sessionID)
	go wm.listenEventFeedback(agent, conn)

	agent.StartStreaming()
}

func (wm *WebMaster) listenScreenWS(wsConn *websocket.Conn, agent *sagent.Agent, sessionID string) {
	for {
		mType, msg, err := wsConn.ReadMessage()
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}
		switch mType {
		case websocket.BinaryMessage:
			// log.Println("Received binary message")
			err := agent.SendEvent(msg)
			if err != nil {
				log.Println("Failed to send event:", err)
			}
		case websocket.TextMessage:
			log.Printf("Received text message: %s", string(msg))
		default:
			log.Printf("Received unsupported message type: %d", mType)
		}
	}
	wsConn.Close()
	agent.Close()
	wm.removeScreenSession(sessionID)
}

func (wm *WebMaster) listenEventFeedback(agent *sagent.Agent, wsConn *websocket.Conn) {
	agent.EventFeedback(func(msg []byte) bool {
		err := wsConn.WriteMessage(websocket.BinaryMessage, msg)
		if err != nil {
			log.Println("Failed to send event feedback via WebSocket:", err)
			return false
		}
		return true
	})
}

func (wm *WebMaster) removeScreenSession(sessionID string) {
	log.Printf("Removing screen session: %s", sessionID)
	if session, exists := wm.ScreenSessions[sessionID]; exists {
		if session.WSConn != nil {
			session.WSConn.Close()
		}
		if session.Agent != nil {
			session.Agent.Close()
		}
	}
	delete(wm.ScreenSessions, sessionID)
}
