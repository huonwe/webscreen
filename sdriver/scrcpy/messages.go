package scrcpy

// control messages
const TYPE_INJECT_KEYCODE byte = 0          //输入入键盘
const TYPE_INJECT_TEXT byte = 1             //输入文本
const TYPE_INJECT_TOUCH_EVENT byte = 2      //输入触摸事件
const TYPE_INJECT_SCROLL_EVENT byte = 3     //输入滚动事件
const TYPE_BACK_OR_SCREEN_ON = 4            //返回或者屏幕开
const TYPE_EXPAND_NOTIFICATION_PANEL = 5    //展开通知面板
const TYPE_EXPAND_SETTINGS_PANEL = 6        //展开设置面板
const TYPE_COLLAPSE_PANELS = 7              //收起面板
const TYPE_GET_CLIPBOARD = 8                //获取剪贴板
const TYPE_SET_CLIPBOARD = 9                //设置剪贴板
const TYPE_SET_DISPLAY_POWER byte = 10      //关闭屏幕
const TYPE_ROTATE_DEVICE byte = 11          //旋转屏幕
const TYPE_UHID_CREATE = 12                 //创建uhid
const TYPE_UHID_INPUT = 13                  //uhid输入
const TYPE_UHID_DESTROY = 14                //销毁uhid
const TYPE_OPEN_HARD_KEYBOARD_SETTINGS = 15 //打开硬件键盘设置
const TYPE_START_APP = 16                   //启动应用
const TYPE_RESET_VIDEO = 17

const ControlMsgTypeReqIDR = 99

const COPY_KEY_NONE = 0
const COPY_KEY_COPY = 1
const COPY_KEY_CUT = 2

// android keycode ev
const ACTION_DOWN byte = 0
const ACTION_UP byte = 1
const ACTION_MOVE byte = 2

//android mouse event

const BUTTON_PRIMARY uint32 = 1 << 0

/**
 * Button constant: Secondary button (right mouse button).
 *
 * @see #getButtonState
 */
const BUTTON_SECONDARY uint32 = 1 << 1

/**
 * Button constant: Tertiary button (middle mouse button).
 *
 * @see #getButtonState
 */
const BUTTON_TERTIARY uint32 = 1 << 2

// Device -> Client messages
const DEVICE_MSG_TYPE_CLIPBOARD = 0
const DEVICE_MSG_TYPE_ACK_CLIPBOARD = 1
const DEVICE_MSG_TYPE_UHID_OUTPUT = 2

// device messages
const TYPE_CLIPBOARD = 0
const TYPE_ACK_CLIPBOARD = 1
const TYPE_UHID_OUTPUT = 2

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

type ScrollEvent struct {
	Type    byte
	PosX    uint32
	PosY    uint32
	Width   uint16
	Height  uint16
	HScroll uint16
	VScroll uint16
	Buttons uint32
}

type UHIDCreateEvent struct {
	Type           byte
	ID             uint16 // 设备 ID (对应官方的 id 字段)
	VendorID       uint16
	ProductID      uint16
	NameSize       uint8
	Name           []byte
	ReportDescSize uint16
	ReportDesc     []byte
}

type UHIDInputEvent struct {
	Type byte
	ID   uint16 // 设备 ID (对应官方的 id 字段)
	Size uint16
	Data []byte
}

type UHIDDestroyEvent struct {
	Type byte
	ID   uint16 // 设备 ID (对应官方的 id 字段)
}
