package linuxcapturer

import (
	"fmt"

	"github.com/bendahl/uinput"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

const (
	CONTROLLER_TYPE_X11     = "xtest"
	CONTROLLER_TYPE_WAYLAND = "uinput"
)

// InputController 负责处理多后端的底层输入操作
type InputController struct {
	controllerType string

	// ========== uinput (Wayland) 相关成员 ==========
	keyboard uinput.Keyboard
	mouse    uinput.Mouse

	// ========== X11 (xtest) 相关成员 ==========
	conn *xgb.Conn
	root xproto.Window

	// ========== 通用共享状态 ==========
	ctrlPressed, shiftPressed, altPressed, metaPressed bool
}

// NewInputController 初始化输入控制器
func NewInputController(controllerType string, display string) (*InputController, error) {
	ic := &InputController{
		controllerType: controllerType,
	}

	switch controllerType {
	case CONTROLLER_TYPE_X11:
		c, err := xgb.NewConnDisplay(display)
		if err != nil {
			return nil, err
		}
		if err := xtest.Init(c); err != nil {
			c.Close()
			return nil, err
		}
		setup := xproto.Setup(c)
		ic.conn = c
		ic.root = setup.Roots[0].Root

	case CONTROLLER_TYPE_WAYLAND:
		kb, err := uinput.CreateKeyboard("/dev/uinput", []byte("webscreen_keyboard"))
		if err != nil {
			return nil, err
		}
		m, err := uinput.CreateMouse("/dev/uinput", []byte("webscreen_mouse"))
		if err != nil {
			kb.Close()
			return nil, err
		}
		ic.keyboard = kb
		ic.mouse = m

	default:
		return nil, fmt.Errorf("unsupported controller type: %s", controllerType)
	}

	return ic, nil
}

// Close 释放所有资源
func (ic *InputController) Close() {
	if ic.keyboard != nil {
		ic.keyboard.Close()
	}
	if ic.mouse != nil {
		ic.mouse.Close()
	}
	if ic.conn != nil {
		ic.conn.Close()
	}
}

// HandleMouseEvent 处理鼠标事件并分发到对应底层接口
func (ic *InputController) HandleMouseEvent(action byte, deltaX, deltaY int32, buttons uint32, wheelDeltaX, wheelDeltaY int16) {
	// 1. 处理鼠标移动 (使用相对坐标 deltaX, deltaY)
	if deltaX != 0 || deltaY != 0 {
		if ic.controllerType == CONTROLLER_TYPE_WAYLAND {
			_ = ic.mouse.Move(deltaX, deltaY)
		} else {
			// X11 是相对坐标移动 (dstWindow 传 0 即为相对移动)
			xproto.WarpPointer(ic.conn, 0, 0, 0, 0, 0, 0, int16(deltaX), int16(deltaY))
		}
	}

	// 纯移动事件时尽早返回
	if action == MouseActionMove && wheelDeltaY == 0 && wheelDeltaX == 0 {
		return
	}

	// 2. 处理滚轮
	if wheelDeltaX != 0 || wheelDeltaY != 0 {
		if ic.controllerType == CONTROLLER_TYPE_WAYLAND {
			if wheelDeltaX != 0 {
				_ = ic.mouse.Wheel(true, int32(wheelDeltaX))
			}
			if wheelDeltaY != 0 {
				_ = ic.mouse.Wheel(false, int32(wheelDeltaY))
			}
		} else {
			// X11 下通过发送特定的 Button 模拟滚轮 (通常只支持垂直由于按钮数量限制)
			if wheelDeltaY != 0 {
				var button byte = MouseBtnWheelDown
				if wheelDeltaY < 0 {
					button = MouseBtnWheelUp
				}
				absDelta := wheelDeltaY
				if absDelta < 0 {
					absDelta = -absDelta
				}
				clicks := int(absDelta / WheelStep)
				if clicks <= 0 {
					clicks = 1
				}
				if clicks > 20 {
					clicks = 20
				}
				for i := 0; i < clicks; i++ {
					ic.triggerMouseButton(button, true)
					ic.triggerMouseButton(button, false)
				}
			}
		}
		return
	}

	// 3. 处理鼠标按键点击
	if action == MouseActionDown || action == MouseActionUp {
		isPress := action == MouseActionDown
		if buttons&WebBtnPrimary != 0 {
			ic.triggerMouseButton(MouseBtnLeft, isPress)
		}
		if buttons&WebBtnSecondary != 0 {
			ic.triggerMouseButton(MouseBtnRight, isPress)
		}
		if buttons&WebBtnTertiary != 0 {
			ic.triggerMouseButton(MouseBtnMiddle, isPress)
		}
	}
}

// HandleTouchEvent 处理触摸事件
func (ic *InputController) HandleTouchEvent(action byte, x, y int32) {
	// 绝对坐标移动
	if ic.controllerType == CONTROLLER_TYPE_WAYLAND {
		// 由于 uinput.Mouse 不能做绝对定位限制，如果是单纯鼠标设备暂无法代偿完美绝对触摸
		// 如果引入 uinput.TouchScreen，则可使用 _ = ic.touch.MoveTo(x, y) 等
		// 遵循如无必要勿增实体，这里先留出扩展口或者忽略，实际 Wayland 环境中可以拓展 Touch 设备逻辑
	} else {
		// X11 使用根窗口(ic.root)作为基准进行绝对坐标移动
		xproto.WarpPointer(ic.conn, 0, ic.root, 0, 0, 0, 0, int16(x), int16(y))
	}

	// 处理点击状态
	if action == TouchActionStart {
		ic.triggerMouseButton(MouseBtnLeft, true)
	} else if action == TouchActionEnd {
		ic.triggerMouseButton(MouseBtnLeft, false)
	}
}

// triggerMouseButton 屏蔽底层差异，执行点击动作
func (ic *InputController) triggerMouseButton(buttonID byte, isPress bool) {
	if ic.controllerType == CONTROLLER_TYPE_WAYLAND {
		switch buttonID {
		case MouseBtnLeft:
			if isPress {
				_ = ic.mouse.LeftPress()
			} else {
				_ = ic.mouse.LeftRelease()
			}
		case MouseBtnRight:
			if isPress {
				_ = ic.mouse.RightPress()
			} else {
				_ = ic.mouse.RightRelease()
			}
		case MouseBtnMiddle:
			if isPress {
				_ = ic.mouse.MiddlePress()
			} else {
				_ = ic.mouse.MiddleRelease()
			}
		}
	} else {
		// X11
		eventType := byte(xproto.ButtonRelease)
		if isPress {
			eventType = xproto.ButtonPress
		}
		xtest.FakeInput(ic.conn, eventType, buttonID, 0, ic.root, 0, 0, 0)
	}
}

// HandleKeyboardEvent 处理键盘事件
func (ic *InputController) HandleKeyboardEvent(action byte, keyCode int32) {
	if keyCode == 0 {
		return
	}

	isPress := action == KeyActionDown
	ic.updateModifierState(keyCode, isPress)

	if ic.controllerType == CONTROLLER_TYPE_WAYLAND {
		linuxKeyCode, ok := AndroidToLinuxEvdevMap[keyCode]
		if ok && linuxKeyCode > 0 {
			if isPress {
				_ = ic.keyboard.KeyPress(int(linuxKeyCode))
			} else {
				_ = ic.keyboard.KeyUp(int(linuxKeyCode))
			}
		}
	} else {
		x11Keycode, ok := AndroidToX11KeycodeMap[int(keyCode)]
		if !ok {
			x11Keycode = byte(keyCode) // Fallback
		}
		eventType := byte(xproto.KeyRelease)
		if isPress {
			eventType = xproto.KeyPress
		}
		xtest.FakeInput(ic.conn, eventType, x11Keycode, 0, ic.root, 0, 0, 0)
	}
}

// updateModifierState 处理常用的修饰键高层状态
func (ic *InputController) updateModifierState(androidKeyCode int32, isPress bool) {
	switch androidKeyCode {
	case 113, 114:
		ic.ctrlPressed = isPress
	case 59, 60:
		ic.shiftPressed = isPress
	case 57, 58:
		ic.altPressed = isPress
	case 117, 118:
		ic.metaPressed = isPress
	}
}

// GetModifierState 获取当前的高层修饰键状态
func (ic *InputController) GetModifierState() (ctrl, shift, alt, meta bool) {
	return ic.ctrlPressed, ic.shiftPressed, ic.altPressed, ic.metaPressed
}
