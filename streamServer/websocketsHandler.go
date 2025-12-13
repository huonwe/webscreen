package streamServer

import (
	"log"
	"net/http"
	"webcpy/scrcpy"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

func (sm *StreamManager) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// log.Println("Failed to upgrade to websocket:", err)
		return
	}
	defer conn.Close()

	log.Println("Client connected via WebSocket")

	// buf := make([]byte, 1)
	for {
		// Read message from client
		messageType, p, err := conn.ReadMessage()
		// log.Println("receive ws message type: ", messageType)
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}
		switch messageType {
		case websocket.BinaryMessage:
			// 处理二进制消息 (控制命令)
			if len(p) < 1 {
				log.Println("Received empty binary message")
				continue
			}
			if p[0] == 0x01 { // Touch Event
				event := &scrcpy.TouchEvent{}
				err := event.UnmarshalBinary(p)
				if err != nil {
					log.Println("Failed to unmarshal touch event:", err)
					continue
				}
				sm.DataAdapter.SendTouchEvent(*event)
			} else if p[0] == 0x02 { // Key Event
				event := &scrcpy.KeyEvent{}
				err := event.UnmarshalBinary(p)
				if err != nil {
					log.Println("Failed to unmarshal key event:", err)
					continue
				}
				// sm.DataAdapter.SendKeyEvent(*event)
				log.Println("key event")
			}
		}

	}
}

// func convertWSMsg(msg []byte) {

// }
