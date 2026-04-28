package scrcpy

import (
	"log"
	"time"
	"webscreen/sdriver"
)

func (sd *ScrcpyDriver) GetReceivers() (<-chan sdriver.AVBox, <-chan sdriver.AVBox, chan sdriver.Event) {
	return sd.VideoChan, sd.AudioChan, sd.ControlChan
}

func (sd *ScrcpyDriver) Start() {
	log.Println("ScrcpyDriver: Start called")
	if sd.videoConn != nil {
		go sd.convertVideoFrame()
	}
	if sd.audioConn != nil {
		go sd.convertAudioFrame()
	}
	if sd.controlConn != nil {
		go sd.transferControlMsg()
	}
}

func (sd *ScrcpyDriver) UpdateDriverConfig(config map[string]string) error {
	return nil
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
		sd.sendCachedKeyFrame()
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
		log.Println("First frame IDR request, sending cached key frame")
		sd.sendCachedKeyFrame()
		sd.KeyFrameRequest()
		return
	} else if time.Since(sd.LastIDRRequestTime) < 2*time.Second {
		// log.Printf("last IDR time request was %.1f seconds ago, sending cached key frame", time.Since(sd.LastIDRRequestTime).Seconds())
		sd.sendCachedKeyFrame()
		return
	}

	sd.KeyFrameRequest()
	sd.LastIDRRequestTime = time.Now()
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
	// sd.adbClient.ReverseRemove(fmt.Sprintf("localabstract:scrcpy_%s", sd.scid))
	sd.adbClient.Stop()
	sd.cancel()
}
