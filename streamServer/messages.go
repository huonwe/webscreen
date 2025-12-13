package streamServer

import (
	"encoding/binary"
	"fmt"
	"webcpy/scrcpy"
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
	WS_TYPE_TOUCH  = 0x01 // touch event
	WS_TYPE_KEY    = 0x02 // key event
	WS_TYPE_SCROLL = 0x03 // scroll event
	WS_TYPE_ROTATE = 0x04 // rotate event
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
	default:
		return 0xFE // unknown type
	}
}

func (sm *StreamManager) createScrcpyTouchEvent(wsMsg []byte) (scrcpy.TouchEvent, error) {
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
		Width:     uint16(sm.DataAdapter.VideoMeta.Width),
		Height:    uint16(sm.DataAdapter.VideoMeta.Height),
	}
	return e, nil
}
