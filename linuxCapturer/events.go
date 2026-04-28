package linuxcapturer

type EventType byte

const (
	EventTypeKeyboard EventType = 0x00
	EventTypeMouse    EventType = 0x01
	EventTypeTouch    EventType = 0x02
)

type Event interface {
	Type() EventType
}

type MouseEvent struct {
	action         byte   // 0=Down, 1=Up, 2=Move
	x, y           int32  // Pointer Absolute Coordinates (Not Used)
	deltaX, deltaY int32  // Pointer Relative Movement (for Move events)
	buttons        uint32 // Mouse Buttons Mask
	wheelDeltaX    int16  // Scroll Wheel Delta X (for Move events)
	wheelDeltaY    int16  // Scroll Wheel Delta Y (for Move events)
}

func (m *MouseEvent) Type() EventType {
	return EventTypeMouse
}

type TouchEvent struct {
	action   byte   // 0=Down, 1=Up, 2=Move
	ptrID    byte   // Touch pointer ID (0-9)
	x, y     uint16 // Absolute coordinates
	pressure uint16 // Touch pressure
	buttons  byte   // Button mask
}

func (t *TouchEvent) Type() EventType {
	return EventTypeTouch
}

type KeyboardEvent struct {
	action  byte  // 0=Down, 1=Up
	keyCode int32 // X11 KeyCode (mapped from Web KeyCode)
}

func (k *KeyboardEvent) Type() EventType {
	return EventTypeKeyboard
}
