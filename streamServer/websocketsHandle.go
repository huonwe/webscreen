package streamServer

import (
	"log"
	"net/http"

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
			// log.Println("msg type:", p[0])
			switch p[0] {
			case WS_TYPE_TOUCH: // Touch Event
				event, err := sm.createScrcpyTouchEvent(p)
				// log.Printf("Touch Event: %+v\n", event)
				if err != nil {
					log.Println("Failed to unmarshal touch event:", err)
					continue
				}
				sm.DataAdapter.SendTouchEvent(event)

			case WS_TYPE_KEY: // Key Event
				event, err := sm.createScrcpyKeyEvent(p)
				// log.Printf("Key Event: %+v\n", event)
				if err != nil {
					log.Println("Failed to unmarshal key event:", err)
					continue
				}
				sm.DataAdapter.SendKeyEvent(event)
				// log.Println("key event sent")
			case WS_TYPE_ROTATE: // Rotate Device
				log.Println("Rotate Device command received")
				sm.DataAdapter.RotateDevice()
			case WS_TYPE_SCROLL: // Scroll Event
				event, err := sm.createScrcpyScrollEvent(p)
				// log.Printf("Scroll Event: %+v\n", event)
				if err != nil {
					log.Println("Failed to unmarshal scroll event:", err)
					continue
				}
				sm.DataAdapter.SendScrollEvent(event)
			default:
				log.Println("Unknown control message type:", p[0])
			}
		default:
			log.Println("Unsupported WebSocket message type:", messageType)
		}

	}
}

// func convertWSMsg(msg []byte) {

// }
