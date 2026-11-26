package webservice

import "github.com/gin-gonic/gin"

func (wm *WebMaster) handleDeviceStreamingStatus(ctx *gin.Context) {
	uuid := ctx.Query("uuid")
	isStreaming, remoteConfig := wm.Nexus.IsAlreadyStreaming(uuid)
	if isStreaming {
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
