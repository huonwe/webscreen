package ScrcpyAgent

import (
	"encoding/binary"
	"fmt"
	"webcpy/sdriver/scrcpy"
)

// WS Control Message Types
// All Big Endian
// Touch Packet Structure:
// 偏移,    长度,         类型,       字段名,      说明
// 0,       1,          uint8,      Type,       固定 0x01 (Touch)
// 1,       1,          uint8,      Action,     "0: Down, 1: Up, 2: Move"
// 2,       1,          uint8,      PtrId,      手指 ID (0~9)，用于多点触控
// 3,       2,          uint16,     X,          "归一化 X (0 = 最左, 65535 = 最右)"
// 5,       2,          uint16,     Y,          "归一化 Y (0 = 最上, 65535 = 最下)"
// 7,       2,          uint16,     Pressure,   压力值 (通常 0 或 65535)
// 9,       1,          uint8,      Buttons,    "鼠标按键 (1:主键, 2:右键)"

const (
	WS_TYPE_TOUCH          = 0x01 // touch event
	WS_TYPE_KEY            = 0x02 // key event
	WS_TYPE_SCROLL         = 0x03 // scroll event
	WS_TYPE_ROTATE         = 0x04 // rotate event
	WS_TYPE_UHID_CREATE    = 12   // create uhid
	WS_TYPE_UHID_INPUT     = 13   // uhid input
	WS_TYPE_UHID_DESTROY   = 14   // destroy uhid
	WS_TYPE_SET_CLIPBOARD  = 15   // set clipboard
	WS_TYPE_GET_CLIPBOARD  = 16   // get clipboard
	WS_TYPE_CLIPBOARD_DATA = 17   // clipboard data (server -> client)
)

// ScrcpyTypeFromWSMsgType converts websocket message type to scrcpy control message type
// Returns the scrcpy type and a boolean indicating if the conversion was successful
func ScrcpyTypeFromWSMsgType(t byte) byte {
	switch t {
	case WS_TYPE_TOUCH:
		return scrcpy.TYPE_INJECT_TOUCH_EVENT
	case WS_TYPE_KEY:
		return scrcpy.TYPE_INJECT_KEYCODE
	case WS_TYPE_SCROLL:
		return scrcpy.TYPE_INJECT_SCROLL_EVENT
	case WS_TYPE_ROTATE:
		return scrcpy.TYPE_ROTATE_DEVICE
	case WS_TYPE_UHID_CREATE:
		return scrcpy.TYPE_UHID_CREATE
	case WS_TYPE_UHID_INPUT:
		return scrcpy.TYPE_UHID_INPUT
	case WS_TYPE_UHID_DESTROY:
		return scrcpy.TYPE_UHID_DESTROY
	default:
		return t
	}
}

func (sm *ScrcpyAgent) createScrcpyKeyEvent(wsMsg []byte) (scrcpy.KeyEvent, error) {
	if len(wsMsg) != 4 {
		return scrcpy.KeyEvent{}, fmt.Errorf("invalid key event message length: %d", len(wsMsg))
	}
	e := scrcpy.KeyEvent{
		Type:    scrcpy.TYPE_INJECT_KEYCODE,
		Action:  wsMsg[1],
		KeyCode: uint32(binary.BigEndian.Uint16(wsMsg[2:4])),
	}
	return e, nil
}

func (a *ScrcpyAgent) createScrcpyTouchEvent(wsMsg []byte) (scrcpy.TouchEvent, error) {
	if len(wsMsg) != 10 {
		return scrcpy.TouchEvent{}, fmt.Errorf("invalid touch event message length: %d", len(wsMsg))
	}
	e := scrcpy.TouchEvent{
		Type:      ScrcpyTypeFromWSMsgType(wsMsg[0]),
		Action:    wsMsg[1],
		PointerID: uint64(wsMsg[2]),
		PosX:      uint32(binary.BigEndian.Uint16(wsMsg[3:5])),
		PosY:      uint32(binary.BigEndian.Uint16(wsMsg[5:7])),
		Pressure:  binary.BigEndian.Uint16(wsMsg[7:9]),
		Buttons:   uint32(wsMsg[9]),
		Width:     uint16(a.MediaMeta.Width),
		Height:    uint16(a.MediaMeta.Height),
	}
	return e, nil
}

func (sm *ScrcpyAgent) createScrcpyScrollEvent(wsMsg []byte) (scrcpy.ScrollEvent, error) {
	// WS Scroll Packet:
	// 0: Type (0x03)
	// 1-2: X (uint16)
	// 3-4: Y (uint16)
	// 5-6: hScroll (int16)
	// 7-8: vScroll (int16)
	if len(wsMsg) != 10 {
		return scrcpy.ScrollEvent{}, fmt.Errorf("invalid scroll event message length: %d", len(wsMsg))
	}
	e := scrcpy.ScrollEvent{
		Type:    scrcpy.TYPE_INJECT_SCROLL_EVENT,
		PosX:    uint32(binary.BigEndian.Uint16(wsMsg[1:3])),
		PosY:    uint32(binary.BigEndian.Uint16(wsMsg[3:5])),
		Width:   uint16(sm.MediaMeta.Width),
		Height:  uint16(sm.MediaMeta.Height),
		HScroll: binary.BigEndian.Uint16(wsMsg[5:7]),
		VScroll: binary.BigEndian.Uint16(wsMsg[7:9]),
		Buttons: uint32(wsMsg[9]),
	}
	return e, nil
}

func (sm *ScrcpyAgent) createScrcpyUHIDCreateEvent(wsMsg []byte) (scrcpy.UHIDCreateEvent, error) {
	// 协议: [Type 1][ID 2][Vendor 2][Prod 2][NameLen 1][Name N][DescLen 2][Desc N]

	// 最小头部: 1+2+2+2+1 = 8 字节
	const minHeaderSize = 8

	if len(wsMsg) < minHeaderSize {
		return scrcpy.UHIDCreateEvent{}, fmt.Errorf("header too short: %d", len(wsMsg))
	}

	// 1. 读取名字长度 (单字节)
	nameSize := int(wsMsg[7]) // 第 8 个字节

	// 2. 计算描述符长度的位置
	// 8 (header) + nameSize
	descSizeOffset := 8 + nameSize

	if len(wsMsg) < descSizeOffset+2 {
		return scrcpy.UHIDCreateEvent{}, fmt.Errorf("msg too short for desc size")
	}

	// 3. 读取描述符长度 (2字节)
	reportDescSize := binary.BigEndian.Uint16(wsMsg[descSizeOffset : descSizeOffset+2])

	// 4. 校验总长
	totalExpected := descSizeOffset + 2 + int(reportDescSize)
	if len(wsMsg) < totalExpected {
		return scrcpy.UHIDCreateEvent{}, fmt.Errorf("body missing: expected %d, got %d", totalExpected, len(wsMsg))
	}

	e := scrcpy.UHIDCreateEvent{
		Type:      scrcpy.TYPE_UHID_CREATE,
		ID:        binary.BigEndian.Uint16(wsMsg[1:3]),
		VendorID:  binary.BigEndian.Uint16(wsMsg[3:5]),
		ProductID: binary.BigEndian.Uint16(wsMsg[5:7]),
		// NameSize:    (不需要存，动态算)
		Name:           wsMsg[8 : 8+nameSize],
		ReportDescSize: reportDescSize,
		ReportDesc:     wsMsg[descSizeOffset+2 : descSizeOffset+2+int(reportDescSize)],
	}
	return e, nil
}

func (sm *ScrcpyAgent) createScrcpyUHIDInputEvent(wsMsg []byte) (scrcpy.UHIDInputEvent, error) {
	// WS Packet:
	// 0: Type (13)
	// 1-2: Device ID (uint16)
	// 3-4: Report Size (uint16)
	// 5+: Report Data

	if len(wsMsg) < 5 {
		return scrcpy.UHIDInputEvent{}, fmt.Errorf("invalid uhid input message length: %d", len(wsMsg))
	}

	reportSize := binary.BigEndian.Uint16(wsMsg[3:5])
	if len(wsMsg) < 5+int(reportSize) {
		return scrcpy.UHIDInputEvent{}, fmt.Errorf("invalid uhid input message length (data): expected %d, got %d", 5+reportSize, len(wsMsg))
	}

	e := scrcpy.UHIDInputEvent{
		Type: scrcpy.TYPE_UHID_INPUT,
		ID:   binary.BigEndian.Uint16(wsMsg[1:3]),
		Size: reportSize,
		Data: wsMsg[5 : 5+int(reportSize)],
	}
	return e, nil
}

func (sm *ScrcpyAgent) createScrcpyUHIDDestroyEvent(wsMsg []byte) (scrcpy.UHIDDestroyEvent, error) {
	// WS Packet:
	// 0: Type (14)
	// 1-2: Device ID (uint16)

	if len(wsMsg) != 3 {
		return scrcpy.UHIDDestroyEvent{}, fmt.Errorf("invalid uhid destroy message length: %d", len(wsMsg))
	}

	e := scrcpy.UHIDDestroyEvent{
		Type: scrcpy.TYPE_UHID_DESTROY,
		ID:   binary.BigEndian.Uint16(wsMsg[1:3]),
	}
	return e, nil
}
