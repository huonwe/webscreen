package webservice

import (
	"net/http"
	"webcpy/webservice/android"

	"github.com/gin-gonic/gin"
)

func handleConsole(c *gin.Context) {
	http.ServeFile(c.Writer, c.Request, "./public/console.html")
}

func (wm *WebMaster) handleSelectDevice(c *gin.Context) {
	var req struct {
		DeviceType string `json:"device_type"`
		DeviceID   string `json:"device_id"`
		DeviceIP   string `json:"device_ip"`
		DevicePort int    `json:"device_port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	wm.devicesDiscoveredMu.Lock()
	wm.defaultDevice = Device{
		Type:     req.DeviceType,
		DeviceID: req.DeviceID,
		IP:       req.DeviceIP,
		Port:     req.DevicePort,
	}
	wm.devicesDiscoveredMu.Unlock()

	c.JSON(200, gin.H{"status": "default device set"})
}

func (wm *WebMaster) handleListDevices(c *gin.Context) {
	wm.devicesDiscoveredMu.RLock()
	defer wm.devicesDiscoveredMu.RUnlock()

	devices, err := android.GetDevices()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, devices)
}

func (wm *WebMaster) handleListDevicesDiscoveried(c *gin.Context) {
	wm.devicesDiscoveredMu.RLock()
	defer wm.devicesDiscoveredMu.RUnlock()

	devices := []Device{}
	for _, v := range wm.devicesDiscovered {
		devices = append(devices, Device{
			Type:     "android",
			DeviceID: v.DeviceID,
			IP:       v.IP,
			Port:     v.Port,
		})
	}

	c.JSON(200, devices)
}

// HandleConnectDevice 处理连接设备的请求
// POST /api/device/connect
func (wm *WebMaster) handleConnectDevice(c *gin.Context) {
	var req struct {
		DeviceType string `json:"device_type"`
		IP         string `json:"ip"`
		Port       string `json:"port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	addr := req.IP
	if req.Port != "" {
		addr = addr + ":" + req.Port
	}
	switch req.DeviceType {
	case DeviceTypeAndroid:
		if err := android.ConnectDevice(addr); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	default:
		c.JSON(400, gin.H{"error": "Unsupported device type"})
		return
	}

	c.JSON(200, gin.H{"status": "connected"})
}

func (wm *WebMaster) handlePairDevice(c *gin.Context) {
	var req struct {
		DeviceType string `json:"device_type"`
		IP         string `json:"ip"`
		Port       string `json:"port"`
		Code       string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	addr := req.IP + ":" + req.Port
	switch req.DeviceType {
	case DeviceTypeAndroid:
		if err := android.PairDevice(addr, req.Code); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	default:
		c.JSON(400, gin.H{"error": "Unsupported device type"})
		return
	}

	c.JSON(200, gin.H{"status": "paired"})
}
