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
		// hasSentKeyFrame := false

		for frame := range videoChan {
			streamManager.WriteVideoSample(&frame)
			// 如果 WebRTC 未连接，则丢弃视频帧 (不写入 Track)
			// if !streamManager.IsConnected() {
			// 	hasSentKeyFrame = false // 重置关键帧等待状态
			// 	continue
			// }

			// // if dataAdapter.VideoMeta.CodecID == "h265" {
			// if !hasSentKeyFrame {
			// 	// H.265 NALU Header: F(1) + Type(6) + LayerId(6) + TID(3)
			// 	// Type 在第一个字节的中间 6 位
			// 	if len(frame.Data) > 0 {
			// 		nalType := (frame.Data[0] >> 1) & 0x3F
			// 		isKeyFrame := nalType >= 19 && nalType <= 21
			// 		isConfig := nalType >= 32 && nalType <= 34

			// 		if !isKeyFrame && !isConfig {
			// 			// 丢弃非关键帧，等待 IDR
			// 			continue
			// 		}

			// 		if isKeyFrame || isConfig {
			// 			log.Println(">>> 收到首个关键帧 (IDR)，开始推流！<<<")
			// 			hasSentKeyFrame = true
			// 		}
			// 	}
			// }
			// // }
			// streamManager.WriteVideoSample(&frame)
			// // streamManager.DataAdapter.VideoPayloadPool.Put(frame.Data)
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
