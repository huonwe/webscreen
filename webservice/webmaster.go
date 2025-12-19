package webservice

import (
	"log"
	"maps"
	"sync"

	"github.com/gin-gonic/gin"
)

type WebMasterConfig struct {
	EnableAndroidDiscover bool
}

type WebMaster struct {
	// WSConns []*websocket.Conn

	ScreenSessions map[string]ScreenSession

	config              WebMasterConfig
	router              *gin.Engine
	devicesConnected    map[string]Device
	devicesDiscovered   map[string]Device
	devicesDiscoveredMu sync.RWMutex
	pauseDiscovery      bool
}

func New(config WebMasterConfig) *WebMaster {
	wm := &WebMaster{
		ScreenSessions:    make(map[string]ScreenSession),
		config:            config,
		devicesDiscovered: make(map[string]Device),
	}
	wm.setRouter()
	return wm
}

func Default() *WebMaster {
	wm := &WebMaster{
		ScreenSessions: make(map[string]ScreenSession),
		config: WebMasterConfig{
			EnableAndroidDiscover: true,
		},
		devicesDiscovered: make(map[string]Device),
	}
	wm.setRouter()
	return wm
}

func (wm *WebMaster) setRouter() {
	r := gin.Default()
	r.Static("/static", "./public/static")
	// r.Static("/", "./public")
	// redirect to /console temporarily
	r.GET("/", func(ctx *gin.Context) {
		ctx.Redirect(302, "/console")
	})
	screen := r.Group("/screen")
	{
		screen.GET("/:device_type/:device_id/:device_ip/:device_port", handleScreen)
		screen.GET("/:device_type/:device_id/:device_ip/:device_port/ws", wm.handleScreenWS)
	}
	r.GET("/console", handleConsole)
	api := r.Group("/api")
	{
		api.GET("/device/list", wm.handleListDevices)
		api.POST("/device/connect", wm.handleConnectDevice)
		api.POST("/device/pair", wm.handlePairDevice)
		api.POST("/device/discovery", wm.handleListDevicesDiscoveried)
		api.POST("/device/select", wm.handleSelectDevice)
	}

	wm.router = r
}

func (wm *WebMaster) Serve() {
	if wm.config.EnableAndroidDiscover {
		go wm.AndroidDevicesDiscovery()
	}
	wm.router.Run(":8081")
}

func (wm *WebMaster) Close() {
	for k,v := range maps.All(wm.ScreenSessions){
		log.Printf("closing session %v", k)
		v.Close()
	}
}
