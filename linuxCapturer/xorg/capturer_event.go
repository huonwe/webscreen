package main

import (
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"

	lc "webscreen/linuxCapturer"
)

type InputController struct {
	conn *xgb.Conn
	root xproto.Window

	// Track modifier key states for proper handling
	ctrlPressed  bool
	shiftPressed bool
	altPressed   bool
	metaPressed  bool
}

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

	return &InputController{conn: c, root: root}, nil
}

func (ic *InputController) Close() {
	if ic.conn != nil {
		ic.conn.Close()
	}
}

func (ic *InputController) HandleMouseEvent(action byte, x, y int16, buttons uint32, wheelDeltaX, wheelDeltaY int16) {
	ic.moveMouse(x, y)

	if action == lc.MouseActionMove && wheelDeltaY == 0 && wheelDeltaX == 0 {
		return
	}

	if wheelDeltaY != 0 {
		ic.handleWheel(wheelDeltaY)
		return
	}

	if action == lc.MouseActionDown || action == lc.MouseActionUp {
		x11Btn := ic.mapWebBtnToX11(buttons)
		isPress := (action == lc.MouseActionDown)
		ic.sendMouseInput(x11Btn, isPress)
	}
}

func (ic *InputController) HandleKeyboardEvent(action byte, keycode byte) {
	if keycode == 0 {
		return
	}

	// Convert Android keycode to X11 keycode
	x11Keycode, exists := lc.AndroidToX11KeycodeMap[int(keycode)]
	if !exists {
		// Fallback: use keycode directly if no mapping exists
		x11Keycode = keycode
	}

	isPress := (action == lc.KeyActionDown)

	var eventType byte
	if isPress {
		eventType = xproto.KeyPress
	} else {
		eventType = xproto.KeyRelease
	}

	// Track modifier key states
	ic.updateModifierState(int(keycode), isPress)

	xtest.FakeInput(ic.conn, eventType, x11Keycode, 0, ic.root, 0, 0, 0)
}

// updateModifierState tracks the state of modifier keys
func (ic *InputController) updateModifierState(androidKeycode int, isPress bool) {
	switch androidKeycode {
	case 113: // CTRL_LEFT
		ic.ctrlPressed = isPress
	case 114: // CTRL_RIGHT
		ic.ctrlPressed = isPress
	case 59: // SHIFT_LEFT
		ic.shiftPressed = isPress
	case 60: // SHIFT_RIGHT
		ic.shiftPressed = isPress
	case 57: // ALT_LEFT
		ic.altPressed = isPress
	case 58: // ALT_RIGHT
		ic.altPressed = isPress
	case 117: // META_LEFT
		ic.metaPressed = isPress
	case 118: // META_RIGHT
		ic.metaPressed = isPress
	}
}

// GetModifierState returns the current state of modifier keys
// Useful for checking if Ctrl+Key or Shift+Key combinations are active
func (ic *InputController) GetModifierState() (ctrl, shift, alt, meta bool) {
	return ic.ctrlPressed, ic.shiftPressed, ic.altPressed, ic.metaPressed
}

func (ic *InputController) moveMouse(x, y int16) {
	xproto.WarpPointer(ic.conn, xproto.Window(0), ic.root, 0, 0, 0, 0, x, y)
}

func (ic *InputController) sendMouseInput(button byte, isPress bool) {
	var eventType byte
	if isPress {
		eventType = xproto.ButtonPress
	} else {
		eventType = xproto.ButtonRelease
	}
	xtest.FakeInput(ic.conn, eventType, button, 0, ic.root, 0, 0, 0)
}

func (ic *InputController) handleWheel(deltaY int16) {
	if deltaY == 0 {
		return
	}

	var button byte
	if deltaY < 0 {
		button = lc.MouseBtnWheelUp
	} else {
		button = lc.MouseBtnWheelDown
	}

	absDelta := deltaY
	if absDelta < 0 {
		absDelta = -absDelta
	}

	clicks := int(absDelta / lc.WheelStep)
	if clicks == 0 {
		clicks = 1
	}
	if clicks > 20 {
		clicks = 20
	}

	for i := 0; i < clicks; i++ {
		ic.sendMouseInput(button, true)
		ic.sendMouseInput(button, false)
	}
}

func (ic *InputController) mapWebBtnToX11(buttons uint32) byte {
	if buttons&lc.WebBtnPrimary != 0 {
		return lc.MouseBtnLeft
	}
	if buttons&lc.WebBtnSecondary != 0 {
		return lc.MouseBtnRight
	}
	if buttons&lc.WebBtnTertiary != 0 {
		return lc.MouseBtnMiddle
	}

	return lc.MouseBtnLeft
}
