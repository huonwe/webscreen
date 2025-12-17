package webservice

import (
	"fmt"
	"log"
	"time"
	"webcpy/webservice/android"
)

func (wm *WebMaster) AndroidDevicesDiscovery() {
	for {
		time.Sleep(2 * time.Second)
		if wm.pauseDiscovery {
			continue
		}
		androidDevices := android.FindAndroidDevices()

		wm.devicesDiscoveredMu.Lock()

		for _, d := range androidDevices {
			device := Device{
				Type:     "android",
				DeviceID: d.DeviceID,
				IP:       d.IP,
				Port:     d.Port,
			}
			id := fmt.Sprintf("%s:%s:%d", device.Type, device.DeviceID, device.Port)
			wm.devicesDiscovered[id] = device
		}
		log.Printf("Discovered Android device: %+v\n", androidDevices)

		wm.devicesDiscoveredMu.Unlock()
	}
}
