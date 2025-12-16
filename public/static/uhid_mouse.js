const videoElementMouse = document.getElementById('remoteVideo');
const SC_CONTROL_MSG_TYPE_UHID_CREATE = 12;
const SC_CONTROL_MSG_TYPE_UHID_INPUT = 13;
const SC_CONTROL_MSG_TYPE_UHID_DESTROY = 14; // 注意：并非所有版本都公开支持 Destroy 消息，如果没有只能靠断开连接

// UHID 鼠标状态
let uhidMouseEnabled = false;
let uhidMouseInitialized = false;
let lastMouseX = 0;
let lastMouseY = 0;
let mouseButtons = 0; // bit 0=左键, bit 1=右键, bit 2=中键

// 鼠标移动累积（用于批量发送）
let pendingMouseMove = { deltaX: 0, deltaY: 0, wheel: 0 };
let mouseRafScheduled = false;

// 自定义 UHID 设备 ID，避免与系统默认设备冲突
// 范围 0-65535，选择一个不太可能被占用的值
const UHID_DEVICE_ID = 2222;
const UHID_DEVICE_NAME = "";

function initUHIDMouse() {
    if (uhidMouseInitialized) {
        console.log("UHID Mouse already initialized");
        return;
    }
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        // window.ws.send(createUHIDDestroyPacket(1)); // 确保之前的设备被销毁
        window.ws.send(createUHIDDestroyPacket());
        console.log("UHID Mouse device destroyed (re-initializing)");
    } else {
        console.warn("WebSocket is not open. Cannot initialize UHID Mouse.");
    }
    
    const packet = createUHIDCreatePacket();
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
        uhidMouseInitialized = true;
        console.log("UHID Mouse device created");
    } else {
        console.warn("WebSocket is not open. Cannot initialize UHID Mouse.");
    }
}

function destroyUHIDMouse() {
    if (!uhidMouseInitialized) {
        return;
    }

    const packet = createUHIDDestroyPacket();
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
        uhidMouseInitialized = false;
        uhidMouseEnabled = false;
        console.log("UHID Mouse device destroyed");
    }
}

function toggleUHIDMouse() {
    if (!uhidMouseEnabled) {
        initUHIDMouse();
        uhidMouseEnabled = true;
        console.log("UHID Mouse enabled - relative mouse mode");
    } else {
        destroyUHIDMouse();
        uhidMouseEnabled = false;
        console.log("UHID Mouse disabled - touch mode");
    }
}

function sendMouseMove() {
    if (!uhidMouseEnabled || !uhidMouseInitialized) return;

    if (pendingMouseMove.deltaX !== 0 || pendingMouseMove.deltaY !== 0 || pendingMouseMove.wheel !== 0) {
        const packet = createUHIDInputPacket(
            mouseButtons,
            pendingMouseMove.deltaX,
            pendingMouseMove.deltaY,
            pendingMouseMove.wheel
        );

        if (window.ws && window.ws.readyState === WebSocket.OPEN) {
            window.ws.send(packet);
        }

        // 重置累积值
        pendingMouseMove.deltaX = 0;
        pendingMouseMove.deltaY = 0;
        pendingMouseMove.wheel = 0;
    }
}

function scheduleMouseSend() {
    if (!mouseRafScheduled) {
        mouseRafScheduled = true;
        requestAnimationFrame(() => {
            mouseRafScheduled = false;
            sendMouseMove();
        });
    }
}

// ========== UHID 鼠标事件处理 ==========

videoElementMouse.addEventListener('mousedown', (event) => {
    if (!uhidMouseEnabled) return;

    event.preventDefault();
    event.stopPropagation();

    // 更新按键状态
    if (event.button === 0) mouseButtons |= 0x01; // 左键
    if (event.button === 1) mouseButtons |= 0x04; // 中键
    if (event.button === 2) mouseButtons |= 0x02; // 右键

    // 发送按键状态变化
    const packet = createUHIDInputPacket(mouseButtons, 0, 0, 0);
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
    }
});

videoElementMouse.addEventListener('mouseup', (event) => {
    if (!uhidMouseEnabled) return;

    event.preventDefault();
    event.stopPropagation();

    // 清除按键状态
    if (event.button === 0) mouseButtons &= ~0x01; // 左键
    if (event.button === 1) mouseButtons &= ~0x04; // 中键
    if (event.button === 2) mouseButtons &= ~0x02; // 右键

    // 发送按键状态变化
    const packet = createUHIDInputPacket(mouseButtons, 0, 0, 0);
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
    }
});

videoElementMouse.addEventListener('mousemove', (event) => {
    if (!uhidMouseEnabled) return;

    // 锁定模式下只信赖 movementX/Y
    let dx = event.movementX;
    let dy = event.movementY;

    // 如果浏览器不支持 movementX (极少见)，才考虑 fallback，但在锁定模式下 fallback 基本无效
    if (dx === undefined) dx = 0;
    if (dy === undefined) dy = 0;

    pendingMouseMove.deltaX += dx;
    pendingMouseMove.deltaY += dy;

    scheduleMouseSend();
});

// 右键菜单拦截（在 UHID 模式下）
videoElementMouse.addEventListener('contextmenu', (event) => {
    if (uhidMouseEnabled) {
        event.preventDefault();
        event.stopPropagation();
    }
});

// 滚轮事件已经在 scroll.js 中处理，但我们也可以在 UHID 模式下捕获
// 注意：需要根据模式选择使用哪种滚轮处理方式
videoElementMouse.addEventListener('wheel', (event) => {
    if (!uhidMouseEnabled) return; // 让 scroll.js 处理

    event.preventDefault();
    event.stopPropagation();

    // 将滚轮值归一化到 -127 到 127
    const wheelDelta = -Math.sign(event.deltaY) * Math.min(Math.abs(event.deltaY) / 10, 127);

    pendingMouseMove.wheel += Math.round(wheelDelta);
    scheduleMouseSend();
}, { passive: false });

// 指针锁定 API (Pointer Lock) - 用于更好的鼠标控制
function requestPointerLock() {
    if (!uhidMouseEnabled) return;

    const requestMethod = videoElementMouse.requestPointerLock ||
        videoElementMouse.mozRequestPointerLock ||
        videoElementMouse.webkitRequestPointerLock;

    if (requestMethod) {
        requestMethod.call(videoElementMouse);
    }
}

function exitPointerLock() {
    const exitMethod = document.exitPointerLock ||
        document.mozExitPointerLock ||
        document.webkitExitPointerLock;

    if (exitMethod) {
        exitMethod.call(document);
    }
}

// 双击切换指针锁定
videoElementMouse.addEventListener('dblclick', (event) => {
    if (!uhidMouseEnabled) return;

    event.preventDefault();

    if (document.pointerLockElement === videoElementMouse ||
        document.mozPointerLockElement === videoElementMouse ||
        document.webkitPointerLockElement === videoElementMouse) {
        exitPointerLock();
    } else {
        requestPointerLock();
    }
});

// 监听指针锁定状态变化
document.addEventListener('pointerlockchange', () => {
    if (document.pointerLockElement === videoElementMouse) {
        console.log("Pointer locked - relative mouse mode active");
    } else {
        console.log("Pointer unlocked");
    }
});

document.addEventListener('pointerlockerror', () => {
    console.error("Pointer lock failed");
});

// UHID Mouse HID Report Descriptor (标准鼠标描述符)
// const MOUSE_REPORT_DESCRIPTOR = new Uint8Array([
//     // Usage Page (Generic Desktop)
//     0x05, 0x01,
//     // Usage (Mouse)
//     0x09, 0x02,

//     // Collection (Application)
//     0xA1, 0x01,

//     // Usage (Pointer)
//     0x09, 0x01,

//     // Collection (Physical)
//     0xA1, 0x00,

//     // Usage Page (Buttons)
//     0x05, 0x09,

//     // Usage Minimum (1)
//     0x19, 0x01,
//     // Usage Maximum (5)
//     0x29, 0x05,
//     // Logical Minimum (0)
//     0x15, 0x00,
//     // Logical Maximum (1)
//     0x25, 0x01,
//     // Report Count (5)
//     0x95, 0x05,
//     // Report Size (1)
//     0x75, 0x01,
//     // Input (Data, Variable, Absolute): 5 buttons bits
//     0x81, 0x02,

//     // Report Count (1)
//     0x95, 0x01,
//     // Report Size (3)
//     0x75, 0x03,
//     // Input (Constant): 3 bits padding
//     0x81, 0x01,

//     // Usage Page (Generic Desktop)
//     0x05, 0x01,
//     // Usage (X)
//     0x09, 0x30,
//     // Usage (Y)
//     0x09, 0x31,
//     // Usage (Wheel)
//     0x09, 0x38,
//     // Logical Minimum (-127)
//     0x15, 0x81,
//     // Logical Maximum (127)
//     0x25, 0x7F,
//     // Report Size (8)
//     0x75, 0x08,
//     // Report Count (3)
//     0x95, 0x03,
//     // Input (Data, Variable, Relative): 3 position bytes (X, Y, Wheel)
//     0x81, 0x06,

//     // Usage Page (Consumer Page)
//     0x05, 0x0C,
//     // Usage(AC Pan)
//     0x0A, 0x38, 0x02,
//     // Logical Minimum (-127)
//     0x15, 0x81,
//     // Logical Maximum (127)
//     0x25, 0x7F,
//     // Report Size (8)
//     0x75, 0x08,
//     // Report Count (1)
//     0x95, 0x01,
//     // Input (Data, Variable, Relative): 1 byte (AC Pan)
//     0x81, 0x06,

//     // End Collection
//     0xC0,

//     // End Collection
//     0xC0,
// ]);

// Scrcpy 官方对应的 8-bit 鼠标描述符
// 兼容性最强版本：带 Report ID 的描述符
const MOUSE_REPORT_DESCRIPTOR = new Uint8Array([
    0x05, 0x01, // Usage Page (Generic Desktop)
    0x09, 0x02, // Usage (Mouse)
    0xA1, 0x01, // Collection (Application)
    0x09, 0x01, // Usage (Pointer)
    0xA1, 0x00, // Collection (Physical)
    
    // Buttons (3 buttons)
    0x05, 0x09, // Usage Page (Buttons)
    0x19, 0x01, // Usage Minimum (1)
    0x29, 0x05, // Usage Maximum (5) - Scrcpy定义了5个键
    0x15, 0x00, // Logical Minimum (0)
    0x25, 0x01, // Logical Maximum (1)
    0x95, 0x05, // Report Count (5)
    0x75, 0x01, // Report Size (1)
    0x81, 0x02, // Input (Data, Variable, Absolute)
    
    // Padding (3 bits) - 凑齐 1 字节
    0x95, 0x01, // Report Count (1)
    0x75, 0x03, // Report Size (3)
    0x81, 0x01, // Input (Constant)
    
    // X, Y, Wheel (8-bit)
    0x05, 0x01, // Usage Page (Generic Desktop)
    0x09, 0x30, // Usage (X)
    0x09, 0x31, // Usage (Y)
    0x09, 0x38, // Usage (Wheel)
    0x15, 0x81, // Logical Minimum (-127)
    0x25, 0x7F, // Logical Maximum (127)
    0x75, 0x08, // Report Size (8)
    0x95, 0x03, // Report Count (3) -> X, Y, Wheel
    0x81, 0x06, // Input (Data, Variable, Relative)
    
    // H-Wheel (AC Pan)
    0x05, 0x0C, // Usage Page (Consumer)
    0x0A, 0x38, 0x02, // Usage (AC Pan)
    0x15, 0x81, // Logical Min (-127)
    0x25, 0x7F, // Logical Max (127)
    0x75, 0x08, // Report Size (8)
    0x95, 0x01, // Report Count (1)
    0x81, 0x06, // Input (Data, Var, Rel)

    0xC0, 0xC0  // End Collection
]);
// 前端 JavaScript 修改
function createUHIDCreatePacket(deviceID = UHID_DEVICE_ID) {
    // 1. 编码名字
    const encoder = new TextEncoder();
    // 如果名字太长，截断到 255 字节 (因为长度只有 1 字节)
    const rawName = UHID_DEVICE_NAME || "Scrcpy Mouse";
    const nameBytes = encoder.encode(rawName).slice(0, 255); 
    const descriptor = MOUSE_REPORT_DESCRIPTOR;

    // 2. 计算大小
    // Header: Type(1) + ID(2) + Vendor(2) + Prod(2) + NameSize(1) = 8 字节
    // DescSize(2)
    const buffer = new ArrayBuffer(8 + nameBytes.length + 2 + descriptor.length);
    const view = new DataView(buffer);
    const uint8View = new Uint8Array(buffer);
    
    let offset = 0;
    
    // Type
    view.setUint8(offset, SC_CONTROL_MSG_TYPE_UHID_CREATE); offset += 1;
    // ID
    view.setUint16(offset, deviceID); offset += 2;
    // Vendor (0)
    view.setUint16(offset, 0x18d1); offset += 2;
    // Product (0)
    view.setUint16(offset, 0x0001); offset += 2;
    
    // --- 关键修改 ---
    // Name Size (改成 1 字节!)
    view.setUint8(offset, nameBytes.length); offset += 1;
    console.log("UHID Device Name Size:", nameBytes.length);
    // Name Data
    if (nameBytes.length > 0) {
        uint8View.set(nameBytes, offset);
        offset += nameBytes.length;
    }
    
    // Desc Size (Java代码里 parseByteArray(2) 表示这里还是 2 字节)
    view.setUint16(offset, descriptor.length); offset += 2;
    
    // Desc Data
    uint8View.set(descriptor, offset);

    return buffer;
}
function createUHIDInputPacket(buttons, deltaX, deltaY, wheel = 0, hWheel = 0, deviceID = UHID_DEVICE_ID) {
    // Scrcpy 协议结构 (Type 13):
    // [0]    Type (1 byte)
    // [1-2]  Device ID (2 bytes)
    // [3-4]  Report Size (2 bytes)
    // [5...] HID Report Data (5 bytes)

    const reportSize = 5; // 变更为 5 字节
    const buffer = new ArrayBuffer(1 + 2 + 2 + reportSize);
    const view = new DataView(buffer);

    let offset = 0;

    // --- Scrcpy 协议头 ---
    view.setUint8(offset, SC_CONTROL_MSG_TYPE_UHID_INPUT); offset += 1;
    view.setUint16(offset, deviceID); offset += 2;
    view.setUint16(offset, reportSize); offset += 2;

    // --- HID Report 内容 (5 Bytes) ---
    // Byte 0: Buttons
    // view.setUint8(offset, 1); offset += 1;
    view.setUint8(offset, buttons); offset += 1;

    // Byte 1: X (8-bit signed)
    // 必须 Clamp 到 -127 ~ 127，否则数据溢出会导致反向移动
    view.setInt8(offset, Math.max(-127, Math.min(127, deltaX))); offset += 1;

    // Byte 2: Y (8-bit signed)
    view.setInt8(offset, Math.max(-127, Math.min(127, deltaY))); offset += 1;

    // Byte 3: Wheel (8-bit signed)
    view.setInt8(offset, Math.max(-127, Math.min(127, wheel))); offset += 1;

    // Byte 4: HWheel (8-bit signed)
    view.setInt8(offset, Math.max(-127, Math.min(127, hWheel))); offset += 1;

    return buffer;
}

function createUHIDDestroyPacket(deviceID = UHID_DEVICE_ID) {
    // UHID Destroy Packet Structure:
    // 0: Type (0x0E = 14)
    // 1-2: Device ID (uint16)

    const buffer = new ArrayBuffer(3);
    const view = new DataView(buffer);

    view.setUint8(0, SC_CONTROL_MSG_TYPE_UHID_DESTROY);
    view.setUint16(1, deviceID);

    return buffer;
}
