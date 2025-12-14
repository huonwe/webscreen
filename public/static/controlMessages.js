const TYPE_TOUCH = 0x01; // touch event
const TYPE_KEY   = 0x02; // key event
const TYPE_SCROLL = 0x03; // scroll event
const TYPE_ROTATE    = 0x04; // rotate event

// 这里WebRTC会自动通过RTCP请求关键帧，但我们也可以手动请求
const TYPE_RKF   = 0xFF; // request key frame

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
const TOUCH_ACTION_DOWN = 0;
const TOUCH_ACTION_UP = 1;
const TOUCH_ACTION_MOVE = 2;

// Key Packet Structure:
// 偏移,长度,类型,字段名,说明
// 0,1,uint8,Type,固定 0x02 (KeyEvent)
// 1,1,uint8,Action,"0: Down, 1: Up"
// 2,2,uint16,KeyCode,Android KeyCode (如 Power=26)
const TYPE_KEY_ACTION_DOWN = 0;
const TYPE_KEY_ACTION_UP = 1;


function createTouchPacket(action, ptrId, x, y, pressure=65535, buttons=1) {
    const buffer = new ArrayBuffer(10);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_TOUCH);
    view.setUint8(1, action);
    view.setUint8(2, ptrId);
    view.setUint16(3, x);
    view.setUint16(5, y);
    view.setUint16(7, pressure);
    view.setUint8(9, buttons);
    return buffer;
}

function praseTouchEvent(packet) {
    const view = new DataView(packet);
    const type = view.getUint8(0);
    if (type !== TYPE_TOUCH) {
        throw new Error("Not a touch event packet");
    }
    const action = view.getUint8(1);
    const ptrId = view.getUint8(2);
    const x = view.getUint16(3);
    const y = view.getUint16(5);
    const pressure = view.getUint16(7);
    const buttons = view.getUint8(9);
    console.log("Parsed Touch Event:", {action, ptrId, x, y, pressure, buttons});
    return {
        action,
        ptrId,
        x,
        y,
        pressure,
        buttons
    };
}

function createKeyPacket(action, keyCode) {
    const buffer = new ArrayBuffer(4);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_KEY);
    view.setUint8(1, action);
    view.setUint16(2, keyCode);
    return buffer;
}

function createRotatePacket() {
    const buffer = new ArrayBuffer(1);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_ROTATE);
    return buffer;
}

function createScrollPacket(x, y, hScroll, vScroll) {
    // Scroll Packet Structure (Custom for WebSocket, will be converted to Scrcpy format on server):
    // 0: Type (0x03)
    // 1-2: X (uint16)
    // 3-4: Y (uint16)
    // 5-6: hScroll (int16)
    // 7-8: vScroll (int16)
    // 9: Buttons (uint8)
    const buffer = new ArrayBuffer(10);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_SCROLL);
    view.setUint16(1, x);
    view.setUint16(3, y);
    view.setInt16(5, hScroll);
    view.setInt16(7, vScroll);
    view.setUint8(9, 0); // No buttons pressed
    return buffer;
}

function createRequestKeyFramePacket() {
    const buffer = new ArrayBuffer(2);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_RKF);
    view.setUint8(1, 0);

    return buffer;
}
