package streamServer

import (
	"log"
	"sync"
	"webcpy/scrcpy"
)

type StreamController struct {
	sync.RWMutex
	CurrentStreamManager *StreamManager
	CurrentDataAdapter   *scrcpy.DataAdapter
}

var GlobalStreamController = &StreamController{}

func (sc *StreamController) StartStream(serial string, options scrcpy.ScrcpyOptions) error {
	sc.Lock()
	defer sc.Unlock()

	// Stop existing stream if any
	if sc.CurrentStreamManager != nil {
		sc.CurrentStreamManager.Close()
		sc.CurrentStreamManager = nil
	}
	if sc.CurrentDataAdapter != nil {
		sc.CurrentDataAdapter.Close()
		sc.CurrentDataAdapter = nil
	}

	log.Printf("Starting stream for device: %s", serial)

	// Configure Scrcpy
	config := map[string]string{
		"device_serial":      serial,
		"server_local_path":  "./scrcpy-server-v3.3.3-m", // Hardcoded for now, should be configurable
		"server_remote_path": "/data/local/tmp/scrcpy-server-dev",
		"scrcpy_version":     "3.3.3",
		"local_port":         "6000", // Should be dynamic if multiple streams supported
	}

	// Initialize DataAdapter
	// Pass options to NewDataAdapter if it supports it, currently it takes map[string]string
	// We might need to update NewDataAdapter to accept ScrcpyOptions struct or merge it into config map
	// For now, assuming NewDataAdapter uses default options internally or we need to pass them via config map
	// Let's update config map with options if possible, but NewDataAdapter signature is map[string]string
	
	da, err := scrcpy.NewDataAdapter(config)
	if err != nil {
		log.Printf("Failed to create DataAdapter: %v", err)
		return err
	}

	// Initialize StreamManager
	sm := NewStreamManager(da, sc)

	sc.CurrentDataAdapter = da
	sc.CurrentStreamManager = sm

	da.ShowDeviceInfo()
	da.StartConvertVideoFrame()
	da.StartConvertAudioFrame()

	// Start pumping frames
	go func() {
		videoChan := da.VideoChan
		for frame := range videoChan {
			// Check if stream is still active
			sc.RLock()
			currentSM := sc.CurrentStreamManager
			sc.RUnlock()
			if currentSM != sm {
				return
			}
			sm.WriteVideoSample(&frame)
		}
	}()

	go func() {
		audioChan := da.AudioChan
		for frame := range audioChan {
			sc.RLock()
			currentSM := sc.CurrentStreamManager
			sc.RUnlock()
			if currentSM != sm {
				return
			}
			sm.WriteAudioSample(&frame)
		}
	}()

	return nil
}

func (sc *StreamController) StopStream() {
	sc.Lock()
	defer sc.Unlock()

	if sc.CurrentStreamManager != nil {
		log.Println("Stopping StreamManager...")
		sc.CurrentStreamManager.Close()
		sc.CurrentStreamManager = nil
	}
	if sc.CurrentDataAdapter != nil {
		log.Println("Stopping DataAdapter...")
		sc.CurrentDataAdapter.Close()
		sc.CurrentDataAdapter = nil
	}
}
