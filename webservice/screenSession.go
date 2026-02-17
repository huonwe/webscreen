package webservice

import (
	agent "webscreen/streamAgent"

	"github.com/gorilla/websocket"
)

type ScreenSession struct {
	SessionID string
	WSConn    []*websocket.Conn
	// WebRTCConn *agent.WebRTCConnection
	Agent *agent.Agent
}

func (sc *ScreenSession) Close() {
	sc.Agent.Close()
	for _, conn := range sc.WSConn {
		conn.Close()
	}
}
