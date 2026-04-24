package main

import (
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

const WheelStep = 40

const (
	MouseActionMove = 0
	MouseActionDown = 1
	MouseActionUp   = 2
)

const (
	MouseBtnLeft      = 1
	MouseBtnMiddle    = 2
	MouseBtnRight     = 3
	MouseBtnWheelUp   = 4
	MouseBtnWheelDown = 5
)

const (
	WebBtnPrimary   uint32 = 1 << 0
	WebBtnSecondary uint32 = 1 << 1
	WebBtnTertiary  uint32 = 1 << 2
)

const (
	KeyActionDown = 0
	KeyActionUp   = 1
)

type InputController struct {
	conn *xgb.Conn
	root xproto.Window
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

	if action == MouseActionMove && wheelDeltaY == 0 && wheelDeltaX == 0 {
		return
	}

	if wheelDeltaY != 0 {
		ic.handleWheel(wheelDeltaY)
		return
	}

	if action == MouseActionDown || action == MouseActionUp {
		x11Btn := ic.mapWebBtnToX11(buttons)
		isPress := (action == MouseActionDown)
		ic.sendMouseInput(x11Btn, isPress)
	}
}

func (ic *InputController) HandleKeyboardEvent(action byte, keycode byte) {
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

	xtest.FakeInput(ic.conn, eventType, keycode, 0, ic.root, 0, 0, 0)
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
		button = MouseBtnWheelUp
	} else {
		button = MouseBtnWheelDown
	}

	absDelta := deltaY
	if absDelta < 0 {
		absDelta = -absDelta
	}

	clicks := int(absDelta / WheelStep)
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
	if buttons&WebBtnPrimary != 0 {
		return MouseBtnLeft
	}
	if buttons&WebBtnSecondary != 0 {
		return MouseBtnRight
	}
	if buttons&WebBtnTertiary != 0 {
		return MouseBtnMiddle
	}

	return MouseBtnLeft
}
