package webservice

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"webscreen/webservice/android"
)

// 设备管理接口：USB 与 Wi-Fi 两种接入方式由同一个 ADB Manager 统一管理。
// 这些接口只在飞牛/主机模式下有意义（本机 root 模式没有 adb）。

type adbDeviceRequest struct {
	Serial  string `json:"serial"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

// GET /api/adb/devices
// 返回完整设备列表：连接类型（USB/WiFi）、状态机、自动名称与自定义名称。
func (wm *WebMaster) handleADBDevices(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"devices": android.GetManager().Snapshot()})
}

// GET /api/adb/events?limit=100
// 返回设备事件日志（检测到设备 / 等待授权 / 已连接 / 已断开 …）。
func (wm *WebMaster) handleADBEvents(c *gin.Context) {
	limit := 100
	if _, err := fmt.Sscanf(c.DefaultQuery("limit", "100"), "%d", &limit); err != nil || limit <= 0 {
		limit = 100
	}
	c.JSON(http.StatusOK, gin.H{"events": android.GetManager().Events(limit)})
}

// POST /api/adb/usb/rescan
// 刷新 USB 设备并重新请求调试授权（未授权设备会重新弹出手机的授权对话框）。
func (wm *WebMaster) handleADBRescanUSB(c *gin.Context) {
	mgr := android.GetManager()
	if err := mgr.RescanUSB(); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "devices": mgr.Snapshot()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "devices": mgr.Snapshot(), "events": mgr.Events(10)})
}

// POST /api/adb/device/name   {"serial": "...", "name": "我的平板"}
func (wm *WebMaster) handleADBSetName(c *gin.Context) {
	var req adbDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误：" + err.Error()})
		return
	}
	if err := android.GetManager().SetName(req.Serial, req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/adb/device/wifi   {"address": "192.168.31.120:5555"}
// 手动添加无线设备（需求第 18 节）。
func (wm *WebMaster) handleADBAddWiFi(c *gin.Context) {
	var req adbDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误：" + err.Error()})
		return
	}
	if err := android.GetManager().AddWiFi(req.Address); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "devices": android.GetManager().Snapshot()})
}

// POST /api/adb/device/remove   {"serial": "..."}
func (wm *WebMaster) handleADBRemove(c *gin.Context) {
	var req adbDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误：" + err.Error()})
		return
	}
	if err := android.GetManager().RemoveDevice(req.Serial); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/adb/device/disconnect   {"serial": "..."}
func (wm *WebMaster) handleADBDisconnect(c *gin.Context) {
	var req adbDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误：" + err.Error()})
		return
	}
	if err := android.GetManager().Disconnect(req.Serial); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/adb/device/reconnect   {"serial": "..."}
func (wm *WebMaster) handleADBReconnect(c *gin.Context) {
	var req adbDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误：" + err.Error()})
		return
	}
	if err := android.GetManager().Reconnect(req.Serial); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
