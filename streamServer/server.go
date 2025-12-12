package streamServer

import (
	"net/http"

	"github.com/gin-gonic/gin"
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

	// 启动服务器
	err := r.Run(":" + port)
	if err != nil {
		panic("启动 HTTP 服务器失败: " + err.Error())
	}
}
