package webservice

import (
	"webscreen/webservice/android"
	"webscreen/webservice/xvfb"

	"github.com/gin-gonic/gin"
)

func (wm *WebMaster) handleListDevices(c *gin.Context) {
	wm.devicesDiscoveredMu.RLock()
	defer wm.devicesDiscoveredMu.RUnlock()

	var devicesInfo []DeviceInfo
	devices, err := android.GetDevices()
	for _, d := range devices {
		devicesInfo = append(devicesInfo, DeviceInfo{
			Type:     d.GetType(),
			DeviceID: d.GetDeviceID(),
			IP:       d.GetIP(),
			Port:     d.GetPort(),
			Status:   d.GetStatus(),
		})
	}
	xvfbDevices, err := xvfb.GetDevices()
	for _, d := range xvfbDevices {
		devicesInfo = append(devicesInfo, DeviceInfo{
			Type:     d.GetType(),
			DeviceID: d.GetDeviceID(),
			IP:       d.GetIP(),
			Port:     d.GetPort(),
			Status:   d.GetStatus(),
		})
	}
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"devices": devicesInfo})
}

func (wm *WebMaster) handleListDevicesDiscoveried(c *gin.Context) {
	wm.devicesDiscoveredMu.RLock()
	defer wm.devicesDiscoveredMu.RUnlock()

	var devices []Device
	for _, v := range wm.devicesDiscovered {
		devices = append(devices, v)
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
