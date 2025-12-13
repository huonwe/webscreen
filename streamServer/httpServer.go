package streamServer

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pion/rtcp"
)

func HandleStatic(c *gin.Context) {
	http.ServeFile(c.Writer, c.Request, "./public"+c.Request.URL.Path)
}
func HTTPServer(sm *StreamManager, port string) {
	r := gin.Default()

	// 提供静态文件服务
	r.Static("/static", "./public/static")
	r.GET("/", HandleStatic)
	r.GET("/index.html", HandleStatic)

	// 处理 SDP 协商
	r.POST("/sdp", sm.HandleSDP)
	r.OPTIONS("/sdp", sm.HandleSDP) // 处理预检请求

	// WebSocket 路由
	r.GET("/ws", sm.HandleWebSocket)

	// 启动服务器
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
				if now.Sub(lastRTCPTime) < time.Millisecond*500 {
					continue
				}
				lastRTCPTime = now
				log.Println("收到 PLI 请求 (Keyframe Request)")
				sm.DataAdapter.RequestKeyFrame()
			}
		}
	}
}
