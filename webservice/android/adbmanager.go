package android

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"webscreen/utils"
)

// 设备状态机取值（需求第 21 节）
const (
	StatusOnline       = "ONLINE"
	StatusOffline      = "OFFLINE"
	StatusConnecting   = "CONNECTING"
	StatusUnauthorized = "UNAUTHORIZED"
	StatusDisconnected = "DISCONNECTED"
	StatusError        = "ERROR"
)

// 连接类型（需求第 10 节）
const (
	TransportUSB  = "USB"
	TransportWiFi = "WiFi"
)

const (
	pollInterval = 2 * time.Second
	maxEvents    = 500
)

// ADBDevice 是一条 `adb devices -l` 记录解析后的结果。
type ADBDevice struct {
	Serial    string `json:"serial"`
	State     string `json:"state"`     // device / unauthorized / offline / no permissions ...
	Transport string `json:"transport"` // USB / WiFi
	Address   string `json:"address"`   // WiFi 时为 ip:port
	Model     string `json:"model"`
	Product   string `json:"product"`
	Device    string `json:"device"`
}

// ManagedDevice 是注册表里的一台设备，携带用户自定义名称与持久化字段。
type ManagedDevice struct {
	Serial     string `json:"serial"`      // 当前使用的 adb serial（USB 优先）
	HardwareID string `json:"hardware_id"` // 硬件序列号，用于把 USB 与 Wi-Fi 归并为同一台设备
	Name       string `json:"name"`        // 用户自定义名称
	AutoName   string `json:"auto_name"`   // 自动推导名称（厂商 + 型号）
	Transport  string `json:"transport"`
	Address    string `json:"address"`   // Wi-Fi 地址 ip:port
	WiFiAddr   string `json:"wifi_addr"` // 备用的无线地址，USB 断开时用于恢复
	Status     string `json:"status"`
	RawState   string `json:"raw_state"`
	Streaming  bool   `json:"streaming"`
	LastSeen   string `json:"last_seen"`
}

// DisplayName 返回界面应显示的名称。
func (d ManagedDevice) DisplayName() string {
	if d.Name != "" {
		return d.Name
	}
	if d.AutoName != "" {
		return d.AutoName
	}
	return d.Serial
}

// Event 是一条给 Web UI 看的日志（需求第 30 节）。
type Event struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// persistedState 是落盘内容：设备自定义名称与无线地址在重启/升级后保留（需求第 28 节）。
type persistedState struct {
	Devices map[string]persistedDevice `json:"devices"`
}

type persistedDevice struct {
	Name     string `json:"name"`
	WiFiAddr string `json:"wifi_addr"`
}

// Manager 统一管理 USB 与 Wi-Fi 两种接入方式的 Android 设备。
type Manager struct {
	mu        sync.RWMutex
	devices   map[string]*ManagedDevice // key: 当前 adb serial
	events    []Event
	stateFile string

	// 无线的期望目标：例如 USB 优先的那台设备被拔掉后，用它恢复无线连接
	wifiTargets map[string]string // hardwareID -> ip:port
	lastResult  map[string]string // serial -> 上次 adb 状态，用于 diff 出事件
	hwCache     map[string]string // serial -> 硬件序列号
	nameCache   map[string]string // serial -> 自动名称

	stopOnce sync.Once
	stopCh   chan struct{}
}

var (
	manager     *Manager
	managerOnce sync.Once
)

// GetManager 返回进程内唯一的设备管理器（第一次调用时创建并加载持久化状态）。
func GetManager() *Manager {
	managerOnce.Do(func() {
		manager = newManager()
	})
	return manager
}

func newManager() *Manager {
	m := &Manager{
		devices:     make(map[string]*ManagedDevice),
		stateFile:   filepath.Join(dataDir(), "devices.json"),
		wifiTargets: make(map[string]string),
		lastResult:  make(map[string]string),
		hwCache:     make(map[string]string),
		nameCache:   make(map[string]string),
		stopCh:      make(chan struct{}),
	}
	m.load()
	return m
}

// dataDir 返回持久化目录：优先 webscreen_datadir（fnOS fpk 传入 TRIM_PKGVAR），
// 否则退回当前工作目录，保证非 fpk 场景也不丢数据。
func dataDir() string {
	if dir := os.Getenv("webscreen_datadir"); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		return dir
	}
	if dir := os.Getenv("TRIM_PKGVAR"); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		return dir
	}
	return "."
}

func (m *Manager) load() {
	raw, err := os.ReadFile(m.stateFile)
	if err != nil {
		return
	}
	var st persistedState
	if err := json.Unmarshal(raw, &st); err != nil {
		log.Printf("[adb] 设备注册表解析失败: %v", err)
		return
	}
	for serial, d := range st.Devices {
		m.devices[serial] = &ManagedDevice{Serial: serial, Name: d.Name, WiFiAddr: d.WiFiAddr, Status: StatusDisconnected}
		if d.WiFiAddr != "" {
			m.wifiTargets[serial] = d.WiFiAddr
		}
	}
	log.Printf("[adb] 已加载设备注册表（%d 条）", len(st.Devices))
}

func (m *Manager) save() {
	st := persistedState{Devices: make(map[string]persistedDevice, len(m.devices))}
	for serial, d := range m.devices {
		if d.Name == "" && d.WiFiAddr == "" {
			continue
		}
		st.Devices[serial] = persistedDevice{Name: d.Name, WiFiAddr: d.WiFiAddr}
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(m.stateFile, raw, 0o644); err != nil {
		log.Printf("[adb] 设备注册表写入失败: %v", err)
	}
}

// Start 启动后台轮询：USB 热插拔、授权状态变化与无线恢复都在这里处理。
func (m *Manager) Start() {
	go func() {
		log.Printf("[adb] 设备管理器已启动（轮询间隔 %s）", pollInterval)
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		m.poll()
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.poll()
			}
		}
	}()
}

// Stop 停止轮询（进程退出时调用）。
func (m *Manager) Stop() {
	m.stopOnce.Do(func() { close(m.stopCh) })
}

// ListADBDevices 运行 `adb devices -l` 并解析。
func ListADBDevices() ([]ADBDevice, error) {
	out, err := runADB("devices", "-l")
	if err != nil {
		return nil, err
	}
	return ParseDevicesOutput(out), nil
}

// ParseDevicesOutput 解析 `adb devices -l` 的输出，并区分 USB 与 Wi-Fi。
func ParseDevicesOutput(out string) []ADBDevice {
	var devices []ADBDevice
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "List of devices") || strings.HasPrefix(trimmed, "* daemon") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		d := ADBDevice{Serial: fields[0], State: fields[1]}
		// `-l` 会给 USB 设备带上 usb:<bus>-<port>，Wi-Fi 设备没有
		d.Transport = TransportWiFi
		for _, f := range fields[2:] {
			switch {
			case strings.HasPrefix(f, "usb:"):
				d.Transport = TransportUSB
			case strings.HasPrefix(f, "model:"):
				d.Model = strings.TrimPrefix(f, "model:")
			case strings.HasPrefix(f, "product:"):
				d.Product = strings.TrimPrefix(f, "product:")
			case strings.HasPrefix(f, "device:"):
				d.Device = strings.TrimPrefix(f, "device:")
			}
		}
		if strings.Contains(d.Serial, ":") && !strings.HasPrefix(d.Serial, "emulator-") {
			// 经典 `adb connect ip:5555` 的条目，serial 就是地址
			d.Transport = TransportWiFi
			d.Address = d.Serial
		}
		devices = append(devices, d)
	}
	return devices
}

func runADB(args ...string) (string, error) {
	adbPath, err := utils.GetADBPath()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(adbPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("adb %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func runADBTarget(serial string, args ...string) (string, error) {
	full := append([]string{"-s", serial}, args...)
	return runADB(full...)
}

// poll 是一次完整的设备状态刷新。
func (m *Manager) poll() {
	found, err := ListADBDevices()
	if err != nil {
		m.mu.Lock()
		// adb 不可用（尚未启动/被杀）时不要清空注册表，只记录一次错误
		if m.lastResult["__error"] != err.Error() {
			m.addEventLocked("error", fmt.Sprintf("ADB 不可用：%s", err.Error()))
			m.lastResult["__error"] = err.Error()
		}
		m.mu.Unlock()
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lastResult["__error"] != "" {
		delete(m.lastResult, "__error")
	}

	seen := make(map[string]bool)
	// 先按硬件序列号归并：同一台设备同时有 USB 与 Wi-Fi 时优先 USB
	byHardware := make(map[string][]ADBDevice)
	for _, d := range found {
		if d.State != "device" {
			byHardware[d.Serial] = append(byHardware[d.Serial], d)
			continue
		}
		hw := m.hardwareID(d.Serial)
		byHardware[hw] = append(byHardware[hw], d)
	}

	for _, group := range byHardware {
		if len(group) == 0 {
			continue
		}
		primary := group[0]
		var fallback *ADBDevice
		for i := range group {
			d := group[i]
			if d.Transport == TransportUSB {
				// USB 优先（需求第 13 节）
				primary = d
			} else if d.Address != "" && fallback == nil {
				fallback = &group[i]
			}
		}
		seen[primary.Serial] = true
		m.updateDevice(primary, fallback)

		// 归并残留：同一台硬件的其它 adb 条目（例如先手动添加的无线地址）
		// 合并进主条目，否则界面上会出现两台"同一台手机"。
		dev := m.devices[primary.Serial]
		for i := range group {
			if group[i].Serial == primary.Serial {
				continue
			}
			old, ok := m.devices[group[i].Serial]
			if !ok {
				continue
			}
			if dev != nil {
				if dev.Name == "" && old.Name != "" {
					dev.Name = old.Name
				}
				if dev.WiFiAddr == "" && old.WiFiAddr != "" {
					dev.WiFiAddr = old.WiFiAddr
				}
			}
			delete(m.devices, group[i].Serial)
			delete(m.lastResult, group[i].Serial)
			m.save()
		}
	}

	// 注册表里已有、但这次没出现的设备 -> OFFLINE / DISCONNECTED
	for serial, dev := range m.devices {
		if seen[serial] {
			continue
		}
		if dev.Status == StatusDisconnected {
			continue
		}
		dev.Status = StatusDisconnected
		dev.RawState = "disconnected"
		dev.Streaming = false
		dev.Transport = ""
		if m.lastResult[serial] != StatusDisconnected {
			m.addEventLocked("warn", fmt.Sprintf("设备已断开：%s（serial %s）", dev.DisplayName(), serial))
		}
		m.lastResult[serial] = StatusDisconnected
	}
}

func (m *Manager) updateDevice(d ADBDevice, fallback *ADBDevice) {
	dev, ok := m.devices[d.Serial]
	if !ok {
		dev = &ManagedDevice{Serial: d.Serial, Status: StatusDisconnected}
		// 若之前以无线地址为 key 记录过，把自定义名称迁移过来
		if d.Address != "" {
			if old, exists := m.devices[d.Address]; exists && old.Name != "" {
				dev.Name = old.Name
			}
		}
		m.devices[d.Serial] = dev
	}

	prev := m.lastResult[d.Serial]
	if prev == "" {
		m.addEventLocked("info", fmt.Sprintf("检测到 Android 设备：%s（%s，serial %s）", d.Serial, d.Transport, d.Serial))
	}

	dev.Transport = d.Transport
	dev.RawState = d.State
	dev.Address = d.Address
	if hw := m.hardwareID(d.Serial); hw != "" {
		dev.HardwareID = hw
	}
	if d.Transport == TransportWiFi && d.Address != "" {
		dev.WiFiAddr = d.Address
	}
	if fallback != nil && fallback.Address != "" {
		dev.WiFiAddr = fallback.Address
	}
	dev.LastSeen = time.Now().Format("2006-01-02 15:04:05")
	if dev.AutoName == "" && m.nameCache[d.Serial] != "" {
		dev.AutoName = m.nameCache[d.Serial]
	}

	switch d.State {
	case "device":
		dev.Status = StatusOnline
		if prev != "" && prev != StatusOnline {
			m.addEventLocked("info", fmt.Sprintf("ADB 设备已连接：%s（%s）", dev.DisplayName(), d.Transport))
		}
		m.resolveAutoNameLocked(d.Serial, d)
	case "unauthorized":
		dev.Status = StatusUnauthorized
		if prev != StatusUnauthorized {
			m.addEventLocked("warn", fmt.Sprintf("设备等待 USB 调试授权：%s，请在手机屏幕上点击“允许 USB 调试”", d.Serial))
		}
	case "offline":
		dev.Status = StatusOffline
		if prev != StatusOffline {
			m.addEventLocked("warn", fmt.Sprintf("设备离线：%s（可能是 USB 拔出或连接不稳定）", d.Serial))
		}
	default:
		dev.Status = StatusError
	}
	m.lastResult[d.Serial] = dev.Status
}

// hardwareID 取设备硬件序列号，用于把 USB 与 Wi-Fi 条目归并成同一台设备。
// 取不到时退回 adb serial 本身。
func (m *Manager) hardwareID(serial string) string {
	if hw, ok := m.hwCache[serial]; ok {
		return hw
	}
	out, err := runADBTarget(serial, "shell", "getprop", "ro.serialno")
	hw := serial
	if err == nil {
		if v := strings.TrimSpace(out); v != "" {
			hw = v
		}
	}
	m.hwCache[serial] = hw
	return hw
}

// resolveAutoNameLocked 用厂商 + 型号生成自动名称（需求第 20 节），每个 serial 只查一次。
func (m *Manager) resolveAutoNameLocked(serial string, d ADBDevice) {
	dev := m.devices[serial]
	if dev == nil || dev.AutoName != "" {
		return
	}
	if cached, ok := m.nameCache[serial]; ok && cached != "" {
		dev.AutoName = cached
		return
	}
	manufacturer, _ := runADBTarget(serial, "shell", "getprop", "ro.product.manufacturer")
	model, _ := runADBTarget(serial, "shell", "getprop", "ro.product.model")
	name := strings.TrimSpace(strings.TrimSpace(manufacturer) + " " + strings.TrimSpace(model))
	if name == "" && d.Model != "" {
		name = d.Model
	}
	if name == "" {
		return
	}
	m.nameCache[serial] = name
	dev.AutoName = name
	m.save()
}

// Snapshot 返回给 API 用的设备列表（USB 在前，其次按名称排序）。
func (m *Manager) Snapshot() []ManagedDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]ManagedDevice, 0, len(m.devices))
	for _, d := range m.devices {
		list = append(list, *d)
	}
	sort.SliceStable(list, func(i, j int) bool {
		if (list[i].Transport == TransportUSB) != (list[j].Transport == TransportUSB) {
			return list[i].Transport == TransportUSB
		}
		return list[i].DisplayName() < list[j].DisplayName()
	})
	return list
}

// Events 返回最近的日志事件（新的在前）。
func (m *Manager) Events(limit int) []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}
	out := make([]Event, 0, limit)
	for i := len(m.events) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.events[i])
	}
	return out
}

// SetName 保存用户自定义设备名称（持久化，升级不丢）。
func (m *Manager) SetName(serial, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	dev, ok := m.devices[serial]
	if !ok {
		return fmt.Errorf("未知设备：%s", serial)
	}
	dev.Name = strings.TrimSpace(name)
	m.save()
	m.addEventLocked("info", fmt.Sprintf("设备名称已更新：%s -> %s", serial, dev.DisplayName()))
	return nil
}

// AddWiFi 手动添加无线设备（ip:port），并记住它以便断线重连。
func (m *Manager) AddWiFi(address string) error {
	address = strings.TrimSpace(address)
	if address == "" {
		return fmt.Errorf("地址不能为空")
	}
	if !strings.Contains(address, ":") {
		address += ":5555"
	}
	m.mu.Lock()
	m.addEventLocked("info", fmt.Sprintf("正在连接无线设备 %s ...", address))
	m.mu.Unlock()

	out, err := runADB("connect", address)
	if err != nil {
		m.mu.Lock()
		m.addEventLocked("error", fmt.Sprintf("无线连接失败：%s（%s）", address, strings.TrimSpace(out)))
		m.mu.Unlock()
		return fmt.Errorf("adb connect %s 失败：%s", address, strings.TrimSpace(out))
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wifiTargets[address] = address
	dev, ok := m.devices[address]
	if !ok {
		dev = &ManagedDevice{Serial: address, Address: address, WiFiAddr: address, Transport: TransportWiFi, Status: StatusConnecting}
		m.devices[address] = dev
	}
	dev.WiFiAddr = address
	m.save()
	m.addEventLocked("info", fmt.Sprintf("无线设备已连接：%s", address))
	// 注意：poll() 自己会加锁，必须在释放锁之后调用，否则死锁（请求挂住并且轮询线程也停摆）
	go m.poll()
	return nil
}

// RemoveDevice 从注册表移除一台设备（若为无线则先断开）。
func (m *Manager) RemoveDevice(serial string) error {
	m.mu.Lock()
	dev, ok := m.devices[serial]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("未知设备：%s", serial)
	}
	addr := dev.WiFiAddr
	name := dev.DisplayName()
	delete(m.devices, serial)
	delete(m.lastResult, serial)
	if addr != "" {
		delete(m.wifiTargets, addr)
	}
	m.save()
	m.addEventLocked("info", fmt.Sprintf("设备已移除：%s", name))
	m.mu.Unlock()
	if addr != "" {
		_, _ = runADB("disconnect", addr)
	}
	return nil
}

// Disconnect 断开一台设备：USB 无法软断开（提示用户拔线），无线则主动断开。
func (m *Manager) Disconnect(serial string) error {
	m.mu.Lock()
	dev, ok := m.devices[serial]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("未知设备：%s", serial)
	}
	transport, addr := dev.Transport, dev.WiFiAddr
	m.mu.Unlock()
	if transport == TransportUSB {
		return fmt.Errorf("USB 连接的设备无法从软件断开，请直接拔掉数据线")
	}
	if addr == "" {
		addr = serial
	}
	_, err := runADB("disconnect", addr)
	m.mu.Lock()
	if err == nil {
		m.addEventLocked("info", fmt.Sprintf("已断开无线设备：%s", addr))
	}
	m.mu.Unlock()
	return err
}

// Reconnect 重新连接一台设备：USB 等待重新插入，无线则重试 adb connect。
func (m *Manager) Reconnect(serial string) error {
	m.mu.Lock()
	dev, ok := m.devices[serial]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("未知设备：%s", serial)
	}
	addr := dev.WiFiAddr
	if addr == "" {
		addr = dev.Address
	}
	transport := dev.Transport
	m.mu.Unlock()

	if transport == TransportUSB || addr == "" {
		// USB：重新枚举一次，真正恢复靠插回数据线
		m.mu.Lock()
		m.addEventLocked("info", fmt.Sprintf("等待 USB 设备重新插入：%s", serial))
		m.mu.Unlock()
		m.mu.Lock()
		m.hwCache = map[string]string{}
		m.mu.Unlock()
		return nil
	}
	_, err := runADB("connect", addr)
	m.mu.Lock()
	if err == nil {
		m.addEventLocked("info", fmt.Sprintf("正在重连无线设备：%s", addr))
	} else {
		m.addEventLocked("error", fmt.Sprintf("无线重连失败：%s", addr))
	}
	m.mu.Unlock()
	return err
}

// RescanUSB 重新走一遍 ADB 握手，用于「刷新并请求 USB 授权」：
// 未授权设备只有在收到新的 ADB 握手时，手机侧 adbd 才会再次弹出
// 「允许 USB 调试」对话框，因此这里先尝试 device 侧重连，再退回重启 adb server。
func (m *Manager) RescanUSB() error {
	m.mu.Lock()
	m.addEventLocked("info", "正在刷新 USB 设备并重新请求调试授权…")
	m.mu.Unlock()

	// 温和路径：让已连接/离线设备重新握手
	if _, err := runADB("reconnect", "device"); err != nil {
		m.mu.Lock()
		m.addEventLocked("warn", fmt.Sprintf("adb reconnect device 未生效：%v，改为重启 adb server", err))
		m.mu.Unlock()
	}
	// 彻底路径：重启 adb server（USB 设备会被重新枚举并重新握手）
	if _, err := runADB("kill-server"); err != nil {
		m.mu.Lock()
		m.addEventLocked("warn", fmt.Sprintf("adb kill-server 失败：%v", err))
		m.mu.Unlock()
	}
	if _, err := runADB("start-server"); err != nil {
		m.mu.Lock()
		m.addEventLocked("error", fmt.Sprintf("adb start-server 失败：%v", err))
		m.mu.Unlock()
		return err
	}
	m.poll()
	m.mu.Lock()
	m.addEventLocked("info", "USB 设备已重新枚举，未授权设备请在手机屏幕上点击“允许 USB 调试”")
	m.mu.Unlock()
	return nil
}

// MarkStreaming 记录某台设备当前的投屏状态，供界面显示。
func (m *Manager) MarkStreaming(serial string, streaming bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, dev := range m.devices {
		if dev.Serial == serial || dev.Address == serial || dev.Name == serial || dev.AutoName == serial || dev.DisplayName() == serial {
			dev.Streaming = streaming
			return
		}
	}
}

func (m *Manager) addEventLocked(level, message string) {
	ev := Event{Time: time.Now().Format("2006-01-02 15:04:05"), Level: level, Message: message}
	m.events = append(m.events, ev)
	if len(m.events) > maxEvents {
		m.events = m.events[len(m.events)-maxEvents:]
	}
	log.Printf("[adb] %s", message)
}

// adbDeviceToAndroidDevice 把注册表记录转换成上游设备列表用的结构（保持字段兼容）。
func (m *Manager) legacyDevices() []AndroidDevice {
	snap := m.Snapshot()
	out := make([]AndroidDevice, 0, len(snap))
	for _, d := range snap {
		status := "connected"
		switch d.Status {
		case StatusUnauthorized:
			status = "unauthorized"
		case StatusOffline:
			status = "offline"
		case StatusDisconnected, StatusError:
			status = "disconnected"
		case StatusConnecting:
			status = "connecting"
		}
		ip, port := "", 0
		if d.Transport == TransportWiFi && d.Address != "" {
			host, p, ok := splitHostPort(d.Address)
			ip = host
			if ok {
				port = p
			}
		}
		out = append(out, AndroidDevice{
			DeviceID:   d.Serial,
			IP:         ip,
			Port:       port,
			Status:     status,
			Connection: d.Transport,
			Name:       d.DisplayName(),
			Model:      d.AutoName,
			State:      d.Status,
			Streaming:  d.Streaming,
		})
	}
	return out
}

func splitHostPort(address string) (string, int, bool) {
	idx := strings.LastIndex(address, ":")
	if idx <= 0 {
		return address, 0, false
	}
	host := address[:idx]
	port := 0
	fmt.Sscanf(address[idx+1:], "%d", &port)
	return host, port, port > 0
}
