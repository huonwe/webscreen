package webservice

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	appauth "webscreen/auth"
)

func (wm *WebMaster) handleSetupUI(c *gin.Context) {
	c.FileFromFS("setup.html", http.FS(wm.staticFS))
}

func (wm *WebMaster) handleSetup(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式无效"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入管理员用户名"})
		return
	}
	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密码长度不能少于 8 位"})
		return
	}

	if err := wm.authService.SetupAdmin(req.Username, req.Password); err != nil {
		if errors.Is(err, appauth.ErrAlreadyInitialized) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "管理员账户创建失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "初始化成功"})
}
