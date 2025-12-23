package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"webscreen/webservice"
)

//go:embed public
var publicFS embed.FS

func main() {
	port := flag.String("port", "8079", "server port")
	pin := flag.String("pin", "123456", "initial PIN for web access")
	// pin should be 6 digits and only digits
	if len(*pin) != 6 {
		log.Fatal("PIN must be exactly 6 digits")
	}
	for _, ch := range *pin {
		if ch < '0' || ch > '9' {
			log.Fatal("PIN must contain only digits")
		}
	}

	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pub, _ := fs.Sub(publicFS, "public")
	webMaster := webservice.Default(pub)
	webMaster.SetPIN(*pin)

	go webMaster.Serve(*port)

	<-ctx.Done()
	log.Println("Gracefully closing")
	webMaster.Close()

}
