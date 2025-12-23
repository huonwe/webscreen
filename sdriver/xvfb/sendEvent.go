package linuxXvfbDriver

import (
	"bytes"
	"encoding/binary"
	"webscreen/sdriver"
)

func (d *LinuxDriver) SendEvent(event sdriver.Event) error {
	// log.Printf("X11Driver: Sending event type %T", event)
	buf := new(bytes.Buffer)

	// 常量定义 (需确保与 sdriver 包一致，或直接使用字面量)
	const (
		PacketTypeKey   = 0x00
		PacketTypeMouse = 0x01
	)

	switch v := event.(type) {

	case *sdriver.MouseEvent:
		buf.WriteByte(PacketTypeMouse) // Header: 0x01

		// Payload
		buf.WriteByte(v.Action)                        // [0] Action
		binary.Write(buf, binary.BigEndian, v.PosX)    // [1-4] X (或 DeltaX)
		binary.Write(buf, binary.BigEndian, v.PosY)    // [5-8] Y (或 DeltaY)
		binary.Write(buf, binary.BigEndian, v.Buttons) // [9-12] Buttons

		// 填充滚轮数据 (对应 TouchEvent 里的 int16(0))
		// 注意：需确保结构体里是 int16，或者在这里强转
		binary.Write(buf, binary.BigEndian, int16(v.WheelDeltaX)) // [13-14] Wheel X
		binary.Write(buf, binary.BigEndian, int16(v.WheelDeltaY)) // [15-16] Wheel Y
	// =================================================================
	// Case 1: 触摸事件 -> 鼠标包
	// 接收端 Payload 长度: 16 bytes
	// 结构: [Action 1][X 4][Y 4][Btn 4][Wheel X][Wheel Y]
	// =================================================================
	case *sdriver.TouchEvent:
		buf.WriteByte(PacketTypeMouse) // Header: 0x01

		// Payload (16 bytes)
		buf.WriteByte(v.Action)                        // [0] Action
		binary.Write(buf, binary.BigEndian, v.PosX)    // [1-4] X
		binary.Write(buf, binary.BigEndian, v.PosY)    // [5-8] Y
		binary.Write(buf, binary.BigEndian, v.Buttons) // [9-12] Buttons
		binary.Write(buf, binary.BigEndian, int16(0))  // [13-14] Wheel (触摸无滚轮)
		binary.Write(buf, binary.BigEndian, int16(0))  // [15-16] Padding (关键！补齐第16字节)

	// =================================================================
	// Case 2: 滚动事件 -> 鼠标包
	// =================================================================
	case *sdriver.ScrollEvent:
		buf.WriteByte(PacketTypeMouse) // Header: 0x01

		// Payload (16 bytes)
		buf.WriteByte(0)                               // [0] Action (滚动视为 Move)
		binary.Write(buf, binary.BigEndian, v.PosX)    // [1-4] X
		binary.Write(buf, binary.BigEndian, v.PosY)    // [5-8] Y
		binary.Write(buf, binary.BigEndian, v.Buttons) // [9-12] Buttons
		// [13-14] Wheel (将 HScroll 转为 int16)
		binary.Write(buf, binary.BigEndian, int16(v.HScroll))
		// [15-16] Wheel (将 VScroll 转为 int16)
		binary.Write(buf, binary.BigEndian, int16(v.VScroll))

	// =================================================================
	// Case 3: 键盘事件 -> 键盘包
	// 接收端 Payload 长度: 5 bytes
	// 结构: [Action 1][KeyCode 4]
	// =================================================================
	case *sdriver.KeyEvent:
		buf.WriteByte(PacketTypeKey) // Header: 0x00

		// Payload (5 bytes)
		buf.WriteByte(v.Action)                                             // [0] Action
		binary.Write(buf, binary.BigEndian, AndroidKeyCodeToX11(v.KeyCode)) // [1-4] KeyCode

	// 其他事件直接忽略
	default:

		return nil
	}

	// 发送数据
	if buf.Len() > 0 {
		_, err := d.conn.Write(buf.Bytes())
		return err
	}
	return nil
}

// AndroidKeyCodeToX11 将 Android 标准 KeyCode 映射为 X11 Keycode
// 转换逻辑: Android KeyCode -> Linux Evdev Code -> X11 KeyCode (Evdev + 8)
func AndroidKeyCodeToX11(androidCode uint32) uint32 {
	switch androidCode {
	// ================= 字母键 (A-Z) =================
	// Android: A=29 ... Z=54
	// Evdev: A=30 ... (不完全顺序对应)
	// X11: Evdev + 8
	case 29:
		return 38 // A
	case 30:
		return 56 // B
	case 31:
		return 54 // C
	case 32:
		return 40 // D
	case 33:
		return 26 // E
	case 34:
		return 41 // F
	case 35:
		return 42 // G
	case 36:
		return 43 // H
	case 37:
		return 31 // I
	case 38:
		return 44 // J
	case 39:
		return 45 // K
	case 40:
		return 46 // L
	case 41:
		return 58 // M
	case 42:
		return 57 // N
	case 43:
		return 32 // O
	case 44:
		return 33 // P
	case 45:
		return 24 // Q
	case 46:
		return 27 // R
	case 47:
		return 39 // S
	case 48:
		return 28 // T
	case 49:
		return 30 // U
	case 50:
		return 55 // V
	case 51:
		return 25 // W
	case 52:
		return 53 // X
	case 53:
		return 29 // Y
	case 54:
		return 52 // Z

	// ================= 数字键 (0-9) =================
	// Android: 0=7, 1=8 ... 9=16
	// X11: 0=19, 1=10 ... 9=18
	case 7:
		return 19 // 0
	case 8:
		return 10 // 1
	case 9:
		return 11 // 2
	case 10:
		return 12 // 3
	case 11:
		return 13 // 4
	case 12:
		return 14 // 5
	case 13:
		return 15 // 6
	case 14:
		return 16 // 7
	case 15:
		return 17 // 8
	case 16:
		return 18 // 9

	// ================= 功能键 & 控制键 =================
	case 66:
		return 36 // Enter (Evdev 28 + 8)
	case 67:
		return 22 // Backspace (Evdev 14 + 8)
	case 112:
		return 119 // Delete (Evdev 111 + 8)
	case 111:
		return 9 // Escape (Evdev 1 + 8)
	case 62:
		return 65 // Space (Evdev 57 + 8)
	case 61:
		return 23 // Tab (Evdev 15 + 8)

	// Home/End/PageUp/PageDown
	// Android Home(3) 通常指手机Home键，但这里如果是键盘Home，通常映射到 X11 Home
	case 3:
		return 110 // Home (Evdev 102 + 8)
	case 123:
		return 115 // End (Evdev 107 + 8)
	case 92:
		return 112 // PageUp (Evdev 104 + 8)
	case 93:
		return 117 // PageDown (Evdev 109 + 8)
	case 124:
		return 118 // Insert (Evdev 110 + 8)

	// 方向键
	case 19:
		return 111 // ArrowUp (Evdev 103 + 8)
	case 20:
		return 116 // ArrowDown (Evdev 108 + 8)
	case 21:
		return 113 // ArrowLeft (Evdev 105 + 8)
	case 22:
		return 114 // ArrowRight (Evdev 106 + 8)

	// 修饰键
	case 59:
		return 50 // ShiftLeft (Evdev 42 + 8)
	case 60:
		return 62 // ShiftRight (Evdev 54 + 8)
	case 113:
		return 37 // ControlLeft (Evdev 29 + 8)
	case 114:
		return 105 // ControlRight (Evdev 97 + 8)
	case 57:
		return 64 // AltLeft (Evdev 56 + 8)
	case 58:
		return 108 // AltRight (Evdev 100 + 8) - 通常是 AltGr
	case 117:
		return 133 // MetaLeft/Win (Evdev 125 + 8)
	case 118:
		return 134 // MetaRight (Evdev 126 + 8)
	case 115:
		return 66 // CapsLock (Evdev 58 + 8)

	// ================= 符号键 (基于美式键盘布局) =================
	// 你的 JS 代码里目前没有映射符号键，如果将来加了，可以参考以下补充：
	case 74:
		return 47 // ; (Semicolon)
	case 70:
		return 21 // = (Equals)
	case 55:
		return 59 // , (Comma)
	case 69:
		return 20 // - (Minus)
	case 56:
		return 60 // . (Period)
	case 76:
		return 61 // / (Slash)
	case 68:
		return 49 // ` (Grave)
	case 71:
		return 34 // [ (Left Bracket)
	case 73:
		return 51 // \ (Backslash)
	case 72:
		return 35 // ] (Right Bracket)
	case 75:
		return 48 // ' (Apostrophe)

	default:
		// log.Printf("Unknown Android KeyCode: %d", androidCode)
		return 0
	}
}
