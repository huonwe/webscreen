package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"webscreen/webservice"
)

//go:embed public
var publicFS embed.FS

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pub, _ := fs.Sub(publicFS, "public")
	webMaster := webservice.Default(pub)
	go webMaster.Serve()

	<-ctx.Done()
	log.Println("Gracefully closing")
	webMaster.Close()

}
