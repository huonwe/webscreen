package scrcpy

import "encoding/binary"

// control messages
var TYPE_INJECT_KEYCODE byte = 0          //输入入键盘
var TYPE_INJECT_TEXT byte = 1             //输入文本
var TYPE_INJECT_TOUCH_EVENT byte = 2      //输入触摸事件
var TYPE_INJECT_SCROLL_EVENT byte = 3     //输入滚动事件
var TYPE_BACK_OR_SCREEN_ON = 4            //返回或者屏幕开
var TYPE_EXPAND_NOTIFICATION_PANEL = 5    //展开通知面板
var TYPE_EXPAND_SETTINGS_PANEL = 6        //展开设置面板
var TYPE_COLLAPSE_PANELS = 7              //收起面板
var TYPE_GET_CLIPBOARD = 8                //获取剪贴板
var TYPE_SET_CLIPBOARD = 9                //设置剪贴板
var TYPE_SET_DISPLAY_POWER byte = 10      //关闭屏幕
var TYPE_ROTATE_DEVICE byte = 11          //旋转屏幕
var TYPE_UHID_CREATE = 12                 //创建uhid
var TYPE_UHID_INPUT = 13                  //uhid输入
var TYPE_UHID_DESTROY = 14                //销毁uhid
var TYPE_OPEN_HARD_KEYBOARD_SETTINGS = 15 //打开硬件键盘设置
var TYPE_START_APP = 16                   //启动应用
var TYPE_RESET_VIDEO = 17

const ControlMsgTypeReqIDR = 99

// android keycode ev
var ACTION_DOWN byte = 0
var ACTION_UP byte = 1
var ACTION_MOVE byte = 2

//android mouse event

var BUTTON_PRIMARY uint32 = 1 << 0

/**
 * Button constant: Secondary button (right mouse button).
 *
 * @see #getButtonState
 */
var BUTTON_SECONDARY uint32 = 1 << 1

/**
 * Button constant: Tertiary button (middle mouse button).
 *
 * @see #getButtonState
 */
var BUTTON_TERTIARY uint32 = 1 << 2

// device messages
var TYPE_CLIPBOARD = 0
var TYPE_ACK_CLIPBOARD = 1
var TYPE_UHID_OUTPUT = 2

// | **字段**         | **长度 (Byte)** | **说明**                                                                             |
// | -------------- | ------------- | ---------------------------------------------------------------------------------- |
// | **Type**       | 1             | 固定为 **2** (`INJECT_TOUCH_EVENT`)                                                   |
// | **Action**     | 1             | **0**: Down (按下)<br><br>  <br><br>**1**: Up (抬起)<br><br>  <br><br>**2**: Move (移动) |
// | **PointerID**  | 8 (uint64)    | 手指 ID。第一根手指通常是 0。多点触控时用不同 ID。                                                      |
// | **Position X** | 4 (uint32)     | 屏幕 X 坐标 (像素)                                                                       |
// | **Position Y** | 4 (uint32)     | 屏幕 Y 坐标 (像素)                                                                       |
// | **Width**      | 2 (uint16)    | 屏幕宽 (用于归一化计算，通常填当前分辨率宽)                                                            |
// | **Height**     | 2 (uint16)    | 屏幕高 (用于归一化计算，通常填当前分辨率高)                                                            |
// | **Pressure**   | 2 (uint16)    | 压力值 (0~65535)，通常填 65535 (最大) 或 0 (抬起时)                                             |
// | **Buttons**    | 4 (int32)     | 按键状态 (通常 0，鼠标点击时才用到)                                                               |
type TouchEvent struct {
	Type      byte
	Action    byte
	PointerID uint64
	PosX      uint32
	PosY      uint32
	Width     uint16
	Height    uint16
	Pressure  uint16
	Buttons   uint32
}

type KeyEvent struct {
	Type    byte
	Action  byte
	KeyCode uint32
}

func (e *TouchEvent) UnmarshalBinary(data []byte) error {
	e.Type = data[0]
	e.Action = data[1]
	e.PointerID = binary.BigEndian.Uint64(data[2:10])
	e.PosX = binary.BigEndian.Uint32(data[10:14])
	e.PosY = binary.BigEndian.Uint32(data[14:18])
	e.Width = binary.BigEndian.Uint16(data[18:20])
	e.Height = binary.BigEndian.Uint16(data[20:22])
	e.Pressure = binary.BigEndian.Uint16(data[22:24])
	e.Buttons = binary.BigEndian.Uint32(data[24:28])
	return nil
}
func (e *KeyEvent) UnmarshalBinary(data []byte) error {
	e.Type = data[0]
	e.Action = data[1]
	e.KeyCode = binary.BigEndian.Uint32(data[2:6])
	return nil
}
