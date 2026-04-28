package main

import "github.com/bendahl/uinput"

var AndroidToLinuxEvdevMap = map[int32]int32{
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

// Android keycode to X11 keycode mapping
// Android keycodes reference: https://developer.android.com/reference/android/view/KeyEvent
// X11 keycodes from standard US QWERTY keyboard (via xev)
var AndroidToX11KeycodeMap = map[int]byte{
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
