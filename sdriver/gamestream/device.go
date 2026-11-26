package gamestream

import (
	"context"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

type Device struct {
	DeviceID string `json:"device_id"`
	IP       string `json:"device_ip"`
	Port     int    `json:"device_port"`
	Status   string `json:"device_status"`
}

type DiscoveredDevice struct {
	DeviceID string `json:"device_id"`
	IP       string `json:"device_ip"`
	Port     int    `json:"device_port"`
	Status   string `json:"device_status"`
	Paired   bool   `json:"paired"`
}

func (d Device) GetType() string     { return "gamestream" }
func (d Device) GetDeviceID() string { return d.DeviceID }
func (d Device) GetIP() string       { return d.IP }
func (d Device) GetPort() int        { return d.Port }
func (d Device) GetStatus() string   { return d.Status }

// DiscoverDevices browses the LAN for all GameStream / Sunshine hosts advertising _nvstream._tcp.
func DiscoverDevices() ([]DiscoveredDevice, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}
	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	var devices []DiscoveredDevice
	seen := map[string]bool{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range entries {
			if len(e.AddrIPv4) == 0 {
				continue
			}
			ip := e.AddrIPv4[0].String()
			if seen[ip] {
				continue
			}
			seen[ip] = true
			name := strings.TrimSuffix(e.Instance, ".")
			if name == "" {
				name = ip
			}
			port := e.Port
			if port <= 0 {
				port = defaultHTTPPort
			}
			devices = append(devices, DiscoveredDevice{
				DeviceID: name,
				IP:       ip,
				Port:     port,
				Status:   "active",
				Paired:   IsHostPaired(ip),
			})
		}
	}()
	if err := resolver.Browse(ctx, "_nvstream._tcp", "local.", entries); err != nil {
		return nil, err
	}
	<-ctx.Done()
	<-done
	return devices, nil
}

// GetDevices returns only paired GameStream / Sunshine hosts so unpaired servers
// are not mixed into the normal device list.
func GetDevices() ([]Device, error) {
	discovered, err := DiscoverDevices()
	if err != nil {
		return nil, err
	}
	var paired []Device
	for _, d := range discovered {
		if d.Paired {
			paired = append(paired, Device{
				DeviceID: d.DeviceID,
				IP:       d.IP,
				Port:     d.Port,
				Status:   d.Status,
			})
		}
	}
	return paired, nil
}
