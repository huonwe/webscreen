package webservice

import (
	"log"

	"github.com/gin-gonic/gin"
)

func (wm *WebMaster) handleDeviceStreamingStatus(ctx *gin.Context) {
	device_uuid := ctx.Query("device_uuid")
	isStreaming, remoteConfig := wm.Nexus.IsAlreadyStreaming(device_uuid)
	if isStreaming {
		log.Printf("Device %s is streaming with config: %+v", device_uuid, remoteConfig)
		ctx.JSON(200, gin.H{
			"streaming": true,
			"config":    remoteConfig,
		})
	} else {
		ctx.JSON(200, gin.H{
			"streaming": false,
		})
	}
}
