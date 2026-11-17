import { addControlButton } from './control_ui.js';

/**
 * UHID Virtual Gamepad
 * 集成虚拟摇杆 UI 与 UHID 通信协议
 */

// --- 1. UHID 协议常量与定义 ---

const UHID_GAMEPAD_MSG_CREATE = 12;
const UHID_GAMEPAD_MSG_INPUT = 13;
const UHID_GAMEPAD_MSG_DESTROY = 14;

const UHID_GAMEPAD_ID = 4;
const UHID_GAMEPAD_NAME = "Virtual Gamepad";

// 状态变量
let uhidGamepadEnabled = false;
let uhidGamepadInitialized = false;

// 手柄状态 (Buttons: 32 bits, Axes: 4 bytes, Hat: 4 bits)
// hat 取值: 0=N, 1=NE, 2=E, 3=SE, 4=S, 5=SW, 6=W, 7=NW, 8=Null(居中)
let gamepadState = {
    buttons: 0,
    x: 0,
    y: 0,
    z: 0,
    rz: 0,
    hat: 8
};

// 摇杆 → 方向键 (Hat) 的死区阈值 (像素, 相对摇杆中心)
// 越小越灵敏; maxDistance = 50px, 取 8px 约 16%, 刚出中心就能触发
const STICK_TO_HAT_DEADZONE = 8;

// 按键映射 (Bit positions)
const BTN_A = 0;
const BTN_B = 1;
const BTN_C = 2;
const BTN_X = 3;
const BTN_Y = 4;
const BTN_Z = 5;
const BTN_L1 = 6;
const BTN_R1 = 7;
const BTN_L2 = 8;
const BTN_R2 = 9;
const BTN_SELECT = 10;
const BTN_START = 11;
const BTN_MODE = 12;   // Home/Guide
const BTN_THUMBL = 13; // L3
const BTN_THUMBR = 14; // R3
// 15-31 预留给更多按键

// HID 描述符 (扩容至 32 个按键)
const GAMEPAD_REPORT_DESCRIPTOR = new Uint8Array([
    0x05, 0x01,        // Usage Page (Generic Desktop Ctrls)
    0x09, 0x05,        // Usage (Game Pad)
    0xA1, 0x01,        // Collection (Application)
    0xA1, 0x00,        //   Collection (Physical)

    // Buttons (32 buttons)
    0x05, 0x09,        //     Usage Page (Button)
    0x19, 0x01,        //     Usage Minimum (0x01)
    0x29, 0x20,        //     Usage Maximum (0x20 = 32) <--- 修改点
    0x15, 0x00,        //     Logical Minimum (0)
    0x25, 0x01,        //     Logical Maximum (1)
    0x75, 0x01,        //     Report Size (1)
    0x95, 0x20,        //     Report Count (32)         <--- 修改点
    0x81, 0x02,        //     Input (Data,Var,Abs)

    // Axes (4 axes: X, Y, Z, Rz)
    0x05, 0x01,        //     Usage Page (Generic Desktop Ctrls)
    0x09, 0x30,        //     Usage (X)
    0x09, 0x31,        //     Usage (Y)
    0x09, 0x32,        //     Usage (Z)
    0x09, 0x35,        //     Usage (Rz)
    0x15, 0x81,        //     Logical Minimum (-127)
    0x25, 0x7F,        //     Logical Maximum (127)
    0x75, 0x08,        //     Report Size (8)
    0x95, 0x04,        //     Report Count (4)
    0x81, 0x02,        //     Input (Data,Var,Abs)

    // Hat Switch (D-Pad) - 4 bits + 4 bits padding = 1 byte
    0x09, 0x39,        //     Usage (Hat switch)
    0x15, 0x00,        //     Logical Minimum (0)
    0x25, 0x07,        //     Logical Maximum (7)
    0x35, 0x00,        //     Physical Minimum (0)
    0x46, 0x3B, 0x01,  //     Physical Maximum (315)
    0x65, 0x14,        //     Unit (Eng Rot:Degrees)
    0x75, 0x04,        //     Report Size (4)
    0x95, 0x01,        //     Report Count (1)
    0x81, 0x42,        //     Input (Data,Var,Abs,Null State)
    // 4-bit padding 对齐到字节
    0x75, 0x04,        //     Report Size (4)
    0x95, 0x01,        //     Report Count (1)
    0x81, 0x03,        //     Input (Const,Var,Abs)
    0x65, 0x00,        //     Unit (None) - 复位单位

    0xC0,              //   End Collection
    0xC0               // End Collection
]);

// --- 2. 核心通信函数 ---

function initUHIDGamepad() {
    if (uhidGamepadInitialized) return;

    // 尝试先销毁旧设备 (如果存在)
    sendDataChannelMessage(window.dataChannelOrdered, createUHIDGamepadDestroyPacket());

    const packet = createUHIDGamepadCreatePacket();
    sendDataChannelMessage(window.dataChannelOrdered, packet);
    uhidGamepadInitialized = true;
    console.log("UHID Gamepad device created");
}

function destroyUHIDGamepad() {
    if (!uhidGamepadInitialized) return;

    const packet = createUHIDGamepadDestroyPacket();
    sendDataChannelMessage(window.dataChannelOrdered, packet);
    uhidGamepadInitialized = false;
    uhidGamepadEnabled = false;
    console.log("UHID Gamepad device destroyed");
}

// 暴露给外部调用的开关函数
function toggleUHIDGamepad() {
    const btn = document.getElementById('uhidGamepadToggleBtn');

    if (!uhidGamepadEnabled) {
        initUHIDGamepad();
        uhidGamepadEnabled = true;
        createVirtualGamepadUI();
        console.log("UHID Gamepad enabled");
        if (btn) btn.classList.add('active');
    } else {
        removeVirtualGamepadUI();
        destroyUHIDGamepad();
        uhidGamepadEnabled = false;
        console.log("UHID Gamepad disabled");
        if (btn) btn.classList.remove('active');
    }
};
addControlButton('uhidGamepadToggleBtn', 'UHID Gamepad',
    'M21 6H3c-1.1 0-1.99.9-1.99 2L1 16c0 1.1.9 2 2 2h18c1.1 0 1.99-.9 1.99-2V8c0-1.1-.89-2-1.99-2zm-10 7H8v3H6v-3H3v-2h3V8h2v3h3v2zm4.5 2c-.83 0-1.5-.67-1.5-1.5S14.67 12 15.5 12s1.5.67 1.5 1.5-.67 1.5-1.5 1.5zm4-3c-.83 0-1.5-.67-1.5-1.5S18.67 9 19.5 9s1.5.67 1.5 1.5-.67 1.5-1.5 1.5z')
    .addEventListener('click', toggleUHIDGamepad);

function sendGamepadReport() {
    if (!uhidGamepadEnabled || !uhidGamepadInitialized) return;

    const packet = createUHIDGamepadInputPacket(gamepadState);
    // if (window.ws && window.ws.readyState === WebSocket.OPEN) {
    //     window.ws.send(packet);
    // }
    sendDataChannelMessage(window.dataChannelUnordered, packet);
}

// --- 3. 协议包构造函数 ---

function createUHIDGamepadCreatePacket() {
    const encoder = new TextEncoder();
    const rawName = UHID_GAMEPAD_NAME;
    const nameBytes = encoder.encode(rawName).slice(0, 255);
    const descriptor = GAMEPAD_REPORT_DESCRIPTOR;

    // Type(1) + ID(2) + Vendor(2) + Product(2) + NameLen(1) + NameBytes + DescLen(2) + DescBytes
    const buffer = new ArrayBuffer(8 + nameBytes.length + 2 + descriptor.length);
    const view = new DataView(buffer);
    const uint8View = new Uint8Array(buffer);

    let offset = 0;
    view.setUint8(offset, UHID_GAMEPAD_MSG_CREATE); offset += 1;
    view.setUint16(offset, UHID_GAMEPAD_ID); offset += 2;
    view.setUint16(offset, 0x18d1); offset += 2; // Vendor: Google
    view.setUint16(offset, 0x0001); offset += 2; // Product
    view.setUint8(offset, nameBytes.length); offset += 1;

    if (nameBytes.length > 0) {
        uint8View.set(nameBytes, offset);
        offset += nameBytes.length;
    }

    view.setUint16(offset, descriptor.length); offset += 2;
    uint8View.set(descriptor, offset);

    return buffer;
}

function createUHIDGamepadInputPacket(state) {
    // Report Size: Buttons(4 bytes) + Axes(4 bytes) + Hat+padding(1 byte) = 9 bytes
    const reportSize = 9;
    const buffer = new ArrayBuffer(1 + 2 + 2 + reportSize);
    const view = new DataView(buffer);

    let offset = 0;
    view.setUint8(offset, UHID_GAMEPAD_MSG_INPUT); offset += 1;
    view.setUint16(offset, UHID_GAMEPAD_ID); offset += 2;
    view.setUint16(offset, reportSize); offset += 2;

    // HID Report Data
    // Buttons: 32 bits (4 bytes)
    view.setUint32(offset, state.buttons, true); offset += 4;

    // Axes: 4 bytes
    view.setInt8(offset, state.x); offset += 1;
    view.setInt8(offset, state.y); offset += 1;
    view.setInt8(offset, state.z); offset += 1;
    view.setInt8(offset, state.rz); offset += 1;

    // Hat switch: 低 4 位有效, 高 4 位为 padding (置 0)
    view.setUint8(offset, state.hat & 0x0F); offset += 1;

    return buffer;
}

function createUHIDGamepadDestroyPacket() {
    const buffer = new ArrayBuffer(3);
    const view = new DataView(buffer);
    view.setUint8(0, UHID_GAMEPAD_MSG_DESTROY);
    view.setUint16(1, UHID_GAMEPAD_ID);
    return buffer;
}

// --- 4. 虚拟手柄 UI 构建 ---

function createVirtualGamepadUI() {
    // 必须挂到视频容器内部, 这样进入全屏(.video-container 被 requestFullscreen)时
    // 叠加层会跟随进入全屏子树, 否则只显示在 body 上会被全屏元素遮住
    let container = document.querySelector('.video-container');
    if (!container) container = document.body;

    // gamepad 使用 position:absolute + 100% 尺寸, 容器必须是定位上下文
    const cs = getComputedStyle(container);
    if (cs.position === 'static') {
        container.style.position = 'relative';
    }

    // 注入 CSS 样式
    injectStyles();

    const gamepadDiv = document.createElement('div');
    gamepadDiv.id = 'virtual-gamepad';

    // --- HTML 结构构建 ---

    // 0. 顶部肩键 L / R (绝对定位在 gamepadDiv 左右上角)
    const btnL = createButton('L', 'gp-btn btn-shoulder shoulder-l', '70px', '32px', 'L');
    const btnR = createButton('R', 'gp-btn btn-shoulder shoulder-r', '70px', '32px', 'R');
    setupActionButton(btnL, BTN_L1);
    setupActionButton(btnR, BTN_R1);

    // 1. 左侧摇杆
    const leftControls = document.createElement('div');
    leftControls.className = 'gamepad-controls left';
    leftControls.id = 'joystick-base';
    const stick = document.createElement('div');
    stick.className = 'joystick-stick';
    stick.id = 'joystick-stick';
    leftControls.appendChild(stick);

    // 2. 中间功能键 (SELECT, L3, MENU, R3, START)
    const centerControls = document.createElement('div');
    centerControls.className = 'gamepad-controls center';

    const btnSelect = createButton('SELECT', 'gp-btn btn-system', '60px', '28px', 'SELECT', true);
    const btnL3 = createButton('L3', 'gp-btn btn-stick', '44px', '44px', 'L3', true);
    const btnHome = createButton('MENU', 'gp-btn btn-home', '54px', '36px', 'HOME', true);
    const btnR3 = createButton('R3', 'gp-btn btn-stick', '44px', '44px', 'R3', true);
    const btnStart = createButton('START', 'gp-btn btn-system', '60px', '28px', 'START', true);

    // 绑定事件
    setupActionButton(btnSelect, BTN_SELECT);
    setupActionButton(btnL3, BTN_THUMBL); // L3 -> THUMBL
    setupActionButton(btnHome, BTN_MODE); // MENU -> MODE
    setupActionButton(btnR3, BTN_THUMBR); // R3 -> THUMBR
    setupActionButton(btnStart, BTN_START);

    centerControls.appendChild(btnSelect);
    centerControls.appendChild(btnL3);
    centerControls.appendChild(btnHome);
    centerControls.appendChild(btnR3);
    centerControls.appendChild(btnStart);

    // 3. 右侧 ABXY
    const rightControls = document.createElement('div');
    rightControls.className = 'gamepad-controls right';

    // 布局位置 (相对于 180x180 容器)
    const btnY = createButton('Y', 'gp-btn btn-y', '60px', '60px', 'Y');
    Object.assign(btnY.style, { top: '0', left: '60px' });

    const btnA = createButton('A', 'gp-btn btn-a', '60px', '60px', 'A');
    Object.assign(btnA.style, { top: '120px', left: '60px' });

    const btnX = createButton('X', 'gp-btn btn-x', '60px', '60px', 'X');
    Object.assign(btnX.style, { top: '60px', left: '0' });

    const btnB = createButton('B', 'gp-btn btn-b', '60px', '60px', 'B');
    Object.assign(btnB.style, { top: '60px', left: '120px' });

    setupActionButton(btnA, BTN_A);
    setupActionButton(btnB, BTN_B);
    setupActionButton(btnX, BTN_X);
    setupActionButton(btnY, BTN_Y);

    rightControls.appendChild(btnY);
    rightControls.appendChild(btnA);
    rightControls.appendChild(btnX);
    rightControls.appendChild(btnB);

    // 组装
    gamepadDiv.appendChild(btnL);
    gamepadDiv.appendChild(btnR);
    gamepadDiv.appendChild(leftControls);
    gamepadDiv.appendChild(centerControls);
    gamepadDiv.appendChild(rightControls);

    container.appendChild(gamepadDiv);

    // 初始化摇杆逻辑
    initJoystickLogic(leftControls, stick);
}

function removeVirtualGamepadUI() {
    const gamepadDiv = document.getElementById('virtual-gamepad');
    if (gamepadDiv) {
        gamepadDiv.remove();
    }
    // 移除注入的样式? 通常没必要，留着也无妨
}

function createButton(text, className, w, h, keyName, releativePosition = false) {
    const btn = document.createElement('div');
    btn.innerText = text;
    btn.className = className;
    btn.style.width = w;
    btn.style.height = h;
    if (releativePosition) {
        btn.style.position = 'relative';
    }
    // btn.style.position = 'absolute';
    if (keyName) btn.setAttribute('data-key', keyName);
    return btn;
}

function injectStyles() {
    if (document.getElementById('gamepad-styles')) return;
    const style = document.createElement('style');
    style.id = 'gamepad-styles';
    style.innerHTML = `
            #virtual-gamepad {
                position: absolute; top: 0; left: 0; width: 100%; height: 100%;
                pointer-events: none; z-index: 100;
                display: flex; justify-content: space-between; align-items: flex-end;
                padding: 40px; box-sizing: border-box;
                user-select: none; touch-action: none;
            }
            .gamepad-controls { pointer-events: auto; position: relative; }
            .gamepad-controls.left {
                width: 180px; height: 180px;
                background: rgba(255, 255, 255, 0.1); border-radius: 50%;
                margin-bottom: 20px; backdrop-filter: blur(2px);
                border: 2px solid rgba(255,255,255,0.15);
                display: flex; justify-content: center; align-items: center;
            }
            .gamepad-controls.right { width: 180px; height: 180px; margin-bottom: 20px; }
            .gamepad-controls.center {
                position: absolute; bottom: 30px; left: 50%; transform: translateX(-50%);
                display: flex; gap: 14px; align-items: center; pointer-events: auto;
            }
            .joystick-stick {
                width: 80px; height: 80px;
                background: radial-gradient(circle at 30% 30%, #555, #222);
                border-radius: 50%; border: 2px solid rgba(255,255,255,0.3);
                box-shadow: 0 5px 15px rgba(0,0,0,0.5);
                position: absolute; transform: translate(0px, 0px); cursor: pointer;
                transition: transform 0.1s;
            }
            .joystick-stick.active { background: radial-gradient(circle at 30% 30%, #666, #333); transition: none; }
            .gp-btn {
                position: absolute; display: flex; justify-content: center; align-items: center;
                color: white; font-weight: bold; cursor: pointer;
                border: 1px solid rgba(255, 255, 255, 0.3);
                background-color: rgba(255, 255, 255, 0.15);
                border-radius: 50%; transition: transform 0.1s;
                box-shadow: 0 4px 6px rgba(0,0,0,0.3);
                text-shadow: 0 1px 2px rgba(0,0,0,0.5);
            }
            .gp-btn:active, .gp-btn.active { transform: scale(0.95); background-color: rgba(255, 255, 255, 0.4) !important; }
            .btn-y { background-color: rgba(255, 200, 0, 0.25); color: #ffd700; }
            .btn-a { background-color: rgba(0, 255, 100, 0.25); color: #00ff66; }
            .btn-x { background-color: rgba(0, 100, 255, 0.25); color: #00bfff; }
            .btn-b { background-color: rgba(255, 50, 50, 0.25); color: #ff4444; }
            .btn-home { background-color: rgba(255, 255, 255, 0.1); border-radius: 12px; font-size: 12px;}
            .btn-stick { background-color: rgba(200, 200, 200, 0.2); font-size: 12px;}
            .btn-system {
                background-color: rgba(200, 200, 200, 0.18);
                border-radius: 14px !important;
                font-size: 11px; letter-spacing: 0.5px;
            }
            .btn-shoulder {
                pointer-events: auto;
                border-radius: 10px !important;
                font-size: 16px; letter-spacing: 1px;
                background-color: rgba(150, 170, 220, 0.22);
            }
            .btn-shoulder.shoulder-l { top: 30px; left: 40px; }
            .btn-shoulder.shoulder-r { top: 30px; right: 40px; }
        `;
    document.head.appendChild(style);
}

// --- 5. 事件绑定逻辑 ---

function setupActionButton(element, btnIndex) {
    const handleDown = (e) => {
        e.preventDefault();
        element.classList.add('active');
        if (navigator.vibrate) navigator.vibrate(10);

        gamepadState.buttons |= (1 << btnIndex);
        sendGamepadReport();
    };
    const handleUp = (e) => {
        e.preventDefault();
        element.classList.remove('active');

        gamepadState.buttons &= ~(1 << btnIndex);
        sendGamepadReport();
    };

    element.addEventListener('mousedown', handleDown);
    element.addEventListener('mouseup', handleUp);
    element.addEventListener('mouseleave', handleUp);
    element.addEventListener('touchstart', handleDown);
    element.addEventListener('touchend', handleUp);
}

// 将摇杆位移(像素)换算为 8 向 Hat 值, 用于兼容不响应线性摇杆的应用
// dx, dy 为相对中心的像素偏移 (dy 正方向朝下, 与屏幕坐标一致)
function computeHatFromStick(dx, dy) {
    const dist = Math.sqrt(dx * dx + dy * dy);
    if (dist < STICK_TO_HAT_DEADZONE) return 8; // Null state

    // atan2(x, -y): 以正北为 0, 顺时针为正
    // 把 dy 取反, 因为屏幕 y 向下而我们要"北"为上
    let deg = Math.atan2(dx, -dy) * 180 / Math.PI;
    if (deg < 0) deg += 360;
    // 每 45° 一个方向, 偏移 22.5° 让边界落在扇区中点之间
    return Math.floor(((deg + 22.5) % 360) / 45);
}

function initJoystickLogic(base, stick) {
    let isDragging = false;
    const maxDistance = 50; // px

    function getCoords(e) {
        if (e.touches && e.touches.length > 0) return { x: e.touches[0].clientX, y: e.touches[0].clientY };
        return { x: e.clientX, y: e.clientY };
    }

    function updateJoystick(e, centerX, centerY) {
        const coords = getCoords(e);
        let dx = coords.x - centerX;
        let dy = coords.y - centerY;

        const distance = Math.sqrt(dx * dx + dy * dy);
        if (distance > maxDistance) {
            const ratio = maxDistance / distance;
            dx *= ratio;
            dy *= ratio;
        }

        stick.style.transform = `translate(${dx}px, ${dy}px)`;

        // 线性摇杆轴: -127 ~ 127 (Y 轴向下为正, 与屏幕坐标一致)
        gamepadState.x = Math.round((dx / maxDistance) * 127);
        gamepadState.y = Math.round((dy / maxDistance) * 127);

        // 同步驱动 Hat (方向键) 以兼容只识别 D-pad 的应用
        gamepadState.hat = computeHatFromStick(dx, dy);

        sendGamepadReport();
    }

    function start(e) {
        e.preventDefault();
        isDragging = true;
        stick.classList.add('active');
        const rect = base.getBoundingClientRect();
        const centerX = rect.left + rect.width / 2;
        const centerY = rect.top + rect.height / 2;
        updateJoystick(e, centerX, centerY);
    }

    function move(e) {
        if (!isDragging) return;
        e.preventDefault();
        const rect = base.getBoundingClientRect();
        updateJoystick(e, rect.left + rect.width / 2, rect.top + rect.height / 2);
    }

    function end(e) {
        e.preventDefault();
        isDragging = false;
        stick.classList.remove('active');
        stick.style.transform = `translate(0px, 0px)`;

        gamepadState.x = 0;
        gamepadState.y = 0;
        gamepadState.hat = 8; // Null state
        sendGamepadReport();
    }

    base.addEventListener('mousedown', start);
    document.addEventListener('mousemove', move);
    document.addEventListener('mouseup', end);

    base.addEventListener('touchstart', start);
    base.addEventListener('touchmove', move);
    base.addEventListener('touchend', end);
    base.addEventListener('touchcancel', end);
}

