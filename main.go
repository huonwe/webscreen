package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"webscreen/utils"
	"webscreen/webservice"
)

//go:embed public
var publicFS embed.FS

func main() {
	host := flag.String("host", "0.0.0.0", "host to bind the server to")
	port := flag.String("port", "8081", "server port")
	pin := flag.String("pin", "", "initial PIN for web access")
	// local-root runs webscreen on the Android device itself (needs root):
	// no adb is used, the scrcpy server is started locally with app_process.
	localRoot := flag.Bool("local-root", false, "run on the device itself (root, no adb): the local Android device is streamed")
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

	if *pin == "" {
		log.Println("Warning: Since v1.3.6, the default PIN is empty, which means no PIN is required to access the web interface.")
	}

	utils.SetLocalRootMode(*localRoot)
	if utils.IsLocalRootMode() {
		log.Println("On-device mode enabled: adb is not used, the local Android device will be streamed")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pub, _ := fs.Sub(publicFS, "public")
	webMaster := webservice.Default(pub)
	webMaster.SetPIN(*pin)

	go webMaster.Serve(*host, *port)

	<-ctx.Done()
	log.Println("Gracefully closing")
	webMaster.Close()

}
