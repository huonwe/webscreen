package webservice

import (
	"io/fs"
	"log"
	"maps"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type WebMasterConfig struct {
	EnableAndroidDiscover bool
}

type WebMaster struct {
	// WSConns []*websocket.Conn

	ScreenSessions map[string]ScreenSession

	pin                  string
	UnlockAttemptRecords map[string]UnlockAttemptRecord
	jwtSecret            []byte

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
		ScreenSessions:       make(map[string]ScreenSession),
		config:               config,
		devicesDiscovered:    make(map[string]Device),
		staticFS:             staticFS,
		UnlockAttemptRecords: make(map[string]UnlockAttemptRecord),
	}
	wm.jwtSecret = []byte(time.Now().String())
	wm.setRouter()
	return wm
}

func Default(staticFS fs.FS) *WebMaster {
	wm := &WebMaster{
		ScreenSessions: make(map[string]ScreenSession),
		config: WebMasterConfig{
			EnableAndroidDiscover: true,
		},
		devicesDiscovered:    make(map[string]Device),
		UnlockAttemptRecords: make(map[string]UnlockAttemptRecord),
		staticFS:             staticFS,
	}
	wm.jwtSecret = []byte(time.Now().String())
	wm.setRouter()
	return wm
}

func (wm *WebMaster) setRouter() {
	// gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	subFS, _ := fs.Sub(wm.staticFS, "static")
	r.StaticFS("/static", http.FS(subFS))

	r.GET("/unlock", func(ctx *gin.Context) {
		ctx.FileFromFS("unlock.html", http.FS(wm.staticFS))
	})
	r.POST("/api/unlock", wm.handleUnlock)

	r.GET("/", func(ctx *gin.Context) {
		ctx.Redirect(302, "/console")
	})
	r.Use(wm.HybridAuthMiddleware())
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
		// api.POST("/device/discovery", wm.handleListDevicesDiscoveried)
		// api.POST("/setPIN", wm.handleSetPIN)
	}

	wm.router = r
}

func (wm *WebMaster) SetPIN(pin string) {
	// Set the PIN for web access
	// This is a placeholder implementation
	log.Printf("PIN set to: %s", pin)
	wm.pin = pin
}

func (wm *WebMaster) Serve(port string) {
	// if wm.config.EnableAndroidDiscover {
	// 	go wm.AndroidDevicesDiscovery()
	// }
	wm.router.Run(":" + port)
}

func (wm *WebMaster) Close() {
	for k, v := range maps.All(wm.ScreenSessions) {
		log.Printf("closing session %v", k)
		v.Close()
	}
}
