package main

import (
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

// ================= 定义动作常量 =================
const WheelStep = 40

// 鼠标动作
const (
	MouseActionMove = 0
	MouseActionDown = 1
	MouseActionUp   = 2
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

// ================= InputController 结构体 =================

type InputController struct {
	conn *xgb.Conn
	root xproto.Window
}

// NewInputController 初始化并连接到指定的 display
func NewInputController(display string) (*InputController, error) {
	c, err := xgb.NewConnDisplay(display)
	if err != nil {
		return nil, err
	}

	if err := xtest.Init(c); err != nil {
		c.Close()
		return nil, err
	}

	setup := xproto.Setup(c)
	root := setup.Roots[0].Root

	return &InputController{
		conn: c,
		root: root,
	}, nil
}

// Close 关闭连接
func (ic *InputController) Close() {
	if ic.conn != nil {
		ic.conn.Close()
	}
}

// ================= 对外暴露的两个核心函数 =================

// HandleMouseEvent 处理所有鼠标相关事件（移动、点击、拖拽、滚轮）
// action: 0=Move, 1=Down, 2=Up
// x, y: 绝对坐标
// buttons: Web端传来的按钮掩码 (支持左/中/右键)
// wheelDeltaX, wheelDeltaY: 滚轮滚动值 (如果是纯点击事件，传 0 即可)
func (ic *InputController) HandleMouseEvent(action byte, x, y int16, buttons uint32, wheelDeltaX, wheelDeltaY int16) {
	ic.moveMouse(x, y)

	if action == MouseActionMove && wheelDeltaY == 0 && wheelDeltaX == 0 {
		return
	}

	// 处理滚轮
	if wheelDeltaY != 0 {
		ic.handleWheel(wheelDeltaY)
		return
	}

	// 处理点击
	if action == MouseActionDown || action == MouseActionUp {
		x11Btn := ic.mapWebBtnToX11(buttons)
		isPress := (action == MouseActionDown)
		ic.sendMouseInput(x11Btn, isPress)
	}
}

// HandleKeyboardEvent 处理所有键盘相关事件
// action: 0=Down, 1=Up
// keycode: X11 对应的硬件扫描码 (Hardware Keycode)
func (ic *InputController) HandleKeyboardEvent(action byte, keycode byte) {
	// 过滤无效的 Keycode
	if keycode == 0 {
		return
	}

	isPress := (action == KeyActionDown)

	var eventType byte
	if isPress {
		eventType = xproto.KeyPress
	} else {
		eventType = xproto.KeyRelease
	}

	// 发送键盘事件
	// detail=keycode, delay=0
	xtest.FakeInput(ic.conn, eventType, keycode, 0, ic.root, 0, 0, 0)

	// 如果需要立即生效，可以 Sync，但在高频输入下不建议每次都 Sync
	// ic.conn.Sync()
}

// ================= 内部辅助函数 (不对外暴露) =================

// moveMouse 移动光标
func (ic *InputController) moveMouse(x, y int16) {
	// WarpPointer 瞬间移动
	xproto.WarpPointer(ic.conn, xproto.Window(0), ic.root, 0, 0, 0, 0, x, y)
}

// sendMouseInput 发送鼠标按键指令
func (ic *InputController) sendMouseInput(button byte, isPress bool) {
	var eventType byte
	if isPress {
		eventType = xproto.ButtonPress
	} else {
		eventType = xproto.ButtonRelease
	}
	xtest.FakeInput(ic.conn, eventType, button, 0, ic.root, 0, 0, 0)
}

// handleWheel 处理滚轮滚动
// Web 端传来的通常是像素差或行数，X11 需要转换为 Button 4 (上) 或 5 (下) 的点击
func (ic *InputController) handleWheel(deltaY int16) {
	if deltaY == 0 {
		return
	}

	var button byte
	// 注意：Web 的 deltaY > 0 通常是页面向下滚，对应滚轮向下 (Button 5)
	// 但不同浏览器可能不同，如果发现反了，这里调换一下 4 和 5
	if deltaY < 0 {
		button = MouseBtnWheelUp // 4
	} else {
		button = MouseBtnWheelDown // 5
	}

	// 计算需要触发几次点击
	// 使用绝对值计算
	absDelta := deltaY
	if absDelta < 0 {
		absDelta = -absDelta
	}

	// 比如 delta 是 120，Step 是 40，那就点击 3 次
	// 至少点击 1 次
	clicks := int(absDelta / WheelStep)
	if clicks == 0 {
		clicks = 1
	}

	// 限制最大单次点击数，防止前端发疯导致后端死循环卡死
	if clicks > 20 {
		clicks = 20
	}

	// 循环发送点击
	for i := 0; i < clicks; i++ {
		ic.sendMouseInput(button, true)  // Press
		ic.sendMouseInput(button, false) // Release

		// 极其重要！
		// 如果不加 Sleep，XServer 可能会把极短时间内的多次输入合并或丢弃
		// 或者应用程序反应不过来
		// 这里的 Sleep 不需要太久，几微秒即可，但在 Go 里最小单位方便控制的是 Sleep(0) 或 yield
		// xtest 实际上很快，一般不需要 sleep，如果发现滚动不顺畅再加
	}
}

// mapWebBtnToX11 将 Web 的按钮掩码映射为 X11 按钮 ID
func (ic *InputController) mapWebBtnToX11(buttons uint32) byte {
	if buttons&WebBtnPrimary != 0 {
		return MouseBtnLeft
	}
	if buttons&WebBtnSecondary != 0 {
		return MouseBtnRight // 注意：Web Secondary 对应 X11 右键 (ID 3)
	}
	if buttons&WebBtnTertiary != 0 {
		return MouseBtnMiddle // Web Tertiary 对应 X11 中键 (ID 2)
	}

	// 默认左键
	return MouseBtnLeft
}
