package webservice

import (
	"log"
	"strconv"
	"uuid"
	"webscreen/consts"
	db "webscreen/database"
	"webscreen/nexus"
	"webscreen/sdriver/gamestream"
	linuxDriver "webscreen/sdriver/linux"
	rockchipDriver "webscreen/sdriver/rockchip-hdmirx"
	"webscreen/sdriver/scrcpy"
	"webscreen/webservice/android"

	"github.com/gin-gonic/gin"
)

func (wm *WebMaster) handleListDevices(c *gin.Context) {
	wm.devicesDiscoveredMu.RLock()
	defer wm.devicesDiscoveredMu.RUnlock()

	// var devicesInfo []db.DeviceInfo

	devices, err := android.GetDevices()
	if err != nil {
		log.Printf("[device-list] Android device discovery failed: %v", err)
	} else {
		for _, d := range devices {
			// devicesInfo = append(devicesInfo, DeviceInfo{
			// 	Type:     d.GetType(),
			// 	DeviceID: d.GetDeviceID(),
			// 	IP:       d.GetIP(),
			// 	Port:     d.GetPort(),
			// 	Status:   d.GetStatus(),
			// })
			wm.registerDevice(db.DeviceInfo{
				Type:     d.GetType(),
				DeviceID: d.GetDeviceID(),
				UUID:     d.UUID,
				IP:       d.GetIP(),
				Port:     d.GetPort(),
				Status:   d.GetStatus(),
			})

		}
	}

	linuxDevices, err := linuxDriver.GetDevices()
	if err != nil {
		log.Printf("[device-list] Linux device discovery failed: %v", err)
	} else {
		for _, d := range linuxDevices {
			// devicesInfo = append(devicesInfo, DeviceInfo{
			// 	Type:     d.GetType(),
			// 	DeviceID: d.GetDeviceID(),
			// 	IP:       d.GetIP(),
			// 	Port:     d.GetPort(),
			// 	Status:   d.GetStatus(),
			// })
			wm.registerDevice(db.DeviceInfo{
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
			// devicesInfo = append(devicesInfo, DeviceInfo{
			// 	Type:     d.GetType(),
			// 	DeviceID: d.GetDeviceID(),
			// 	IP:       d.GetIP(),
			// 	Port:     d.GetPort(),
			// 	Status:   d.GetStatus(),
			// })
			wm.registerDevice(db.DeviceInfo{
				Type:     d.GetType(),
				DeviceID: d.GetDeviceID(),
				IP:       d.GetIP(),
				Port:     d.GetPort(),
				Status:   d.GetStatus(),
			})
		}
	}

	gamestreamDevices, err := gamestream.GetDevices()
	if err != nil {
		log.Printf("[device-list] GameStream discovery failed: %v", err)
	} else {
		for _, d := range gamestreamDevices {
			wm.registerDevice(db.DeviceInfo{
				Type:     d.GetType(),
				DeviceID: d.GetDeviceID(),
				IP:       d.GetIP(),
				Port:     d.GetPort(),
				Status:   d.GetStatus(),
			})
		}
	}

	all_devices, err := db.QueryAllDevices(wm.DB)
	if err != nil {
		log.Printf("Error querying all devices: %v", err)
		c.JSON(500, gin.H{"error": "Internal server error"})
		return
	}
	log.Printf("Queried all devices: %+v", all_devices)

	c.JSON(200, gin.H{"devices": all_devices})
}

// HandleConnectDevice 处理连接设备的请求
// POST /api/device/connect
func (wm *WebMaster) handleConnectDevice(c *gin.Context) {
	var req struct {
		DeviceType string `json:"device_type"`
		IP         string `json:"device_ip"`
		Port       string `json:"device_port"`
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
	case consts.DEVICE_TYPE_ANDROID:
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
		IP         string `json:"device_ip"`
		Port       string `json:"device_port"`
		Code       string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	addr := req.IP + ":" + req.Port
	switch req.DeviceType {
	case consts.DEVICE_TYPE_ANDROID:
		if err := android.PairDevice(addr, req.Code); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	case consts.DEVICE_TYPE_GAMESTREAM:
		port, _ := strconv.Atoi(req.Port)
		if err := gamestream.PairDevice(req.IP, port, req.Code); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	default:
		c.JSON(400, gin.H{"error": "Unsupported device type"})
		return
	}

	c.JSON(200, gin.H{"status": "paired"})
}

func (wm *WebMaster) handleGameStreamDiscover(c *gin.Context) {
	devices, err := gamestream.DiscoverDevices()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"devices": devices})
}

func (wm *WebMaster) handleGameStreamPair(c *gin.Context) {
	var req struct {
		DeviceID string `json:"device_id"`
		IP       string `json:"device_ip"`
		Port     int    `json:"device_port"`
		PIN      string `json:"pin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	if req.Port <= 0 {
		req.Port = 47989
	}
	if err := gamestream.PairDevice(req.IP, req.Port, req.PIN); err != nil {
		log.Printf("[gamestream] pair %s:%d failed: %v", req.IP, req.Port, err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	deviceID := req.DeviceID
	if deviceID == "" {
		deviceID = req.IP
	}
	wm.registerDevice(db.DeviceInfo{
		Type:     consts.DEVICE_TYPE_GAMESTREAM,
		DeviceID: deviceID,
		IP:       req.IP,
		Port:     req.Port,
		Status:   "active",
	})
	c.JSON(200, gin.H{"status": "paired"})
}

func (wm *WebMaster) handleDeviceConfigDescription(c *gin.Context) {
	dtype := c.Query("device_type")
	id := c.Query("device_id")
	switch dtype {
	case consts.DEVICE_TYPE_ANDROID:
		desc := scrcpy.ConfigDescription(id)
		c.JSON(200, desc)
	case consts.DEVICE_TYPE_LINUX:
		desc := linuxDriver.ConfigDescription()
		c.JSON(200, desc)
	case consts.DEVICE_TYPE_ROCKCHIP_HDMIRX:
		desc := rockchipDriver.ConfigDescription()
		c.JSON(200, desc)
	case consts.DEVICE_TYPE_GAMESTREAM:
		desc := gamestream.ConfigDescription()
		c.JSON(200, desc)
	case "universal":
		desc := nexus.ConfigDescription()
		c.JSON(200, desc)
	default:
		c.JSON(400, gin.H{"error": "Unsupported device type"})
		return
	}

}

func (wm *WebMaster) registerDevice(device db.DeviceInfo) {
	// register the device to the sqlite if not exists, otherwise update the device info
	exists, err := db.DeviceExists(wm.DB, device.Type, device.IP, device.Port)
	if err != nil {
		log.Printf("Error checking device existence: %v", err)
		return
	}
	if !exists {
		// Insert the device
		deviceUUID := device.UUID
		if deviceUUID == "" {
			deviceUUID = uuid.New().String()
		}
		db.InsertDevice(wm.DB, deviceUUID, device.Type, device.DeviceID, device.DeviceID, device.IP, device.Port, device.Status, "")
	}

}

func (wm *WebMaster) handleUpdateDeviceDisplayName(c *gin.Context) {
	var req struct {
		DeviceUUID string `json:"device_uuid"`
		Name       string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	if err := db.UpdateDeviceDisplayNameByUUID(wm.DB, req.DeviceUUID, req.Name); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "updated"})
}
