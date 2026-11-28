package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"scrmux/consts"
	db "scrmux/database"
	"scrmux/webservice"

	"github.com/gin-gonic/gin"
)

//go:embed public
var publicFS embed.FS

func main() {
	gin.SetMode(gin.ReleaseMode)

	var DATA_DIR string
	var TLS bool
	var TLS_DOMAIN string
	var IP string
	var PORT int
	flag.StringVar(&DATA_DIR, "D", "./data", "directory to store data")
	flag.StringVar(&DATA_DIR, "data", "./data", "directory to store data")
	flag.BoolVar(&TLS, "tls", true, "enable TLS")
	flag.StringVar(&TLS_DOMAIN, "tls_domain", "", "TLS domain name for the server")
	flag.StringVar(&IP, "ip", "", "IP address to bind the server to")
	flag.IntVar(&PORT, "port", 0, "server port")

	flag.Parse()
	consts.DATA_DIR = DATA_DIR

	MyDataBase, err := db.InitDB(DATA_DIR + "/scrmux.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	consts.DB = MyDataBase

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pub, _ := fs.Sub(publicFS, "public")
	if err := os.MkdirAll(DATA_DIR, 0755); err != nil {
		log.Printf("create data dir failed: %v\n", err)
		return
	}
	config, err := webservice.LoadConfig(DATA_DIR + "/config.yml")
	if err != nil {
		log.Fatalf("Failed to load config.yml: %v", err)
	}
	if IP != "" {
		config.IP_Listen = IP
	}
	if PORT != 0 {
		config.Port_Listen = PORT
	}
	if TLS_DOMAIN != "" {
		config.TLS_Domain = TLS_DOMAIN
	}

	webMaster := webservice.New(config, pub)
	log.Printf("Starting web service on %s:%d", config.IP_Listen, config.Port_Listen)
	log.Printf("Data directory: %s", DATA_DIR)
	log.Printf("Trusted IPs: %v", config.TrustedIPs)
	go webMaster.Serve(config.IP_Listen, config.Port_Listen, TLS, config.TLS_Domain)

	<-ctx.Done()
	log.Println("Closing...")
	webMaster.Close()
	_ = consts.DB.Close()

}
