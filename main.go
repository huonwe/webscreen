package main

import "webcpy/webservice"

func main() {
	webMaster := webservice.Default()
	webMaster.Serve()
}
