package main

import (
	//"fmt"

	"webcpy/streamServer"
)

// 配置部分
const (
	ScrcpyVersion = "3.3.3" // 必须与 jar 包完全一致
	LocalPort     = "6000"
	// 请确保此路径下有 scrcpy-server-v3.3.3.jar
	ServerLocalPath  = "./scrcpy-server-v3.3.3-m"
	ServerRemotePath = "/data/local/tmp/scrcpy-server-dev"

	HTTPPort = "8081"
)

func main() {
	// Initialize StreamController
	streamController := streamServer.GlobalStreamController

	// Start HTTP Server
	// The server will handle starting the stream via /api/start_stream
	streamServer.HTTPServer(streamController, HTTPPort)
}
