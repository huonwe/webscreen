package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"scrmux/consts"
	db "scrmux/database"
	"scrmux/webservice"
	"strconv"
	"syscall"

	"github.com/gin-gonic/gin"
)

//go:embed public
var publicFS embed.FS

func main() {
	gin.SetMode(gin.ReleaseMode)

	var DATA_DIR string
	var TLS bool
	var DOMAIN string
	var IP string
	var PORT int
	flag.StringVar(&DATA_DIR, "D", getEnvString("DATA_DIR", "./data"), "directory to store data")
	envTLS, envTLSSet := getEnvBool("TLS")
	flag.BoolVar(&TLS, "tls", envTLS, "enable TLS")
	flag.StringVar(&DOMAIN, "domain", getEnvString("DOMAIN", ""), "Domain name for the server")
	flag.StringVar(&IP, "ip", getEnvString("IP", ""), "IP address to bind the server to")
	flag.IntVar(&PORT, "port", getEnvInt("PORT", 0), "server port")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pub, _ := fs.Sub(publicFS, "public")
	if err := os.MkdirAll(DATA_DIR, 0755); err != nil {
		log.Printf("create data dir failed: %v\n", err)
		return
	}
	config, err := webservice.LoadConfig(filepath.Join(DATA_DIR, "config.yml"))
	if err != nil {
		log.Fatalf("Failed to load config.yml: %v", err)
	}
	consts.DATA_DIR = DATA_DIR

	log.Printf("Loading database from: %s", filepath.Join(DATA_DIR, "scrmux.db"))
	MyDataBase, err := db.InitDB(filepath.Join(DATA_DIR, "scrmux.db"))
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	consts.DB = MyDataBase

	if IP != "" {
		config.IP_Listen = IP
	}
	if PORT > 0 {
		config.Port_Listen = PORT
	}
	if DOMAIN != "" {
		config.Domain = DOMAIN
	}
	// 判断命令行或环境变量是否显式指定了 TLS
	tlsExplicitlySet := envTLSSet
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "tls" {
			tlsExplicitlySet = true
		}
	})
	if tlsExplicitlySet {
		config.TLS = TLS
	}

	webMaster := webservice.New(config, pub)
	log.Printf("Data directory: %s", DATA_DIR)
	log.Printf("Starting web service on %s:%d", config.IP_Listen, config.Port_Listen)
	log.Printf("TLS enabled: %v", config.TLS)
	log.Printf("Trusted IPs: %v", config.TrustedIPs)
	go webMaster.Serve(config.IP_Listen, config.Port_Listen, config.TLS, config.Domain)

	<-ctx.Done()
	log.Println("Closing...")
	webMaster.Close()
	_ = consts.DB.Close()

}

func getEnvBool(key string) (val bool, ok bool) {
	if raw, exists := os.LookupEnv(key); exists && raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			return parsed, true
		}
	}
	return false, false
}
func getEnvString(key string, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return fallback
}
