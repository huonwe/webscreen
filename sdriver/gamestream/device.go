package gamestream

import (
	"strings"
	"time"

	"github.com/hashicorp/mdns"
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
	const service = "_nvstream._tcp"
	const domain = "local."

	// 使用带缓冲的 channel，避免 mdns.Query 在高并发响应时阻塞
	entriesCh := make(chan *mdns.ServiceEntry, 32)

	var devices []DiscoveredDevice
	seen := make(map[string]bool)
	done := make(chan struct{})

	go func() {
		defer close(done)
		suffix := "." + service + "." + domain

		for e := range entriesCh {
			// hashicorp/mdns 的 AddrV4 是单个 net.IP，需判空及排除 0.0.0.0
			if e.AddrV4 == nil || e.AddrV4.IsUnspecified() || e.AddrV4.To4() == nil {
				continue
			}
			ip := e.AddrV4.String()
			if seen[ip] {
				continue
			}
			seen[ip] = true

			// hashicorp/mdns 的 e.Name 为完整实例名（如 "MyPC._nvstream._tcp.local."）
			// 剥离服务后缀以保持与原 zeroconf 的 e.Instance 一致
			name := strings.TrimSuffix(e.Name, suffix)
			name = strings.TrimSuffix(name, ".")
			if name == "" {
				name = ip
			}

			port := e.Port
			if port <= 0 {
				port = 47989
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

	params := mdns.DefaultParams(service)
	params.Domain = domain
	params.Entries = entriesCh
	params.Timeout = 1500 * time.Millisecond
	// Windows 下禁用 IPv6 组播可避免虚拟网卡接口绑定失败及响应延迟
	params.DisableIPv6 = true
	// Interface 为 nil 时，hashicorp/mdns 会自动遍历所有 FlagUp | FlagMulticast 的网卡
	params.Interface = nil

	err := mdns.Query(params)
	// mdns.Query 阻塞直到超时结束，且不会主动关闭 Entries channel，必须手动关闭
	close(entriesCh)
	<-done

	if err != nil {
		return nil, err
	}
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
