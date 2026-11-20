package webservice

import (
	"database/sql"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"webscreen/auth"
	appauth "webscreen/auth"
	"webscreen/nexus"
	"webscreen/utils"
	"webscreen/webservice/android"

	"github.com/gin-gonic/gin"
)

type WebMasterConfig struct {
	EnableAndroidDiscover bool
	PIN                   string   `yaml:"pin"`
	JWTSecret             string   `yaml:"jwt_secret"`
	TrustedIPs            []string `yaml:"trusted_ips"`
}

type WebMaster struct {
	// WSConns []*websocket.Conn

	Nexus         nexus.Nexus
	WebRTCManager *WebRTCManager
	// ScreenSessions map[string]ScreenSession

	pin                  string
	trustedIPs           []string
	UnlockAttemptRecords map[string]UnlockAttemptRecord
	jwtSecret            []byte
	DB                   *sql.DB
	authService          *appauth.Service
	authMu               sync.Mutex

	config              WebMasterConfig
	router              *gin.Engine
	devicesConnected    map[string]Device
	devicesDiscovered   map[string]Device
	devicesDiscoveredMu sync.RWMutex
	pauseDiscovery      bool
	staticFS            fs.FS
}

func New(config WebMasterConfig, staticFS fs.FS, database *sql.DB) *WebMaster {
	wm := &WebMaster{
		// ScreenSessions:       make(map[string]ScreenSession),
		config:               config,
		devicesDiscovered:    make(map[string]Device),
		staticFS:             staticFS,
		UnlockAttemptRecords: make(map[string]UnlockAttemptRecord),
		WebRTCManager:        NewWebRTCManager(),
		pin:                  config.PIN,
		trustedIPs:           append([]string(nil), config.TrustedIPs...),
		authService:          auth.NewService(database, config.JWTSecret),
		DB:                   database,
		Nexus:                nexus.NewNexus(database),
	}
	secret := config.JWTSecret
	if secret == "" {
		secret = time.Now().String()
	}
	wm.jwtSecret = []byte(secret)
	return wm
}

// func Default(staticFS fs.FS) *WebMaster {
// 	wm := New(WebMasterConfig{
// 		EnableAndroidDiscover: true,
// 	}, staticFS)
// 	return wm
// }

func (wm *WebMaster) setRouter() {
	// gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 前端资源是 //go:embed 进二进制的：升级后浏览器若沿用缓存的旧 HTML/JS，
	// 会出现"按钮在但点了没反应"（旧页面没有新脚本标签）这类极难排查的现象。
	// 因此统一禁止缓存，保证升级后刷新即生效。
	r.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	})

	subFS, _ := fs.Sub(wm.staticFS, "static")
	r.StaticFS("/static", http.FS(subFS))

	r.GET("/unlock", func(ctx *gin.Context) {
		ctx.FileFromFS("unlock.html", http.FS(wm.staticFS))
	})
	r.POST("/api/unlock", wm.handleUnlock)

	r.GET("/", func(ctx *gin.Context) {
		ctx.Redirect(302, "/console")
	})
	r.GET("/login", wm.handleLoginUI)
	r.POST("/api/login", wm.handleLogin)
	r.GET("/setup", wm.handleSetupUI)
	r.POST("/api/setup", wm.handleSetup)
	r.Use(wm.HybridAuthMiddleware())
	screen := r.Group("/screen")
	{
		screen.GET("/:id", func(ctx *gin.Context) {
			ctx.FileFromFS("screen.html", http.FS(wm.staticFS))
		})
		screen.GET("/ws", wm.handleScreenWS)
		screen.GET("/wsn", wm.handleScreenWSN)
	}

	r.GET("/console", func(c *gin.Context) {
		c.FileFromFS("console.html", http.FS(wm.staticFS))
	})
	r.GET("/consoleN", func(c *gin.Context) {
		c.FileFromFS("consoleN.html", http.FS(wm.staticFS))
	})
	api := r.Group("/api")
	{
		api.GET("/auth/status", wm.handleAuthStatus)
		api.GET("/device/list", wm.handleListDevices)
		api.POST("/device/connect", wm.handleConnectDevice)
		api.POST("/device/pair", wm.handlePairDevice)
		api.GET("/device/configDescription", wm.handleDeviceConfigDescription)
		// api.GET("/generalConfigDescription", wm.handleGeneralConfigDescription)

		// api.POST("/device/discovery", wm.handleListDevicesDiscoveried)

		// 统一设备管理（USB / Wi-Fi）：设备列表、事件日志、命名与连接控制
		api.GET("/adb/devices", wm.handleADBDevices)
		api.GET("/adb/events", wm.handleADBEvents)
		api.POST("/adb/usb/rescan", wm.handleADBRescanUSB)
		api.POST("/adb/device/name", wm.handleADBSetName)
		api.POST("/adb/device/wifi", wm.handleADBAddWiFi)
		api.POST("/adb/device/remove", wm.handleADBRemove)
		api.POST("/adb/device/disconnect", wm.handleADBDisconnect)
		api.POST("/adb/device/reconnect", wm.handleADBReconnect)

		// 受控端背光（隐私保护：屏幕黑，但不锁屏，串流照常）
		api.POST("/screen/backlight", wm.handleBacklight)

		// auth := api.Group("/auth")
		// {
		// 	auth.GET("/status", authSrv.HandleStatus)
		// }
	}

	wm.router = r
}

func (wm *WebMaster) SetPIN(pin string) {
	// Set the PIN for web access
	// This is a placeholder implementation
	log.Printf("PIN set to: %s", pin)
	wm.pin = pin
}

func (wm *WebMaster) Serve(host, port string) {
	// if wm.config.EnableAndroidDiscover {
	// 	go wm.AndroidDevicesDiscovery()
	// }
	// 统一设备管理器：USB 热插拔、RSA 授权状态与无线重连都由它跟踪。
	// 本机 root 模式下没有 adb，不需要它。
	if !utils.IsLocalRootMode() {
		android.GetManager().Start()
	}
	wm.setRouter()
	err := wm.router.Run(host + ":" + port)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func (wm *WebMaster) Close() {
	wm.authMu.Lock()
	defer wm.authMu.Unlock()
	if wm.DB != nil {
		_ = wm.DB.Close()
		wm.DB = nil
		wm.authService = nil
	}
	// for k, v := range maps.All(wm.ScreenSessions) {
	// 	log.Printf("closing session %v", k)
	// 	v.Close()
	// }
}
