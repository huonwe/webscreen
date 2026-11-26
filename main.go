package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"webscreen/consts"
	db "webscreen/database"
	"webscreen/webservice"

	"github.com/gin-gonic/gin"
)

//go:embed public
var publicFS embed.FS

var DATA_DIR string

func main() {
	// host := flag.String("host", "0.0.0.0", "host to bind the server to")
	// port := flag.String("port", "8081", "server port")
	flag.StringVar(&DATA_DIR, "D", "./data", "directory to store data")
	flag.Parse()
	consts.SetDataDir(DATA_DIR)
	// pin should be 6 digits and only digits
	gin.SetMode(gin.ReleaseMode)
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
	// config.EnableAndroidDiscover = true
	MyDataBase, err := db.InitDB(DATA_DIR + "/webscreen.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	webMaster := webservice.New(config, pub, MyDataBase)
	log.Printf("Starting web service on %s:%d", config.Host, config.Port)
	log.Printf("Data directory: %s", DATA_DIR)
	log.Printf("Trusted IPs: %v", config.TrustedIPs)
	go webMaster.Serve(config.Host, config.Port)

	<-ctx.Done()
	log.Println("Gracefully closing")
	webMaster.Close()

}
