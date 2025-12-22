package sagent

import (
	"encoding/binary"
	"fmt"
	"webscreen/sdriver"
)

func (a *Agent) parseEvent(raw []byte) (sdriver.Event, error) {
	if len(raw) < 1 {
		return nil, fmt.Errorf("empty event data")
	}
	eventType := sdriver.EventType(raw[0])
	// log.Printf("Parsing control event of type: %d", eventType)

	switch eventType {
	// case sdriver.EVENT_TYPE_MOUSE:
	// 	return a.parseMouseEvent(raw)
	case sdriver.EVENT_TYPE_TOUCH:
		return a.parseTouchEvent(raw)
	case sdriver.EVENT_TYPE_KEY:
		return a.parseKeyEvent(raw)
	case sdriver.EVENT_TYPE_SCROLL:
		return a.parseScrollEvent(raw)
	case sdriver.EVENT_TYPE_ROTATE:
		return a.parseRotateEvent(raw)
	case sdriver.EVENT_TYPE_UHID_CREATE:
		return a.parseUHIDCreateEvent(raw)
	case sdriver.EVENT_TYPE_UHID_INPUT:
		return a.parseUHIDInputEvent(raw)
	case sdriver.EVENT_TYPE_UHID_DESTROY:
		return a.parseUHIDDestroyEvent(raw)
	case sdriver.EVENT_TYPE_GET_CLIPBOARD:
		return a.parseGetClipboardEvent(raw)
	case sdriver.EVENT_TYPE_SET_CLIPBOARD:
		return a.parseSetClipboardEvent(raw)
	case sdriver.EVENT_TYPE_REQ_IDR:
		return a.parseIDRReqEvent(raw)
	default:
		return nil, fmt.Errorf("unknown event type: %d", eventType)
	}
}

func (a *Agent) parseTouchEvent(raw []byte) (*sdriver.TouchEvent, error) {
	if len(raw) != 10 {
		return nil, fmt.Errorf("invalid touch event message length: %d", len(raw))
	}
	e := &sdriver.TouchEvent{
		Action:    raw[1],
		PointerID: uint64(raw[2]),
		PosX:      uint32(binary.BigEndian.Uint16(raw[3:5])),
		PosY:      uint32(binary.BigEndian.Uint16(raw[5:7])),
		Pressure:  binary.BigEndian.Uint16(raw[7:9]),
		Buttons:   uint32(raw[9]),
		Width:     uint16(a.driver.MediaMeta().Width),
		Height:    uint16(a.driver.MediaMeta().Height),
	}
	return e, nil
}

func (a *Agent) parseKeyEvent(raw []byte) (*sdriver.KeyEvent, error) {
	if len(raw) != 4 {
		return nil, fmt.Errorf("invalid key event message length: %d", len(raw))
	}
	e := &sdriver.KeyEvent{
		Action:  raw[1],
		KeyCode: uint32(binary.BigEndian.Uint16(raw[2:4])),
	}
	return e, nil
}

func (a *Agent) parseScrollEvent(raw []byte) (*sdriver.ScrollEvent, error) {
	if len(raw) != 10 {
		return nil, fmt.Errorf("invalid scroll event message length: %d", len(raw))
	}
	e := &sdriver.ScrollEvent{
		PosX:    uint32(binary.BigEndian.Uint16(raw[1:3])),
		PosY:    uint32(binary.BigEndian.Uint16(raw[3:5])),
		Width:   uint16(a.driver.MediaMeta().Width),
		Height:  uint16(a.driver.MediaMeta().Height),
		HScroll: binary.BigEndian.Uint16(raw[5:7]),
		VScroll: binary.BigEndian.Uint16(raw[7:9]),
		Buttons: uint32(raw[9]),
	}
	return e, nil
}

func (a *Agent) parseRotateEvent(raw []byte) (*sdriver.RotateEvent, error) {
	return &sdriver.RotateEvent{}, nil
}

func (a *Agent) parseIDRReqEvent(raw []byte) (*sdriver.IDRReqEvent, error) {
	return &sdriver.IDRReqEvent{}, nil
}

func (a *Agent) parseUHIDCreateEvent(raw []byte) (*sdriver.UHIDCreateEvent, error) {
	// 协议: [Type 1][ID 2][Vendor 2][Prod 2][NameLen 1][Name N][DescLen 2][Desc N]
	const minHeaderSize = 8
	if len(raw) < minHeaderSize {
		return nil, fmt.Errorf("header too short: %d", len(raw))
	}

	nameSize := int(raw[7])
	descSizeOffset := 8 + nameSize

	if len(raw) < descSizeOffset+2 {
		return nil, fmt.Errorf("msg too short for desc size")
	}

	reportDescSize := binary.BigEndian.Uint16(raw[descSizeOffset : descSizeOffset+2])
	totalExpected := descSizeOffset + 2 + int(reportDescSize)
	if len(raw) < totalExpected {
		return nil, fmt.Errorf("body missing: expected %d, got %d", totalExpected, len(raw))
	}

	e := &sdriver.UHIDCreateEvent{
		ID:             binary.BigEndian.Uint16(raw[1:3]),
		VendorID:       binary.BigEndian.Uint16(raw[3:5]),
		ProductID:      binary.BigEndian.Uint16(raw[5:7]),
		NameSize:       uint8(nameSize),
		Name:           raw[8 : 8+nameSize],
		ReportDescSize: reportDescSize,
		ReportDesc:     raw[descSizeOffset+2 : descSizeOffset+2+int(reportDescSize)],
	}
	return e, nil
}

func (a *Agent) parseUHIDInputEvent(raw []byte) (*sdriver.UHIDInputEvent, error) {
	// WS Packet: [Type 1][ID 2][Size 2][Data N]
	if len(raw) < 5 {
		return nil, fmt.Errorf("invalid uhid input message length: %d", len(raw))
	}

	reportSize := binary.BigEndian.Uint16(raw[3:5])
	if len(raw) < 5+int(reportSize) {
		return nil, fmt.Errorf("invalid uhid input message length (data): expected %d, got %d", 5+reportSize, len(raw))
	}

	e := &sdriver.UHIDInputEvent{
		ID:   binary.BigEndian.Uint16(raw[1:3]),
		Size: reportSize,
		Data: raw[5 : 5+int(reportSize)],
	}
	return e, nil
}

func (a *Agent) parseUHIDDestroyEvent(raw []byte) (*sdriver.UHIDDestroyEvent, error) {
	// WS Packet: [Type 1][ID 2]
	if len(raw) != 3 {
		return nil, fmt.Errorf("invalid uhid destroy message length: %d", len(raw))
	}

	e := &sdriver.UHIDDestroyEvent{
		ID: binary.BigEndian.Uint16(raw[1:3]),
	}
	return e, nil
}

func (a *Agent) parseGetClipboardEvent(raw []byte) (*sdriver.GetClipboardEvent, error) {
	// WS Packet: [Type 1]
	// log.Printf("Parsing GetClipboardEvent, raw length: %d", len(raw))
	CopyKey := raw[1]
	e := &sdriver.GetClipboardEvent{
		CopyKey: CopyKey, // 默认不模拟复制按键
	}
	return e, nil
}

func (a *Agent) parseSetClipboardEvent(raw []byte) (*sdriver.SetClipboardEvent, error) {
	// WS Packet: [Type 1][Sequence 8][Paste 1][TextLen 4][Text N]
	const minHeaderSize = 14 // 1 + 8 + 1 + 4
	if len(raw) < minHeaderSize {
		return nil, fmt.Errorf("invalid set clipboard message length: %d, expected at least %d", len(raw), minHeaderSize)
	}

	sequence := binary.BigEndian.Uint64(raw[1:9])
	paste := raw[9] != 0
	textLen := binary.BigEndian.Uint32(raw[10:14])

	if len(raw) < minHeaderSize+int(textLen) {
		return nil, fmt.Errorf("invalid set clipboard message length (text): expected %d, got %d", minHeaderSize+textLen, len(raw))
	}

	e := &sdriver.SetClipboardEvent{
		Sequence: sequence,
		Paste:    paste,
		Content:  raw[14 : 14+textLen],
	}
	return e, nil
}
