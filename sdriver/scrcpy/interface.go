package scrcpy

import (
	"fmt"
	"log"
	"time"
	"webcpy/sdriver"
)

// type SDriver interface {
// 	GetReceivers() (<-chan AVBox, <-chan AVBox, chan ControlEvent)

// 	StartStreaming()
// 	StopStreaming()

// 	SendControl(event ControlEvent) error
// 	RequestIDR() error
// 	Capabilities() DriverCaps
// 	CodecInfo() (videoCodec string, audioCodec string)
// 	MediaMeta() MediaMeta
// 	Stop()
// }

func (sd *ScrcpyDriver) GetReceivers() (<-chan sdriver.AVBox, <-chan sdriver.AVBox, <-chan sdriver.ControlEvent) {
	return sd.VideoChan, sd.AudioChan, sd.ControlChan
}

func (sd *ScrcpyDriver) StartStreaming() {
	log.Println("ScrcpyDriver: StartStreaming called")
	go sd.convertVideoFrame()
	go sd.convertAudioFrame()
	go sd.transferControlMsg()
}

func (sd *ScrcpyDriver) StopStreaming() {
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
	case *sdriver.UHIDCreateEvent:
		sd.SendUHIDCreateEvent(e)
	case *sdriver.UHIDInputEvent:
		sd.SendUHIDInputEvent(e)
	case *sdriver.UHIDDestroyEvent:
		sd.SendUHIDDestroyEvent(e)
	case *sdriver.ReqIDREvent:
		sd.KeyFrameRequest()
	default:
		// Fallback for IDR if passed as a different struct with same Type
		if event.Type() == sdriver.ControlMsgTypeReqIDR {
			sd.KeyFrameRequest()
			return nil
		}
		log.Printf("ScrcpyDriver: Unhandled event type: %T", event)
	}

	return nil
}

func (sd *ScrcpyDriver) RequestIDR() {
	sd.keyFrameMutex.RLock()
	sps := append([]byte(nil), sd.LastSPS...)
	pps := append([]byte(nil), sd.LastPPS...)
	vps := append([]byte(nil), sd.LastVPS...)
	// idr := append([]byte(nil), sd.LastIDR...)
	sd.keyFrameMutex.RUnlock()

	if len(vps) > 0 {
		select {
		case sd.VideoChan <- sdriver.AVBox{Data: vps, PTS: time.Duration(time.Now().Unix()), IsKeyFrame: false, IsConfig: true}:
		default:
		}
	}
	if len(sps) > 0 {
		select {
		case sd.VideoChan <- sdriver.AVBox{Data: sps, PTS: time.Duration(time.Now().Unix()), IsKeyFrame: false, IsConfig: true}:
		default:
		}
	}
	if len(pps) > 0 {
		select {
		case sd.VideoChan <- sdriver.AVBox{Data: pps, PTS: time.Duration(time.Now().Unix()), IsKeyFrame: false, IsConfig: true}:
		default:
		}
	}
	// if len(idr) > 0 {
	// 	select {
	// 	case sd.VideoChan <- sdriver.AVBox{Data: idr, PTS: 0, IsKeyFrame: true, IsConfig: false}:
	// 	default:
	// 	}
	// }
	sd.KeyFrameRequest()
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
	sd.adbClient.ReverseRemove(fmt.Sprintf("localabstract:scrcpy_%s", sd.socketName))
	sd.adbClient.Stop()
}
