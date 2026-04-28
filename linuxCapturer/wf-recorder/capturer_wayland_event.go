package main

import (
	"log"

	"github.com/bendahl/uinput"
)

var androidToLinuxEvdevMap = map[int32]int32{
	// 字母
	29: uinput.KeyA, 30: uinput.KeyB, 31: uinput.KeyC, 32: uinput.KeyD,
	33: uinput.KeyE, 34: uinput.KeyF, 35: uinput.KeyG, 36: uinput.KeyH,
	37: uinput.KeyI, 38: uinput.KeyJ, 39: uinput.KeyK, 40: uinput.KeyL,
	41: uinput.KeyM, 42: uinput.KeyN, 43: uinput.KeyO, 44: uinput.KeyP,
	45: uinput.KeyQ, 46: uinput.KeyR, 47: uinput.KeyS, 48: uinput.KeyT,
	49: uinput.KeyU, 50: uinput.KeyV, 51: uinput.KeyW, 52: uinput.KeyX,
	53: uinput.KeyY, 54: uinput.KeyZ,

	// 数字
	7: uinput.Key0, 8: uinput.Key1, 9: uinput.Key2, 10: uinput.Key3,
	11: uinput.Key4, 12: uinput.Key5, 13: uinput.Key6, 14: uinput.Key7,
	15: uinput.Key8, 16: uinput.Key9,

	// 常用控制键
	66:  uinput.KeyEnter,
	67:  uinput.KeyBackspace,
	112: uinput.KeyDelete,
	111: uinput.KeyEsc,
	62:  uinput.KeySpace,
	61:  uinput.KeyTab,
	69:  uinput.KeyMinus,
	70:  uinput.KeyEqual,
	71:  uinput.KeyLeftbrace,
	72:  uinput.KeyRightbrace,
	73:  uinput.KeyBackslash,
	74:  uinput.KeySemicolon,
	75:  uinput.KeyApostrophe,
	68:  uinput.KeyGrave,
	55:  uinput.KeyComma,
	56:  uinput.KeyDot,
	76:  uinput.KeySlash,

	// 修饰键
	59:  uinput.KeyLeftshift,
	60:  uinput.KeyRightshift,
	113: uinput.KeyLeftctrl,
	114: uinput.KeyRightctrl,
	57:  uinput.KeyLeftalt,
	58:  uinput.KeyRightalt,

	// 方向键
	19: uinput.KeyUp,
	20: uinput.KeyDown,
	21: uinput.KeyLeft,
	22: uinput.KeyRight,
}

type InputControllerWayland struct {
	keyboard  uinput.Keyboard
	mouse     uinput.Mouse
	lastX     int32
	lastY     int32
	initPos   bool
	eventChan chan inputEvent
	done      chan struct{}

	ctrlPressed  bool
	shiftPressed bool
	altPressed   bool
	metaPressed  bool
}

type inputEvent struct {
	eventType int // 0=mouse, 1=keyboard
	mouseData *mouseEventData
	keyData   *keyEventData
}

type mouseEventData struct {
	action      byte
	x, y        int32
	buttons     uint32
	wheelDeltaX int16
	wheelDeltaY int16
}

type keyEventData struct {
	action  byte
	keyCode int32
}

func NewInputControllerWayland(width, height int32) (*InputControllerWayland, error) {
	// 尝试创建虚拟键盘
	kb, err := uinput.CreateKeyboard("/dev/uinput", []byte("webscreen_keyboard"))
	if err != nil {
		return nil, err
	}

	// 尝试创建绝对坐标的虚拟触摸板(代替鼠标)
	m, err := uinput.CreateMouse("/dev/uinput", []byte("webscreen_mouse"))
	if err != nil {
		kb.Close()
		return nil, err
	}

	ic := &InputControllerWayland{
		keyboard:  kb,
		mouse:     m,
		eventChan: make(chan inputEvent, 64),
		done:      make(chan struct{}),
	}

	// 启动异步事件处理 goroutine
	go ic.eventWorker()

	return ic, nil
}

func (ic *InputControllerWayland) Close() {
	close(ic.done)
	if ic.keyboard != nil {
		ic.keyboard.Close()
	}
	if ic.mouse != nil {
		ic.mouse.Close()
	}
}

// eventWorker 异步处理键盘鼠标事件，避免阻塞主事件循环
func (ic *InputControllerWayland) eventWorker() {
	for {
		select {
		case <-ic.done:
			return
		case evt := <-ic.eventChan:
			if evt.eventType == 0 && evt.mouseData != nil {
				ic.processMouseEventSync(evt.mouseData)
			} else if evt.eventType == 1 && evt.keyData != nil {
				ic.processKeyEventSync(evt.keyData)
			}
		}
	}
}

func (ic *InputControllerWayland) processMouseEventSync(data *mouseEventData) {
	var deltaX int32
	var deltaY int32
	if !ic.initPos {
		ic.lastX = data.x
		ic.lastY = data.y
		ic.initPos = true
	} else {
		deltaX = data.x - ic.lastX
		deltaY = data.y - ic.lastY
		ic.lastX = data.x
		ic.lastY = data.y
	}

	if deltaX != 0 || deltaY != 0 {
		if err := ic.mouse.Move(deltaX, deltaY); err != nil {
			log.Printf("mouse move failed: %v", err)
		}
	}

	if data.wheelDeltaX != 0 {
		if err := ic.mouse.Wheel(true, int32(data.wheelDeltaX)); err != nil {
			log.Printf("mouse horizontal wheel failed: %v", err)
		}
	}
	if data.wheelDeltaY != 0 {
		if err := ic.mouse.Wheel(false, int32(data.wheelDeltaY)); err != nil {
			log.Printf("mouse wheel failed: %v", err)
		}
	}

	if (data.buttons & WebBtnPrimary) != 0 {
		if data.action == MouseActionDown {
			if err := ic.mouse.LeftPress(); err != nil {
				log.Printf("mouse left press failed: %v", err)
			}
		} else if data.action == MouseActionUp {
			if err := ic.mouse.LeftRelease(); err != nil {
				log.Printf("mouse left release failed: %v", err)
			}
		}
	}
	if (data.buttons & WebBtnSecondary) != 0 {
		if data.action == MouseActionDown {
			if err := ic.mouse.RightPress(); err != nil {
				log.Printf("mouse right press failed: %v", err)
			}
		} else if data.action == MouseActionUp {
			if err := ic.mouse.RightRelease(); err != nil {
				log.Printf("mouse right release failed: %v", err)
			}
		}
	}
	if (data.buttons & WebBtnTertiary) != 0 {
		if data.action == MouseActionDown {
			if err := ic.mouse.MiddlePress(); err != nil {
				log.Printf("mouse middle press failed: %v", err)
			}
		} else if data.action == MouseActionUp {
			if err := ic.mouse.MiddleRelease(); err != nil {
				log.Printf("mouse middle release failed: %v", err)
			}
		}
	}
}

func (ic *InputControllerWayland) processKeyEventSync(data *keyEventData) {
	linuxKeyCode := mapAndroidToLinuxEvdev(data.keyCode)
	if linuxKeyCode <= 0 {
		return
	}

	ic.updateModifierState(data.keyCode, data.action == KeyActionDown)

	if data.action == KeyActionDown {
		ic.keyboard.KeyPress(int(linuxKeyCode))
	} else if data.action == KeyActionUp {
		ic.keyboard.KeyUp(int(linuxKeyCode))
	}
}

// HandleMouseEvent 异步提交鼠标事件到处理队列
func (ic *InputControllerWayland) HandleMouseEvent(action byte, x, y int32, buttons uint32, wheelDeltaX, wheelDeltaY int16) {
	select {
	case ic.eventChan <- inputEvent{
		eventType: 0,
		mouseData: &mouseEventData{
			action:      action,
			x:           x,
			y:           y,
			buttons:     buttons,
			wheelDeltaX: wheelDeltaX,
			wheelDeltaY: wheelDeltaY,
		},
	}:
	default:
		log.Printf("mouse event queue full, dropping event")
	}
}

// HandleKeyboardEvent 异步提交键盘事件到处理队列
func (ic *InputControllerWayland) HandleKeyboardEvent(action byte, keyCode int32) {
	select {
	case ic.eventChan <- inputEvent{
		eventType: 1,
		keyData: &keyEventData{
			action:  action,
			keyCode: keyCode,
		},
	}:
	default:
		log.Printf("keyboard event queue full, dropping event (keyCode=%d, action=%d)", keyCode, action)
	}
}

func (ic *InputControllerWayland) updateModifierState(androidKeyCode int32, isPress bool) {
	switch androidKeyCode {
	case 59, 60:
		ic.shiftPressed = isPress
	case 57, 58:
		ic.altPressed = isPress
	case 113, 114:
		ic.ctrlPressed = isPress
	case 117, 118:
		ic.metaPressed = isPress
	}
}

func (ic *InputControllerWayland) GetModifierState() (ctrl, shift, alt, meta bool) {
	return ic.ctrlPressed, ic.shiftPressed, ic.altPressed, ic.metaPressed
}

// Android KeyCode 可以在 keyboard.js 中找到
func mapAndroidToLinuxEvdev(androidKeyCode int32) int32 {
	if v, ok := androidToLinuxEvdevMap[androidKeyCode]; ok {
		return v
	}
	return -1
}
