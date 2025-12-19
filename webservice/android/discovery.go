package android

import (
	"context"
	"log"
	"time"

	"github.com/grandcat/zeroconf"
)

// FindDevices 使用 mDNS 查找局域网内的 Android 设备
func FindAndroidDevices() []AndroidDevice {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Println("Failed to initialize resolver:", err)
		return []AndroidDevice{}
	}

	entries := make(chan *zeroconf.ServiceEntry)
	var devices []AndroidDevice

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	// Discover ADB services
	// _adb._tcp is the standard service type for ADB over Wi-Fi
	if err := resolver.Browse(ctx, "_adb._tcp", "local.", entries); err != nil {
		log.Println("Failed to browse:", err)
		return []AndroidDevice{}
	}

	for entry := range entries {
		ip := ""
		// 如果没有找到 192.168，则使用第一个 IPv4
		if len(entry.AddrIPv4) > 0 {
			ip = entry.AddrIPv4[0].String()
		}
		// 最后尝试 IPv6
		if ip == "" && len(entry.AddrIPv6) > 0 {
			ip = entry.AddrIPv6[0].String()
		}

		devices = append(devices, AndroidDevice{
			DeviceID: entry.Instance,
			IP:       ip,
			Port:     entry.Port,
		})
	}

	return devices
}
