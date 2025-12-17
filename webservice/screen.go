package webservice

import (
	agent "webcpy/streamAgent"

	"github.com/gorilla/websocket"
)

type ScreenSession struct {
	WSConn *websocket.Conn
	Agent  agent.SAgent
}
