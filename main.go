package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"webscreen/db"
	"webscreen/webservice"
)

//go:embed public
var publicFS embed.FS

func main() {
	host := flag.String("host", "0.0.0.0", "host to bind the server to")
	port := flag.String("port", "8081", "server port")
	pin := flag.String("pin", "", "initial PIN for web access")

	flag.Parse()
	// pin should be 6 digits and only digits
	if *pin != "" {
		if len(*pin) != 6 {
			log.Fatal("PIN must be exactly 6 digits")
		}
		for _, ch := range *pin {
			if ch < '0' || ch > '9' {
				log.Fatal("PIN must contain only digits")
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pub, _ := fs.Sub(publicFS, "public")
	config, err := webservice.LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("Failed to load config.yml: %v", err)
	}
	// config.EnableAndroidDiscover = true
	MyDataBase, err := db.InitDB("webscreen.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	webMaster := webservice.New(config, pub, MyDataBase)
	if *pin != "" {
		webMaster.SetPIN(*pin)
	}

	go webMaster.Serve(*host, *port)

	<-ctx.Done()
	log.Println("Gracefully closing")
	webMaster.Close()

}
