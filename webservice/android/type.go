package android

import sagent "webscreen/streamAgent"

type AndroidDevice struct {
	DeviceID string `json:"device_id"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Status   string `json:"status"`
	// 以下字段由设备管理器（adbmanager.go）填充，用于区分 USB / Wi-Fi、
	// 显示授权状态与用户自定义名称；老前端只读上面几个字段，保持兼容。
	Connection string `json:"connection,omitempty"` // USB / WiFi
	Name       string `json:"name,omitempty"`       // 用户自定义名称或自动名称
	Model      string `json:"model,omitempty"`      // 自动识别的厂商 + 型号
	State      string `json:"state,omitempty"`      // ONLINE / UNAUTHORIZED / OFFLINE / ...
	Streaming  bool   `json:"streaming,omitempty"`  // 是否正在投屏
}

func (d AndroidDevice) GetType() string {
	return sagent.DEVICE_TYPE_ANDROID
}

func (d AndroidDevice) GetDeviceID() string {
	return d.DeviceID
}

func (d AndroidDevice) GetIP() string {
	return d.IP
}

func (d AndroidDevice) GetPort() int {
	return d.Port
}

func (d AndroidDevice) GetStatus() string {
	return d.Status
}
