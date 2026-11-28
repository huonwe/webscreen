package webservice

import (
	"log"
	"net/http"
	"scrmux/consts"
	"scrmux/nexus"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

func (wm *WebMaster) handleScreenWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Failed to upgrade to websocket:", err)
		return
	}
	defer conn.Close()
	userID, exists := c.Get("userid")
	if !exists {
		conn.WriteJSON(map[string]any{"status": "error", "message": "Unauthorized", "stage": "connection_view_session"})
		return
	}
	connectionConfig := nexus.ConnectionConfig{}
	err = conn.ReadJSON(&connectionConfig)
	if err != nil {
		log.Println("Failed to read connection options:", err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "connection_view_session"})
		return
	}
	log.Println("Received connection config: ", connectionConfig.Protocol, connectionConfig.RemoteUUID, connectionConfig.VideoCodec, connectionConfig.AudioCodec)
	remoteUUID := connectionConfig.RemoteUUID
	if remoteUUID == "" {
		conn.WriteJSON(map[string]any{"status": "error", "message": "Remote UUID is required", "stage": "connection_view_session"})
		return
	}
	if connectionConfig.Protocol == consts.PROTOCOL_TYPE_WEBSOCKET {
		connectionConfig.WSConn = conn
	}

	remoteConfig := nexus.RemoteConfig{}
	err = conn.ReadJSON(&remoteConfig)
	if err != nil {
		log.Println("Failed to read remote config:", err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "connection_view_session"})
		return
	}
	log.Printf("Received remote config: %+v", remoteConfig)

	viewSession, err := wm.Nexus.NewViewSession(userID.(string), connectionConfig)
	if err != nil {
		log.Println("Failed to create view session:", err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "connection_view_session"})
		return
	}
	if connectionConfig.Protocol == consts.PROTOCOL_TYPE_WEBSOCKET {
		viewSession.WSConn = conn
		log.Println("set WSConnection to viewSession")
		conn.WriteJSON(map[string]any{"status": "ok", "useWSTransport": true, "message": "WebSocket connection established", "stage": "connection_view_session"})
	}
	if connectionConfig.Protocol == consts.PROTOCOL_TYPE_WEBRTC {
		finalSDP := viewSession.FinalSDP2Nexus
		if finalSDP == "" {
			log.Println("Failed to create view session: Final SDP is empty")
			conn.WriteJSON(map[string]any{"status": "error", "message": "Failed to create view session: Final SDP is empty", "stage": "connection_view_session"})
			return
		}
		conn.WriteJSON(map[string]any{"status": "ok", "sdp": finalSDP, "stage": "connection_view_session"})
	}

	// Connect to the remote device using the provided remote UUID
	err = wm.Nexus.ConnectToRemote(remoteUUID, remoteConfig)
	if err != nil {
		log.Println("Failed to connect to remote device:", err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "connection_remote"})
		return
	}
	capabilities := wm.Nexus.GetRemoteCapabilities(remoteUUID)
	media_meta := wm.Nexus.GetRemoteMediaMeta(remoteUUID)
	log.Printf("Remote Capabilities: %+v", capabilities)
	log.Printf("Remote Media Meta: %+v", media_meta)
	conn.WriteJSON(map[string]any{"status": "ok", "capabilities": capabilities, "media_meta": media_meta, "stage": "metainfo_remote"})
	err = wm.Nexus.BindViewSessionToRemote(viewSession.SessionID, remoteUUID)
	if err != nil {
		log.Println("Failed to bind view session to remote:", err)
		conn.WriteJSON(map[string]any{"status": "error", "message": err.Error(), "stage": "connection_remote"})
		wm.Nexus.CloseViewSession(viewSession.SessionID)
		return
	}
	<-viewSession.Done()
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
