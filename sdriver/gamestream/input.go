package gamestream

import (
	"encoding/binary"
	"math"
	"webscreen/sdriver"
)

const (
	magicKeyDown           uint32 = 0x00000003
	magicKeyUp             uint32 = 0x00000004
	magicMouseAbs          uint32 = 0x00000005
	magicMouseRel          uint32 = 0x00000007
	magicMouseBtnDown      uint32 = 0x00000008
	magicMouseBtnUp        uint32 = 0x00000009
	magicScroll            uint32 = 0x0000000A
	magicMultiController   uint32 = 0x0000000C
	magicHScroll           uint32 = 0x55000001
	magicTouch             uint32 = 0x55000002
	magicControllerArrival uint32 = 0x55000004

	ctrlChannelGamepadBase uint8 = 0x10

	modShift byte = 0x01
	modCtrl  byte = 0x02
	modAlt   byte = 0x04
	modMeta  byte = 0x08
)

func (d *Driver) SendEvent(event sdriver.Event) error {
	if d.ctrl == nil {
		return nil
	}

	switch e := event.(type) {
	case *sdriver.KeyEvent:
		return d.sendKeyEvent(e)
	case sdriver.KeyEvent:
		return d.sendKeyEvent(&e)
	case *sdriver.MouseEvent:
		return d.sendMouseEvent(e)
	case sdriver.MouseEvent:
		return d.sendMouseEvent(&e)
	case *sdriver.ScrollEvent:
		return d.sendScrollEvent(e)
	case sdriver.ScrollEvent:
		return d.sendScrollEvent(&e)
	case *sdriver.TouchEvent:
		return d.sendTouchEvent(e)
	case sdriver.TouchEvent:
		return d.sendTouchEvent(&e)
	case *sdriver.GamepadEvent:
		return d.sendGamepadEvent(e)
	case sdriver.GamepadEvent:
		return d.sendGamepadEvent(&e)
	case *sdriver.IDRReqEvent, sdriver.IDRReqEvent:
		d.RequestIDR(true)
		return nil
	default:
		return nil
	}
}

func (d *Driver) sendKeyEvent(e *sdriver.KeyEvent) error {
	vk := androidKeyToVK(e.KeyCode)
	if vk == 0 {
		return nil
	}
	magic := magicKeyDown
	if e.Action != 0 {
		magic = magicKeyUp
	}

	d.inputMu.Lock()
	d.updateModifierState(vk, e.Action == 0)
	mods := d.modifiers
	d.inputMu.Unlock()

	// NV_KEYBOARD_PACKET: size(4 BE) + magic(4 LE) + flags(1) + keyCode(2 LE) + modifiers(1) + zero2(2) = 14 bytes
	pkt := make([]byte, 14)
	binary.BigEndian.PutUint32(pkt[0:4], 10)
	binary.LittleEndian.PutUint32(pkt[4:8], magic)
	pkt[8] = 0
	binary.LittleEndian.PutUint16(pkt[9:11], uint16(0x8000|vk))
	pkt[11] = mods
	return d.ctrl.send(ctrlTypeInputData, pkt, ctrlChannelKey, true)
}

func (d *Driver) updateModifierState(vk uint16, down bool) {
	var bit byte
	switch vk {
	case 0x10, 0xA0, 0xA1:
		bit = modShift
	case 0x11, 0xA2, 0xA3:
		bit = modCtrl
	case 0x12, 0xA4, 0xA5:
		bit = modAlt
	case 0x5B, 0x5C:
		bit = modMeta
	}
	if bit == 0 {
		return
	}
	if down {
		d.modifiers |= bit
	} else {
		d.modifiers &^= bit
	}
}

func (d *Driver) sendMouseEvent(e *sdriver.MouseEvent) error {
	dx := int16(int32(e.PosX))
	dy := int16(int32(e.PosY))
	if dx != 0 || dy != 0 {
		// NV_REL_MOUSE_MOVE_PACKET: size(4 BE=8) + magic(4 LE=0x07) + deltaX(2 BE) + deltaY(2 BE) = 12 bytes
		pkt := make([]byte, 12)
		binary.BigEndian.PutUint32(pkt[0:4], 8)
		binary.LittleEndian.PutUint32(pkt[4:8], magicMouseRel)
		binary.BigEndian.PutUint16(pkt[8:10], uint16(dx))
		binary.BigEndian.PutUint16(pkt[10:12], uint16(dy))
		if err := d.ctrl.send(ctrlTypeInputData, pkt, ctrlChannelMouse, true); err != nil {
			return err
		}
	}

	// Button down/up
	if e.Action == 0 || e.Action == 1 {
		magic := magicMouseBtnDown
		if e.Action == 1 {
			magic = magicMouseBtnUp
		}
		if btn := mapMouseButton(e.Buttons); btn != 0 {
			bp := make([]byte, 9) // size(4) + magic(4) + button(1)
			binary.BigEndian.PutUint32(bp[0:4], 5)
			binary.LittleEndian.PutUint32(bp[4:8], magic)
			bp[8] = btn
			if err := d.ctrl.send(ctrlTypeInputData, bp, ctrlChannelMouse, true); err != nil {
				return err
			}
		}
	}

	if e.WheelDeltaY != 0 {
		if err := d.sendVScroll(e.WheelDeltaY * 120); err != nil {
			return err
		}
	}
	if e.WheelDeltaX != 0 {
		if err := d.sendHScroll(e.WheelDeltaX * 120); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) sendScrollEvent(e *sdriver.ScrollEvent) error {
	if int16(e.VScroll) != 0 {
		if err := d.sendVScroll(int16(e.VScroll) * 120); err != nil {
			return err
		}
	}
	if int16(e.HScroll) != 0 {
		if err := d.sendHScroll(int16(e.HScroll) * 120); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) sendVScroll(amt int16) error {
	pkt := make([]byte, 14) // size(4) + magic(4) + amt1(2) + amt2(2) + zero(2)
	binary.BigEndian.PutUint32(pkt[0:4], 10)
	binary.LittleEndian.PutUint32(pkt[4:8], magicScroll)
	binary.BigEndian.PutUint16(pkt[8:10], uint16(amt))
	binary.BigEndian.PutUint16(pkt[10:12], uint16(amt))
	return d.ctrl.send(ctrlTypeInputData, pkt, ctrlChannelMouse, true)
}

func (d *Driver) sendHScroll(amt int16) error {
	pkt := make([]byte, 10) // size(4) + magic(4) + scrollAmount(2)
	binary.BigEndian.PutUint32(pkt[0:4], 6)
	binary.LittleEndian.PutUint32(pkt[4:8], magicHScroll)
	binary.BigEndian.PutUint16(pkt[8:10], uint16(amt))
	return d.ctrl.send(ctrlTypeInputData, pkt, ctrlChannelMouse, true)
}

func (d *Driver) sendTouchEvent(e *sdriver.TouchEvent) error {
	var evType byte = 0x03 // LI_TOUCH_EVENT_MOVE
	switch e.Action {
	case sdriver.TOUCH_ACTION_DOWN:
		evType = 0x01 // LI_TOUCH_EVENT_DOWN
	case sdriver.TOUCH_ACTION_UP:
		evType = 0x02 // LI_TOUCH_EVENT_UP
	}
	w := float32(e.Width)
	if w <= 0 {
		w = float32(d.meta.Width)
	}
	h := float32(e.Height)
	if h <= 0 {
		h = float32(d.meta.Height)
	}
	nx := float32(e.PosX) / w
	ny := float32(e.PosY) / h

	// SS_TOUCH_PACKET: size(4) + magic(4) + eventType(1) + zero(1) + rotation(2) + pointerId(4) + x(4) + y(4) + pressureOrDistance(4) + contactAreaMajor(4) + contactAreaMinor(4) = 36 bytes
	pkt := make([]byte, 36)
	binary.BigEndian.PutUint32(pkt[0:4], 32)
	binary.LittleEndian.PutUint32(pkt[4:8], magicTouch)
	pkt[8] = evType
	binary.LittleEndian.PutUint16(pkt[10:12], 0xFFFF)
	binary.LittleEndian.PutUint32(pkt[12:16], uint32(e.PointerID))
	binary.LittleEndian.PutUint32(pkt[16:20], math.Float32bits(nx))
	binary.LittleEndian.PutUint32(pkt[20:24], math.Float32bits(ny))
	binary.LittleEndian.PutUint32(pkt[24:28], math.Float32bits(float32(e.Pressure)/65535.0))
	return d.ctrl.send(ctrlTypeInputData, pkt, ctrlChannelTouch, true)
}

func (d *Driver) sendGamepadEvent(e *sdriver.GamepadEvent) error {
	num := e.ControllerNumber & 0x03
	ch := ctrlChannelGamepadBase + num

	d.inputMu.Lock()
	needArrival := !d.padArrived[num] && (e.ActiveGamepadMask&(1<<num)) != 0
	if needArrival {
		d.padArrived[num] = true
	}
	if (e.ActiveGamepadMask & (1 << num)) == 0 {
		d.padArrived[num] = false
	}
	d.inputMu.Unlock()

	if needArrival {
		// SS_CONTROLLER_ARRIVAL_PACKET (16 bytes): size(4 BE=12) + magic(4 LE) + controllerNumber(1) + type(1=Xbox 0x01) + capabilities(2 LE=0x03) + supportedButtonFlags(4 LE)
		arr := make([]byte, 16)
		binary.BigEndian.PutUint32(arr[0:4], 12)
		binary.LittleEndian.PutUint32(arr[4:8], magicControllerArrival)
		arr[8] = num
		arr[9] = 0x01 // Xbox controller
		binary.LittleEndian.PutUint16(arr[10:12], 0x0003)
		binary.LittleEndian.PutUint32(arr[12:16], 0x0000FFFF)
		if err := d.ctrl.send(ctrlTypeInputData, arr, ch, true); err != nil {
			return err
		}
	}

	// NV_MULTI_CONTROLLER_PACKET (34 bytes: size=30 + 4-byte size prefix)
	pkt := make([]byte, 34)
	binary.BigEndian.PutUint32(pkt[0:4], 30)
	binary.LittleEndian.PutUint32(pkt[4:8], magicMultiController)
	binary.LittleEndian.PutUint16(pkt[8:10], 0x001A) // MC_HEADER_B
	binary.LittleEndian.PutUint16(pkt[10:12], uint16(num))
	binary.LittleEndian.PutUint16(pkt[12:14], e.ActiveGamepadMask)
	binary.LittleEndian.PutUint16(pkt[14:16], 0x0014) // MC_MID_B
	binary.LittleEndian.PutUint16(pkt[16:18], uint16(e.Buttons&0xFFFF))
	pkt[18] = e.LeftTrigger
	pkt[19] = e.RightTrigger
	binary.LittleEndian.PutUint16(pkt[20:22], uint16(e.LeftStickX))
	binary.LittleEndian.PutUint16(pkt[22:24], uint16(e.LeftStickY))
	binary.LittleEndian.PutUint16(pkt[24:26], uint16(e.RightStickX))
	binary.LittleEndian.PutUint16(pkt[26:28], uint16(e.RightStickY))
	binary.LittleEndian.PutUint16(pkt[28:30], 0x009C) // MC_TAIL_A
	binary.LittleEndian.PutUint16(pkt[30:32], uint16(e.Buttons>>16))
	binary.LittleEndian.PutUint16(pkt[32:34], 0x0055) // MC_TAIL_B
	return d.ctrl.send(ctrlTypeInputData, pkt, ch, true)
}

func mapMouseButton(buttons uint32) byte {
	switch {
	case buttons&sdriver.BUTTON_PRIMARY != 0:
		return 1
	case buttons&sdriver.BUTTON_TERTIARY != 0:
		return 2
	case buttons&sdriver.BUTTON_SECONDARY != 0:
		return 3
	default:
		return 0
	}
}

func androidKeyToVK(code uint32) uint16 {
	// Letters A-Z: Android 29..54 -> VK 0x41..0x5A
	if code >= 29 && code <= 54 {
		return uint16('A' + (code - 29))
	}
	// Digits 0-9: Android 7..16 -> VK 0x30..0x39
	if code >= 7 && code <= 16 {
		return uint16('0' + (code - 7))
	}
	// F1-F12: Android 131..142 -> VK 0x70..0x7B
	if code >= 131 && code <= 142 {
		return uint16(0x70 + (code - 131))
	}
	switch code {
	case 66:
		return 0x0D // Enter
	case 67:
		return 0x08 // Backspace
	case 61:
		return 0x09 // Tab
	case 62:
		return 0x20 // Space
	case 111:
		return 0x1B // Escape
	case 112:
		return 0x2E // Delete
	case 124:
		return 0x2D // Insert
	case 122:
		return 0x24 // Home
	case 123:
		return 0x23 // End
	case 92:
		return 0x21 // PageUp
	case 93:
		return 0x22 // PageDown
	case 19:
		return 0x26 // Up
	case 20:
		return 0x28 // Down
	case 21:
		return 0x25 // Left
	case 22:
		return 0x27 // Right
	case 59:
		return 0xA0 // LShift
	case 60:
		return 0xA1 // RShift
	case 113:
		return 0xA2 // LCtrl
	case 114:
		return 0xA3 // RCtrl
	case 57:
		return 0xA4 // LAlt
	case 58:
		return 0xA5 // RAlt
	case 117:
		return 0x5B // LWin
	case 118:
		return 0x5C // RWin
	case 115:
		return 0x14 // CapsLock
	case 68:
		return 0xC0 // Grave
	case 69:
		return 0xBD // Minus
	case 70:
		return 0xBB // Equals
	case 71:
		return 0xDB // LBracket
	case 72:
		return 0xDD // RBracket
	case 73:
		return 0xDC // Backslash
	case 74:
		return 0xBA // Semicolon
	case 75:
		return 0xDE // Apostrophe
	case 55:
		return 0xBC // Comma
	case 56:
		return 0xBE // Period
	case 76:
		return 0xBF // Slash
	default:
		return 0
	}
}
