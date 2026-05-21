package android

import (
	"context"
	"webscreen/sdriver/scrcpy"
)

func GetDevices() ([]AndroidDevice, error) {
	transport, err := scrcpy.NewLocalADBTransport()
	if err != nil {
		return nil, err
	}
	devices, err := transport.Devices(context.Background())
	if err != nil {
		return nil, err
	}

	adbDevices := make([]AndroidDevice, 0, len(devices))
	for _, device := range devices {
		adbDevices = append(adbDevices, AndroidDevice{
			DeviceID: device.Serial,
			Status:   string(device.Status),
		})
	}
	return adbDevices, nil
}

func ConnectDevice(address string) error {
	transport, err := scrcpy.NewLocalADBTransport()
	if err != nil {
		return err
	}
	return transport.Connect(context.Background(), address)
}

func PairDevice(address, code string) error {
	transport, err := scrcpy.NewLocalADBTransport()
	if err != nil {
		return err
	}
	return transport.Pair(context.Background(), address, code)
}
