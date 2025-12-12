package main

import (
	"fmt"
	"log"
	"webcpy/scrcpy"
	"webcpy/streamServer"
)

// 配置部分
const (
	ScrcpyVersion = "3.3.3" // 必须与 jar 包完全一致
	LocalPort     = "6000"
	// 请确保此路径下有 scrcpy-server-v3.3.3.jar
	ServerLocalPath  = "./scrcpy-server-v3.3.3"
	ServerRemotePath = "/data/local/tmp/scrcpy-server-dev"

	HTTPPort = "8081"
)

func main() {
	var err error
	config := map[string]string{
		"device_serial":      "", // 默认设备
		"server_local_path":  ServerLocalPath,
		"server_remote_path": ServerRemotePath,
		"scrcpy_version":     ScrcpyVersion,
		"local_port":         LocalPort,
	}
	dataAdapter, err := scrcpy.NewDataAdapter(config)
	if err != nil {
		log.Fatalf("Failed to create DataAdapter: %v", err)
	}
	defer dataAdapter.Close()

	streamManager := streamServer.NewStreamManager(dataAdapter)
	go streamServer.HTTPServer(streamManager, HTTPPort)

	dataAdapter.ShowDeviceInfo()
	dataAdapter.StartConvertVideoFrame()

	videoChan := dataAdapter.VideoChan
	for frame := range videoChan {
		fmt.Printf("Frame Timestamp: %v, Size: %v nilType: %v\n", frame.Timestamp, len(frame.Data), frame.Data[4]&0x1F)
		streamManager.WriteVideoSample(&frame)
		streamManager.DataAdapter.VideoPayloadPool.Put(frame.Data)
	}
}
