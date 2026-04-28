package linuxcapturer

// ================= 定义动作常量 =================
const WheelStep = 40

// 鼠标\触控动作类型
const (
	MouseActionDown = 0
	MouseActionUp   = 1
	MouseActionMove = 2
)

const (
	TouchActionDown = 0
	TouchActionUp   = 1
	TouchActionMove = 2
)

// X11 鼠标按键映射 (标准定义)
const (
	MouseBtnLeft      = 1
	MouseBtnMiddle    = 2
	MouseBtnRight     = 3
	MouseBtnWheelUp   = 4
	MouseBtnWheelDown = 5
)

// Web 端传入的 Button 掩码 (与你之前的定义保持一致)
const (
	WebBtnPrimary   uint32 = 1 << 0 // 左键
	WebBtnSecondary uint32 = 1 << 1 // 右键 (Web 通常把右键定义为 2)
	WebBtnTertiary  uint32 = 1 << 2 // 中键
)

// 键盘动作
const (
	KeyActionDown = 0 // 按下
	KeyActionUp   = 1 // 抬起
)
