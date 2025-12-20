package webservice

// import (
// 	"fmt"
// 	"time"
// 	"webscreen/webservice/android"
// )

// func (wm *WebMaster) AndroidDevicesDiscovery() {
// 	for {
// 		time.Sleep(2 * time.Second)
// 		if wm.pauseDiscovery {
// 			continue
// 		}
// 		androidDevices := android.FindAndroidDevices()

// 		wm.devicesDiscoveredMu.Lock()

// 		for _, d := range androidDevices {
// 			id := fmt.Sprintf("%s:%s:%d", d.GetType(), d.GetDeviceID(), d.GetPort())
// 			wm.devicesDiscovered[id] = d
// 		}
// 		// log.Printf("Discovered Android device: %+v\n", androidDevices)

// 		wm.devicesDiscoveredMu.Unlock()
// 	}
// }
