package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"webcpy/webservice"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	webMaster := webservice.Default()
	go webMaster.Serve()

	<-ctx.Done()
	log.Println("Gracefully closing")
	webMaster.Close()

}
