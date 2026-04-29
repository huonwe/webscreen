package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/bendahl/uinput"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

// InputController 负责处理多后端的底层输入操作
type InputController struct {
	controllerType string

	// ========== uinput (Wayland) 相关成员 ==========
	keyboard uinput.Keyboard
	mouse    uinput.Mouse
	touch    uinput.MultiTouch

	// ========== X11 (xtest) 相关成员 ==========
	conn *xgb.Conn
	root xproto.Window

	screenWidth  uint16
	screenHeight uint16
}

// NewInputController 初始化输入控制器
func NewInputController(controllerType string, display string, screenWidth uint16, screenHeight uint16) (*InputController, error) {
	ic := &InputController{
		controllerType: controllerType,
		screenWidth:    screenWidth,
		screenHeight:   screenHeight,
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
		if geo, err := xproto.GetGeometry(c, xproto.Drawable(ic.root)).Reply(); err == nil {
			ic.screenWidth = uint16(geo.Width)
			ic.screenHeight = uint16(geo.Height)
		}

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
		touch, err := uinput.CreateMultiTouch(
			"/dev/uinput",
			[]byte("webscreen_touch"),
			int32(0),
			int32(screenWidth),
			int32(0),
			int32(screenHeight),
			10, // max slots
		)
		if err != nil {
			kb.Close()
			m.Close()
			return nil, err
		}
		ic.keyboard = kb
		ic.mouse = m
		ic.touch = touch

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
	if ic.touch != nil {
		ic.touch.Close()
	}
	if ic.conn != nil {
		ic.conn.Close()
	}
}

func (ic *InputController) ServeControlConn(conn net.Conn) error {
	head := make([]byte, 1)
	buff := make([]byte, 256) // 足够大以容纳任何事件的完整数据包
	for {
		_, err := io.ReadFull(conn, head)
		if err != nil {
			return fmt.Errorf("control connection error: %w", err)
		}
		eventType := EventType(head[0])
		// log.Printf("Received event type: 0x%X", head[0])
		switch eventType {
		case EventTypeKeyboard:
			if _, err := io.ReadFull(conn, buff[:5]); err != nil {
				return fmt.Errorf("failed to read keyboard event payload: %w", err)
			}
			event, err := ParseKeyboardEvent(buff[:5])
			if err != nil {
				return fmt.Errorf("failed to parse keyboard event: %w", err)
			}
			ic.HandleKeyboardEvent(event.(*KeyboardEvent).action, event.(*KeyboardEvent).keyCode)

		case EventTypeMouse:
			if _, err := io.ReadFull(conn, buff[:17]); err != nil {
				return fmt.Errorf("failed to read mouse event payload: %w", err)
			}
			event, err := ParseMouseEvent(buff[:17])
			if err != nil {
				return fmt.Errorf("failed to parse mouse event: %w", err)
			}
			me := event.(*MouseEvent)
			ic.HandleMouseEvent(me.action, me.deltaX, me.deltaY, me.buttons, me.wheelDeltaX, me.wheelDeltaY)

		case EventTypeTouch:
			if _, err := io.ReadFull(conn, buff[:9]); err != nil {
				return fmt.Errorf("failed to read touch event payload: %w", err)
			}
			event, err := ParseTouchEvent(buff[:9])
			if err != nil {
				return fmt.Errorf("failed to parse touch event: %w", err)
			}
			te := event.(*TouchEvent)
			ic.HandleTouchEvent(te.action, te.ptrID, te.x, te.y, te.pressure, te.buttons)

		default:
			return fmt.Errorf("unknown event type: 0x%X", head[0])
			// log.Printf("收到未知事件类型: 0x%X", head[0])
		}
	}
}

// HandleMouseEvent 处理鼠标事件并分发到对应底层接口
func (ic *InputController) HandleMouseEvent(action byte, deltaX, deltaY int32, buttons uint32, wheelDeltaX, wheelDeltaY int16) {
	// 1. 处理鼠标移动 (使用相对坐标 deltaX, deltaY)
	// log.Printf("Mouse Event - Action: %d, DeltaX: %d, DeltaY: %d", action, deltaX, deltaY)
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

// 映射公式: (当前坐标 / 屏幕最大值) * uinput最大值
func scaleCoord(val uint16, screenMax uint16, uinputMax int32) int32 {
	if screenMax == 0 {
		return 0
	}
	return int32(uint32(val) * uint32(uinputMax) / uint32(screenMax))
}

// HandleTouchEvent 处理触摸事件（协议与 touch.js 保持一致）
func (ic *InputController) HandleTouchEvent(action, ptrID byte, x, y, pressure uint16, buttons byte) {
	_ = ptrID
	_ = pressure

	// log.Printf("Touch Event - Action: %d, PtrID: %d, X: %d, Y: %d", action, ptrID, x, y)

	// 传入的 x/y 为屏幕坐标
	if ic.controllerType == CONTROLLER_TYPE_WAYLAND {
		if ic.touch == nil {
			return
		}

		finger := ic.touch.GetContacts()[ptrID%10] // 取模以防 ptrID 超出范围

		switch action {
		case TouchActionDown, TouchActionMove:
			finger.TouchDownAt(int32(x), int32(y))
		case TouchActionUp:
			finger.TouchUp()
		}

		return
	} else {
		// X11: 将归一化坐标映射到根窗口绝对坐标
		// absX := scaleNormalizedToRange(x, ic.rootWidth)
		// absY := scaleNormalizedToRange(y, ic.rootHeight)
		xproto.WarpPointer(ic.conn, 0, ic.root, 0, 0, 0, 0, int16(x), int16(y))

		// X11/XTest 原生态不支持多点触控（由于X11核心协议是在触摸屏流行之前设计的）。
		// 在这里我们将单点触控降级为鼠标操作来实现基本的“点击”和“滑动”：
		switch action {
		case TouchActionDown:
			ic.triggerMouseButton(MouseBtnLeft, true)
		case TouchActionUp:
			ic.triggerMouseButton(MouseBtnLeft, false)
		}
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

func ParseKeyboardEvent(payload []byte) (event Event, err error) {
	if len(payload) != 5 {
		return nil, fmt.Errorf("invalid keyboard event payload length: %d", len(payload))
	}
	action := payload[0]
	keyCode := int32(binary.BigEndian.Uint32(payload[1:5]))
	return &KeyboardEvent{
		action:  action,
		keyCode: keyCode,
	}, nil
}

func ParseMouseEvent(payload []byte) (event Event, err error) {
	if len(payload) != 17 {
		return nil, fmt.Errorf("invalid mouse event payload length: %d", len(payload))
	}
	action := payload[0]
	deltaX := int32(binary.BigEndian.Uint32(payload[1:5]))
	deltaY := int32(binary.BigEndian.Uint32(payload[5:9]))
	buttons := binary.BigEndian.Uint32(payload[9:13])
	wheelDeltaX := int16(binary.BigEndian.Uint16(payload[13:15]))
	wheelDeltaY := int16(binary.BigEndian.Uint16(payload[15:]))
	return &MouseEvent{
		action:      action,
		x:           0,
		y:           0,
		buttons:     buttons,
		deltaX:      deltaX,
		deltaY:      deltaY,
		wheelDeltaX: wheelDeltaX,
		wheelDeltaY: wheelDeltaY,
	}, nil
}

func ParseTouchEvent(payload []byte) (event Event, err error) {
	if len(payload) != 9 {
		return nil, fmt.Errorf("invalid touch event payload length: %d", len(payload))
	}
	action := payload[0]
	ptrID := payload[1]
	x := binary.BigEndian.Uint16(payload[2:4])
	y := binary.BigEndian.Uint16(payload[4:6])
	pressure := binary.BigEndian.Uint16(payload[6:8])
	buttons := payload[8]
	return &TouchEvent{
		action:   action,
		ptrID:    ptrID,
		x:        x,
		y:        y,
		pressure: pressure,
		buttons:  buttons,
	}, nil
}
