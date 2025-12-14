package main

import (
	//"fmt"

	"log"
	"webcpy/scrcpy"
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
	defer streamManager.Close()
	go streamServer.HTTPServer(streamManager, HTTPPort)

	dataAdapter.ShowDeviceInfo()
	dataAdapter.StartConvertVideoFrame()
	dataAdapter.StartConvertAudioFrame()

	// videoChan := dataAdapter.VideoChan
	// for frame := range videoChan {
	// 	streamManager.WriteVideoSample(&frame)
	// 	streamManager.DataAdapter.VideoPayloadPool.Put(frame.Data)
	// }
	go func() {
		videoChan := dataAdapter.VideoChan
		for frame := range videoChan {
			streamManager.WriteVideoSample(&frame)
			// streamManager.DataAdapter.VideoPayloadPool.Put(frame.Data)
		}
	}()
	go func() {
		audioChan := dataAdapter.AudioChan
		for frame := range audioChan {
			streamManager.WriteAudioSample(&frame)
			// streamManager.DataAdapter.AudioPayloadPool.Put(frame.Data)
		}
	}()
	select {}
}
