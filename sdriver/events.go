package sdriver

type EventType uint8

type Event interface {
	Type() EventType
}

type SampleEvent interface {
	Event
	GetContent() []byte
}

const (
	// Basic Events
	EVENT_TYPE_KEY    EventType = 0x00
	EVENT_TYPE_MOUSE  EventType = 0x01
	EVENT_TYPE_TOUCH  EventType = 0x02
	EVENT_TYPE_SCROLL EventType = 0x03

	// Clipboard Events Agent -> Driver
	EVENT_TYPE_GET_CLIPBOARD EventType = 0x08
	EVENT_TYPE_SET_CLIPBOARD EventType = 0x09
	// Clipboard Events Driver -> Agent -> Web
	EVENT_TYPE_RECEIVE_CLIPBOARD EventType = 0x17

	// Command
	EVENT_TYPE_DISPLAY_OFF EventType = 0x0A
	EVENT_TYPE_ROTATE      EventType = 0x0B

	// UHID Events
	EVENT_TYPE_UHID_CREATE  EventType = 0x0C
	EVENT_TYPE_UHID_INPUT   EventType = 0x0D
	EVENT_TYPE_UHID_DESTROY EventType = 0x0E

	EVENT_TYPE_REQ_IDR EventType = 0x63
)

// 鼠标动作枚举
const (
	TOUCH_ACTION_MOVE = 0
	TOUCH_ACTION_DOWN = 1
	TOUCH_ACTION_UP   = 2
)

const (
	BUTTON_PRIMARY   uint32 = 1 << 0
	BUTTON_SECONDARY uint32 = 1 << 1
	BUTTON_TERTIARY  uint32 = 1 << 2
)

const (
	COPY_KEY_NONE uint8 = 0
	COPY_KEY_COPY uint8 = 1
	COPY_KEY_CUT  uint8 = 2
)

type TouchEvent struct {
	Action    byte
	PointerID uint64
	PosX      uint32
	PosY      uint32
	Width     uint16
	Height    uint16
	Pressure  uint16
	Buttons   uint32
}

func (e TouchEvent) Type() EventType {
	return EVENT_TYPE_TOUCH
}

type KeyEvent struct {
	Action  byte
	KeyCode uint32
}

func (e KeyEvent) Type() EventType {
	return EVENT_TYPE_KEY
}

type ScrollEvent struct {
	PosX    uint32
	PosY    uint32
	Width   uint16
	Height  uint16
	HScroll uint16
	VScroll uint16
	Buttons uint32
}

func (e ScrollEvent) Type() EventType {
	return EVENT_TYPE_SCROLL
}

type RotateEvent struct {
}

func (e RotateEvent) Type() EventType {
	return EVENT_TYPE_ROTATE
}

type UHIDCreateEvent struct {
	ID             uint16 // 设备 ID (对应官方的 id 字段)
	VendorID       uint16
	ProductID      uint16
	NameSize       uint8
	Name           []byte
	ReportDescSize uint16
	ReportDesc     []byte
}

func (e UHIDCreateEvent) Type() EventType {
	return EVENT_TYPE_UHID_CREATE
}

type UHIDInputEvent struct {
	ID   uint16 // 设备 ID (对应官方的 id 字段)
	Size uint16
	Data []byte
}

func (e UHIDInputEvent) Type() EventType {
	return EVENT_TYPE_UHID_INPUT
}

type UHIDDestroyEvent struct {
	ID uint16 // 设备 ID (对应官方的 id 字段)
}

func (e UHIDDestroyEvent) Type() EventType {
	return EVENT_TYPE_UHID_DESTROY
}

type ReqIDREvent struct{}

func (e ReqIDREvent) Type() EventType {
	return EVENT_TYPE_REQ_IDR
}

type GetClipboardEvent struct {
	CopyKey uint8 // 是否模拟复制按键
}

func (e GetClipboardEvent) Type() EventType {
	return EVENT_TYPE_GET_CLIPBOARD
}

type SetClipboardEvent struct {
	Sequence uint64 // 序列号
	Paste    bool   // 是否模拟粘贴
	Content  []byte // 剪贴板文本内容
}

func (e SetClipboardEvent) Type() EventType {
	return EVENT_TYPE_SET_CLIPBOARD
}

type ReceiveClipboardEvent struct {
	Content []byte // 剪贴板文本内容
}

func (e ReceiveClipboardEvent) Type() EventType {
	return EVENT_TYPE_RECEIVE_CLIPBOARD
}

func (e ReceiveClipboardEvent) GetContent() []byte {
	return e.Content
}
