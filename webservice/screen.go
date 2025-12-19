package webservice

import (
	agent "webcpy/streamAgent"

	"github.com/gorilla/websocket"
)

type ScreenSession struct {
	WSConn *websocket.Conn
	Agent  *agent.Agent
}

func (sc *ScreenSession) Close() {
	sc.Agent.Close()
	sc.WSConn.Close()
}