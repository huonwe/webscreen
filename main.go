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
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pub, _ := fs.Sub(publicFS, "public")
	webMaster := webservice.Default(pub)

	go webMaster.Serve(*port)

	<-ctx.Done()
	log.Println("Gracefully closing")
	webMaster.Close()

}
