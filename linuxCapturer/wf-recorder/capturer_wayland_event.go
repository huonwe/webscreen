package main

import (
	"log"
	lc "webscreen/linuxCapturer"

	"github.com/bendahl/uinput"
)

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

	if (data.buttons & lc.WebBtnPrimary) != 0 {
		if data.action == lc.MouseActionDown {
			if err := ic.mouse.LeftPress(); err != nil {
				log.Printf("mouse left press failed: %v", err)
			}
		} else if data.action == lc.MouseActionUp {
			if err := ic.mouse.LeftRelease(); err != nil {
				log.Printf("mouse left release failed: %v", err)
			}
		}
	}
	if (data.buttons & lc.WebBtnSecondary) != 0 {
		if data.action == lc.MouseActionDown {
			if err := ic.mouse.RightPress(); err != nil {
				log.Printf("mouse right press failed: %v", err)
			}
		} else if data.action == lc.MouseActionUp {
			if err := ic.mouse.RightRelease(); err != nil {
				log.Printf("mouse right release failed: %v", err)
			}
		}
	}
	if (data.buttons & lc.WebBtnTertiary) != 0 {
		if data.action == lc.MouseActionDown {
			if err := ic.mouse.MiddlePress(); err != nil {
				log.Printf("mouse middle press failed: %v", err)
			}
		} else if data.action == lc.MouseActionUp {
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

	ic.updateModifierState(data.keyCode, data.action == lc.KeyActionDown)

	if data.action == lc.KeyActionDown {
		ic.keyboard.KeyPress(int(linuxKeyCode))
	} else if data.action == lc.KeyActionUp {
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
	if v, ok := lc.AndroidToLinuxEvdevMap[androidKeyCode]; ok {
		return v
	}
	return -1
}
