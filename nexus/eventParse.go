package nexus

import (
	"encoding/binary"
	"fmt"
	"log"
	"scrmux/sdriver"
)

func (b *remoteBinding) HandleControlEvent(raw []byte) error {
	if !b.sdriver.Capabilities().CanControl {
		return fmt.Errorf("driver does not support control events")
	}
	event, err := b.parseEvent(raw)
	if err != nil {
		log.Printf("[agent] Failed to parse control event: %v", err)
		return err
	}
	// log.Println("[agent] Parsed control event:", event)
	// log.Printf("Parsed control event: %+v", event)
	return b.sdriver.SendEvent(event)
}

// func (r *Remote) HandleControlEventStream(raw []byte) {
// 	event, err := b.parseEvent(raw)
// 	if err != nil {
// 		log.Printf("[agent] Failed to parse control event: %v", err)
// 		return
// 	}
// 	_, _, _, controlCh := r.sdriver.GetReceivers()
// 	select {
// 	case controlCh <- event:
// 	default:
// 		log.Printf("[agent] Failed to send control event: channel full")
// 	}
// 	// log.Printf("Parsed control event: %+v", event)
// }

func (b *remoteBinding) parseEvent(raw []byte) (sdriver.Event, error) {
	if len(raw) < 1 {
		return nil, fmt.Errorf("empty event data")
	}
	eventType := sdriver.EventType(raw[0])
	// log.Printf("Parsing control event of type: %d", eventType)

	switch eventType {
	// case sdriver.EVENT_TYPE_MOUSE:
	// 	return b.parseMouseEvent(raw)
	case sdriver.EVENT_TYPE_TOUCH:
		return b.parseTouchEvent(raw)
	case sdriver.EVENT_TYPE_MOUSE:
		return b.parseMouseEvent(raw)
	case sdriver.EVENT_TYPE_KEY:
		return b.parseKeyEvent(raw)
	case sdriver.EVENT_TYPE_SCROLL:
		return b.parseScrollEvent(raw)
	case sdriver.EVENT_TYPE_ROTATE:
		return b.parseRotateEvent()
	case sdriver.EVENT_TYPE_UHID_CREATE:
		return b.parseUHIDCreateEvent(raw)
	case sdriver.EVENT_TYPE_UHID_INPUT:
		return b.parseUHIDInputEvent(raw)
	case sdriver.EVENT_TYPE_UHID_DESTROY:
		return b.parseUHIDDestroyEvent(raw)
	case sdriver.EVENT_TYPE_GET_CLIPBOARD:
		return b.parseGetClipboardEvent(raw)
	case sdriver.EVENT_TYPE_SET_CLIPBOARD:
		return b.parseSetClipboardEvent(raw)
	case sdriver.EVENT_TYPE_REQ_IDR:
		return b.parseIDRReqEvent()
	case sdriver.EVENT_TYPE_GAMEPAD:
		return b.parseGamepadEvent(raw)
	default:
		return nil, fmt.Errorf("unknown event type: %d", eventType)
	}
}

func (b *remoteBinding) parseTouchEvent(raw []byte) (*sdriver.TouchEvent, error) {
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
		Width:     uint16(b.sdriver.MediaMeta().Width),
		Height:    uint16(b.sdriver.MediaMeta().Height),
	}
	return e, nil
}

func (b *remoteBinding) parseMouseEvent(raw []byte) (*sdriver.MouseEvent, error) {
	// 1(Type) + 1(Action) + 4(X) + 4(Y) + 4(Buttons) + 2(WheelX) + 2(WheelY) = 18
	if len(raw) != 18 {
		return nil, fmt.Errorf("invalid mouse event message length: %d", len(raw))
	}

	e := &sdriver.MouseEvent{
		// [0] 是 Type，调用者可能已经判断过了，这里跳过
		Action:      raw[1],                                     // Offset 1
		PosX:        binary.BigEndian.Uint32(raw[2:6]),          // Offset 2-5 (4 bytes)
		PosY:        binary.BigEndian.Uint32(raw[6:10]),         // Offset 6-9 (4 bytes)
		Buttons:     binary.BigEndian.Uint32(raw[10:14]),        // Offset 10-13 (4 bytes)
		WheelDeltaX: int16(binary.BigEndian.Uint16(raw[14:16])), // Offset 14-15 (2 bytes)
		WheelDeltaY: int16(binary.BigEndian.Uint16(raw[16:18])), // Offset 16-17 (2 bytes)
	}

	return e, nil
}

func (b *remoteBinding) parseKeyEvent(raw []byte) (*sdriver.KeyEvent, error) {
	if len(raw) != 4 {
		return nil, fmt.Errorf("invalid key event message length: %d", len(raw))
	}
	e := &sdriver.KeyEvent{
		Action:  raw[1],
		KeyCode: uint32(binary.BigEndian.Uint16(raw[2:4])),
	}
	return e, nil
}

func (b *remoteBinding) parseScrollEvent(raw []byte) (*sdriver.ScrollEvent, error) {
	if len(raw) != 10 {
		return nil, fmt.Errorf("invalid scroll event message length: %d", len(raw))
	}
	e := &sdriver.ScrollEvent{
		PosX:    uint32(binary.BigEndian.Uint16(raw[1:3])),
		PosY:    uint32(binary.BigEndian.Uint16(raw[3:5])),
		Width:   uint16(b.sdriver.MediaMeta().Width),
		Height:  uint16(b.sdriver.MediaMeta().Height),
		HScroll: binary.BigEndian.Uint16(raw[5:7]),
		VScroll: binary.BigEndian.Uint16(raw[7:9]),
		Buttons: uint32(raw[9]),
	}
	return e, nil
}

func (b *remoteBinding) parseRotateEvent() (*sdriver.RotateEvent, error) {
	return &sdriver.RotateEvent{}, nil
}

func (b *remoteBinding) parseIDRReqEvent() (*sdriver.IDRReqEvent, error) {
	return &sdriver.IDRReqEvent{}, nil
}

func (b *remoteBinding) parseGamepadEvent(raw []byte) (*sdriver.GamepadEvent, error) {
	if len(raw) != 18 {
		return nil, fmt.Errorf("invalid gamepad event message length: %d", len(raw))
	}
	return &sdriver.GamepadEvent{
		ControllerNumber:  raw[1],
		ActiveGamepadMask: binary.BigEndian.Uint16(raw[2:4]),
		Buttons:           binary.BigEndian.Uint32(raw[4:8]),
		LeftTrigger:       raw[8],
		RightTrigger:      raw[9],
		LeftStickX:        int16(binary.BigEndian.Uint16(raw[10:12])),
		LeftStickY:        int16(binary.BigEndian.Uint16(raw[12:14])),
		RightStickX:       int16(binary.BigEndian.Uint16(raw[14:16])),
		RightStickY:       int16(binary.BigEndian.Uint16(raw[16:18])),
	}, nil
}

func (b *remoteBinding) parseUHIDCreateEvent(raw []byte) (*sdriver.UHIDCreateEvent, error) {
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

	name := append([]byte{0x77, 0x65, 0x62, 0x73, 0x63, 0x72, 0x65, 0x65, 0x6e, 0x20}, raw[8:8+nameSize]...)
	e := &sdriver.UHIDCreateEvent{
		ID:             binary.BigEndian.Uint16(raw[1:3]),
		VendorID:       binary.BigEndian.Uint16(raw[3:5]),
		ProductID:      binary.BigEndian.Uint16(raw[5:7]),
		NameSize:       uint8(len(name)),
		Name:           name,
		ReportDescSize: reportDescSize,
		ReportDesc:     raw[descSizeOffset+2 : descSizeOffset+2+int(reportDescSize)],
	}
	return e, nil
}

func (b *remoteBinding) parseUHIDInputEvent(raw []byte) (*sdriver.UHIDInputEvent, error) {
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

func (b *remoteBinding) parseUHIDDestroyEvent(raw []byte) (*sdriver.UHIDDestroyEvent, error) {
	// WS Packet: [Type 1][ID 2]
	if len(raw) != 3 {
		return nil, fmt.Errorf("invalid uhid destroy message length: %d", len(raw))
	}

	e := &sdriver.UHIDDestroyEvent{
		ID: binary.BigEndian.Uint16(raw[1:3]),
	}
	return e, nil
}

func (b *remoteBinding) parseGetClipboardEvent(raw []byte) (*sdriver.GetClipboardEvent, error) {
	// WS Packet: [Type 1]
	// log.Printf("Parsing GetClipboardEvent, raw length: %d", len(raw))
	CopyKey := raw[1]
	e := &sdriver.GetClipboardEvent{
		CopyKey: CopyKey,
	}
	return e, nil
}

func (b *remoteBinding) parseSetClipboardEvent(raw []byte) (*sdriver.SetClipboardEvent, error) {
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
