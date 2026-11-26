package webservice

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (wm *WebMaster) handleLoginUI(c *gin.Context) {
	hasAdmin, err := wm.authService.HasAdmin()
	if err != nil {
		c.String(http.StatusInternalServerError, "账户状态检查失败")
		return
	}
	if !hasAdmin {
		c.Redirect(http.StatusFound, "/setup")
		return
	}
	if wm.pin != 0 && wm.isTrustedIP(c.ClientIP()) {
		c.Redirect(http.StatusFound, "/unlock")
		return
	}
	c.FileFromFS("login.html", http.FS(wm.staticFS))
}

func (wm *WebMaster) handleLogin(c *gin.Context) {
	if wm.pin != 0 && wm.isTrustedIP(c.ClientIP()) {
		c.JSON(http.StatusForbidden, gin.H{"error": "受信任 IP 请使用 PIN 登录"})
		return
	}

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名"})
		return
	}
	if req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入访问密码"})
		return
	}

	token, err := wm.authService.Login(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	c.SetCookie("auth_token", token, 7*24*60*60, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"token":   token,
	})
}
