package webservice

import (
	"io/fs"
	"log"
	"maps"
	"net/http"
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
	staticFS            fs.FS
}

func New(config WebMasterConfig, staticFS fs.FS) *WebMaster {
	wm := &WebMaster{
		ScreenSessions:    make(map[string]ScreenSession),
		config:            config,
		devicesDiscovered: make(map[string]Device),
		staticFS:          staticFS,
	}
	wm.setRouter()
	return wm
}

func Default(staticFS fs.FS) *WebMaster {
	wm := &WebMaster{
		ScreenSessions: make(map[string]ScreenSession),
		config: WebMasterConfig{
			EnableAndroidDiscover: true,
		},
		devicesDiscovered: make(map[string]Device),
		staticFS:          staticFS,
	}
	wm.setRouter()
	return wm
}

func (wm *WebMaster) setRouter() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	subFS, _ := fs.Sub(wm.staticFS, "static")
	r.StaticFS("/static", http.FS(subFS))
	// r.GET("/", func(ctx *gin.Context) {
	// 	ctx.Redirect(302, "/console")
	// })
	screen := r.Group("/screen")
	{
		screen.GET("/:id", func(ctx *gin.Context) {
			ctx.FileFromFS("screen.html", http.FS(wm.staticFS))
		})
		screen.GET("/ws", wm.handleScreenWS)
	}
	r.GET("/console", func(c *gin.Context) {
		c.FileFromFS("console.html", http.FS(wm.staticFS))
	})
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
	for k, v := range maps.All(wm.ScreenSessions) {
		log.Printf("closing session %v", k)
		v.Close()
	}
}
