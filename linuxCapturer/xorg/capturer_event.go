package main

import (
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"github.com/jezek/xgb/xtest"
)

const WheelStep = 40

// Android keycode to X11 keycode mapping
// Android keycodes reference: https://developer.android.com/reference/android/view/KeyEvent
// X11 keycodes from standard US QWERTY keyboard (via xev)
var androidToX11KeycodeMap = map[int]byte{
	// Numbers (Android KEYCODE_0-9 = 7-16, X11 = 19,10-18)
	7:  19, // 0
	8:  10, // 1
	9:  11, // 2
	10: 12, // 3
	11: 13, // 4
	12: 14, // 5
	13: 15, // 6
	14: 16, // 7
	15: 17, // 8
	16: 18, // 9

	// Special keys
	67:  22, // DEL (Backspace) - IMPORTANT: Android DEL = 67, X11 = 22
	61:  23, // TAB
	62:  65, // SPACE
	66:  36, // ENTER (Return)
	111: 9,  // ESCAPE

	// Punctuation/Symbols
	69: 20, // MINUS (-)
	70: 21, // EQUALS (=)
	71: 34, // LEFT_BRACKET ([)
	72: 35, // RIGHT_BRACKET (])
	73: 51, // BACKSLASH (\)
	68: 49, // GRAVE (`)
	74: 47, // SEMICOLON (;)
	75: 48, // APOSTROPHE (')
	55: 59, // COMMA (,)
	56: 60, // PERIOD (.)
	76: 61, // SLASH (/)

	// Letters (Android KEYCODE_A-Z = 29-54)
	// Standard US QWERTY X11: Q(24) W(25) E(26) R(27) T(28) Y(29) U(30) I(31) O(32) P(33)
	//                         A(38) S(39) D(40) F(41) G(42) H(43) J(44) K(45) L(46)
	//                         Z(52) X(53) C(54) V(55) B(56) N(57) M(58)
	29: 38, // A
	30: 56, // B
	31: 54, // C
	32: 40, // D
	33: 26, // E
	34: 41, // F
	35: 42, // G
	36: 43, // H
	37: 31, // I
	38: 44, // J
	39: 45, // K
	40: 46, // L
	41: 58, // M
	42: 57, // N
	43: 32, // O
	44: 33, // P
	45: 24, // Q
	46: 27, // R
	47: 39, // S
	48: 28, // T
	49: 30, // U
	50: 55, // V
	51: 25, // W
	52: 53, // X
	53: 29, // Y
	54: 52, // Z

	// Modifier keys
	59:  50,  // SHIFT_LEFT
	60:  62,  // SHIFT_RIGHT
	57:  64,  // ALT_LEFT
	58:  108, // ALT_RIGHT
	113: 37,  // CTRL_LEFT
	114: 105, // CTRL_RIGHT
	117: 133, // META_LEFT (Windows key / Super)
	118: 134, // META_RIGHT

	// Function keys (Android KEYCODE_F1-F12 = 131-142, X11 = 67-78)
	131: 67, // F1
	132: 68, // F2
	133: 69, // F3
	134: 70, // F4
	135: 71, // F5
	136: 72, // F6
	137: 73, // F7
	138: 74, // F8
	139: 75, // F9
	140: 76, // F10
	141: 95, // F11
	142: 96, // F12

	// Lock keys
	115: 66, // CAPS_LOCK
	120: 78, // SCROLL_LOCK

	// Navigation keys
	19:  111, // DPAD_UP / UP
	20:  116, // DPAD_DOWN / DOWN
	21:  113, // DPAD_LEFT / LEFT
	22:  114, // DPAD_RIGHT / RIGHT
	92:  112, // PAGE_UP
	93:  117, // PAGE_DOWN
	122: 110, // MOVE_HOME / HOME
	123: 115, // MOVE_END / END
	124: 118, // INSERT
	112: 119, // FORWARD_DEL / DELETE

	// Media keys
	85: 172, // MEDIA_PLAY_PAUSE
	86: 174, // MEDIA_STOP
	87: 171, // MEDIA_NEXT
	88: 173, // MEDIA_PREVIOUS
	89: 169, // MEDIA_REWIND
	90: 175, // MEDIA_FAST_FORWARD
	91: 121, // MUTE

	// Volume keys
	24: 123, // VOLUME_UP
	25: 122, // VOLUME_DOWN

	// Special keys
	3:  110, // HOME
	4:  9,   // BACK (treated as ESC)
	26: 156, // POWER
	27: 212, // CAMERA
}

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

	// Convert Android keycode to X11 keycode
	x11Keycode, exists := androidToX11KeycodeMap[int(keycode)]
	if !exists {
		// Fallback: use keycode directly if no mapping exists
		x11Keycode = keycode
	}

	isPress := (action == KeyActionDown)

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
