package webservice

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func handleConsole(c *gin.Context) {
	http.ServeFile(c.Writer, c.Request, "./public/console.html")
}
