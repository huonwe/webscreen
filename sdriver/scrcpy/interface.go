package scrcpy

import (
	"fmt"
	"log"
	"time"
	"webscreen/sdriver"
)

func (sd *ScrcpyDriver) GetReceivers() (<-chan sdriver.AVBox, <-chan sdriver.AVBox, <-chan sdriver.Event) {
	return sd.VideoChan, sd.AudioChan, sd.ControlChan
}

func (sd *ScrcpyDriver) Start() {
	log.Println("ScrcpyDriver: Start called")
	go sd.convertVideoFrame()
	go sd.convertAudioFrame()
	go sd.transferControlMsg()
}

func (sd *ScrcpyDriver) Pause() {
	// sd.stopVideoReader()
}

func (sd *ScrcpyDriver) SendEvent(event sdriver.Event) error {
	switch e := event.(type) {
	case *sdriver.TouchEvent:
		sd.SendTouchEvent(e)
	case *sdriver.KeyEvent:
		sd.SendKeyEvent(e)
	case *sdriver.ScrollEvent:
		sd.SendScrollEvent(e)
	case *sdriver.RotateEvent:
		sd.RotateDevice()
	case *sdriver.GetClipboardEvent:
		sd.SendGetClipboardEvent(e)
	case *sdriver.SetClipboardEvent:
		sd.SendSetClipboardEvent(e)
	case *sdriver.UHIDCreateEvent:
		sd.SendUHIDCreateEvent(e)
	case *sdriver.UHIDInputEvent:
		sd.SendUHIDInputEvent(e)
	case *sdriver.UHIDDestroyEvent:
		sd.SendUHIDDestroyEvent(e)
	case *sdriver.IDRReqEvent:
		sd.KeyFrameRequest()
	default:
		log.Printf("ScrcpyDriver: Unhandled event type: %T", event)
	}

	return nil
}

func (sd *ScrcpyDriver) RequestIDR(firstFrame bool) {
	if len(sd.LastSPS) == 0 && len(sd.LastPPS) == 0 && len(sd.LastVPS) == 0 && len(sd.LastIDR) == 0 {
		sd.KeyFrameRequest()
		return
	}

	if firstFrame {
		sd.keyFrameMutex.RLock()
		cachedSPS := createCopy(sd.LastSPS)
		cachedPPS := createCopy(sd.LastPPS)
		cachedVPS := createCopy(sd.LastVPS)
		cachedIDR := createCopy(sd.LastIDR)
		sd.keyFrameMutex.RUnlock()
		log.Println("⚡ Sending cached IDR and parameter sets for first frame")
		if len(sd.LastVPS) > 0 {
			sd.VideoChan <- sdriver.AVBox{Data: cachedVPS, PTS: sd.LastPTS, IsKeyFrame: false, IsConfig: true}
		}
		sd.VideoChan <- sdriver.AVBox{Data: cachedSPS, PTS: sd.LastPTS, IsKeyFrame: false, IsConfig: true}
		sd.VideoChan <- sdriver.AVBox{Data: cachedPPS, PTS: sd.LastPTS, IsKeyFrame: false, IsConfig: true}
		sd.VideoChan <- sdriver.AVBox{Data: cachedIDR, PTS: sd.LastPTS, IsKeyFrame: true, IsConfig: false}
	}

	if time.Since(sd.lastIDRRequestTime) > 2*time.Second {
		log.Println("⚡ Requesting new KeyFrame from device...")
		sd.KeyFrameRequest()
	}
}

func (sd *ScrcpyDriver) Capabilities() sdriver.DriverCaps {
	return sd.capabilities
}

func (sd *ScrcpyDriver) MediaMeta() sdriver.MediaMeta {
	return sd.mediaMeta
}

func (sd *ScrcpyDriver) Stop() {
	if sd.videoConn != nil {
		sd.videoConn.Close()
	}
	if sd.audioConn != nil {
		sd.audioConn.Close()
	}
	if sd.controlConn != nil {
		sd.controlConn.Close()
	}
	sd.adbClient.ReverseRemove(fmt.Sprintf("localabstract:scrcpy_%s", sd.scid))
	sd.adbClient.Stop()
	sd.cancel()
}
