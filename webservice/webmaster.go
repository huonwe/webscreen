package webservice

import (
	"crypto/tls"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"scrmux/auth"
	appauth "scrmux/auth"
	db "scrmux/database"
	"scrmux/nexus"
	"scrmux/webservice/android"

	"github.com/gin-gonic/gin"
)

type WebMaster struct {
	// WSConns []*websocket.Conn

	Nexus *nexus.Nexus
	// ScreenSessions map[string]ScreenSession

	pin                  string
	trustedIPs           []string
	UnlockAttemptRecords map[string]UnlockAttemptRecord
	jwtSecret            []byte
	authService          *appauth.Service
	authMu               sync.Mutex

	config              WebMasterConfig
	router              *gin.Engine
	devicesConnected    map[string]db.DeviceInfo
	devicesDiscovered   map[string]db.DeviceInfo
	devicesDiscoveredMu sync.RWMutex
	pauseDiscovery      bool
	staticFS            fs.FS
}

func New(config WebMasterConfig, staticFS fs.FS) *WebMaster {
	wm := &WebMaster{
		// ScreenSessions:       make(map[string]ScreenSession),
		config:               config,
		devicesDiscovered:    make(map[string]db.DeviceInfo),
		staticFS:             staticFS,
		UnlockAttemptRecords: make(map[string]UnlockAttemptRecord),
		pin:                  config.PIN,
		trustedIPs:           append([]string(nil), config.TrustedIPs...),
		authService:          auth.NewService(config.JWTSecret),
		Nexus:                nexus.NewNexus(config.MTU),
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
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong over HTTPS"})
	})

	// 前端资源是 //go:embed 进二进制的：升级后浏览器若沿用缓存的旧 HTML/JS，
	// 会出现"按钮在但点了没反应"（旧页面没有新脚本标签）这类极难排查的现象。
	// 因此统一禁止缓存，保证升级后刷新即生效。
	// r.Use(func(c *gin.Context) {
	// 	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	// 	c.Header("Pragma", "no-cache")
	// 	c.Header("Expires", "0")
	// 	c.Next()
	// })

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
	}

	r.GET("/console", func(c *gin.Context) {
		c.FileFromFS("console.html", http.FS(wm.staticFS))
	})

	api := r.Group("/api")
	{
		api.GET("/auth/status", wm.handleAuthStatus)
		api.GET("/device/list", wm.handleListDevices)
		api.GET("/device/streaming/status", wm.handleDeviceStreamingStatus)
		api.POST("/device/connect", wm.handleConnectDevice)
		api.POST("/device/updateDisplayName", wm.handleUpdateDeviceDisplayName)
		api.POST("/device/pair", wm.handlePairDevice)
		api.GET("/device/configDescription", wm.handleDeviceConfigDescription)
		api.GET("/gamestream/discover", wm.handleGameStreamDiscover)
		api.POST("/gamestream/pair", wm.handleGameStreamPair)

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

	}

	wm.router = r
}

func (wm *WebMaster) Serve(ip string, port int, tls_on bool, tls_hostname string) {
	// 统一设备管理器：USB 热插拔、RSA 授权状态与无线重连都由它跟踪。
	android.GetManager().Start()
	wm.setRouter()
	if tls_on {
		cert, err := GenerateSelfSignedCert(tls_hostname)
		if err != nil {
			log.Fatalf("生成证书失败: %v", err)
		}
		srv := &http.Server{
			Addr:    net.JoinHostPort(ip, fmt.Sprintf("%d", port)),
			Handler: wm.router,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
			},
		}
		err = srv.ListenAndServeTLS("", "")
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
		return
	} else {
		log.Printf("Starting server without TLS on %s:%d", ip, port)
		err := http.ListenAndServe(net.JoinHostPort(ip, fmt.Sprintf("%d", port)), wm.router)
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}
}

func (wm *WebMaster) Close() {
	nexus := wm.Nexus
	if nexus != nil {
		_ = nexus.Close()
		wm.Nexus = nil
	}
}
