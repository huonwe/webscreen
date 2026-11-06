package webservice

import (
	"log"
	linuxDriver "webscreen/sdriver/linux"
	rockchipDriver "webscreen/sdriver/rockchip-hdmirx"
	"webscreen/sdriver/scrcpy"
	sagent "webscreen/streamAgent"
	"webscreen/webservice/android"
	"webscreen/webservice/linux"

	"github.com/gin-gonic/gin"
)

func (wm *WebMaster) handleListDevices(c *gin.Context) {
	wm.devicesDiscoveredMu.RLock()
	defer wm.devicesDiscoveredMu.RUnlock()

	var devicesInfo []DeviceInfo
	devices, err := android.GetDevices()
	if err != nil {
		log.Printf("[device-list] Android device discovery failed: %v", err)
	} else {
		for _, d := range devices {
			devicesInfo = append(devicesInfo, DeviceInfo{
				Type:     d.GetType(),
				DeviceID: d.GetDeviceID(),
				IP:       d.GetIP(),
				Port:     d.GetPort(),
				Status:   d.GetStatus(),
			})
		}
	}

	linuxDevices, err := linux.GetDevices()
	if err != nil {
		log.Printf("[device-list] Linux device discovery failed: %v", err)
	} else {
		for _, d := range linuxDevices {
			devicesInfo = append(devicesInfo, DeviceInfo{
				Type:     d.GetType(),
				DeviceID: d.GetDeviceID(),
				IP:       d.GetIP(),
				Port:     d.GetPort(),
				Status:   d.GetStatus(),
			})
		}
	}

	rockchipDevices, err := rockchipDriver.GetDevices()
	if err != nil {
		log.Printf("[device-list] Rockchip HDMI RX discovery failed: %v", err)
	} else {
		for _, d := range rockchipDevices {
			devicesInfo = append(devicesInfo, DeviceInfo{
				Type:     d.GetType(),
				DeviceID: d.GetDeviceID(),
				IP:       d.GetIP(),
				Port:     d.GetPort(),
				Status:   d.GetStatus(),
			})
		}
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
	case sagent.DEVICE_TYPE_ANDROID:
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
	case sagent.DEVICE_TYPE_ANDROID:
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

func (wm *WebMaster) handleDeviceConfigDescription(c *gin.Context) {
	dtype := c.Query("device_type")
	id := c.Query("device_id")
	switch dtype {
	case sagent.DEVICE_TYPE_ANDROID:
		desc := scrcpy.ConfigDescription(id)
		c.JSON(200, desc)
	case sagent.DEVICE_TYPE_LINUX:
		desc := linuxDriver.ConfigDescription()
		c.JSON(200, desc)
	case sagent.DEVICE_TYPE_ROCKCHIP_HDMIRX:
		desc := rockchipDriver.ConfigDescription()
		c.JSON(200, desc)
	case "universal":
		desc := sagent.ConfigDescription()
		c.JSON(200, desc)
	default:
		c.JSON(400, gin.H{"error": "Unsupported device type"})
		return
	}

}
