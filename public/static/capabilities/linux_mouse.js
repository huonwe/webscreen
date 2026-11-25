// 动作常量定义 (必须与 Go 后端 InputController 保持一致)
// Go端: 0=Down, 1=Up, 2=Move
const TYPE_MOUSE = 0x01;
const TYPE_TOUCH = 0x02;

const MOUSE_ACTION_DOWN = 0;
const MOUSE_ACTION_UP = 1;
const MOUSE_ACTION_MOVE = 2;

/**
 * Remote Control Mouse Handler
 */

// 配置项
const MOUSE_SENSITIVITY = 1.0;

// 状态变量
let isPointerLocked = false;
let mouseButtonsMask = 0; // Bitmask: 1=Left, 2=Right, 4=Middle
let pendingMovement = { x: 0, y: 0, wheelY: 0 };
let rafScheduled = false;

let accumX = 0;
let accumY = 0;

function getMouseButtonMask(button) {
    switch (button) {
        case 0: return 1;  // Left
        case 2: return 2;  // Right
        case 1: return 4;  // Middle
        case 3: return 8;  // Side Back (X1)
        case 4: return 16; // Side Forward (X2)
        default: return 0;
    }
}

function initRemoteControl() {
    if (!remoteCanvas) {
        console.error(`Video element not found.`);
        return;
    }

    // 防止重复初始化
    if (window.mouseControlInitialized) return;
    window.mouseControlInitialized = true;

    // 1. 点击视频区域请求锁定鼠标
    remoteCanvas.addEventListener('mousedown', (e) => {
        if (!isPointerLocked) {
            console.log("Requesting pointer lock...");
            requestPointerLock();
        }
    });

    // 2. 监听指针锁定状态变化
    document.addEventListener('pointerlockchange', handlePointerLockChange);
    document.addEventListener('mozpointerlockchange', handlePointerLockChange);
    document.addEventListener('webkitpointerlockchange', handlePointerLockChange);

    // 3. 注册鼠标事件监听
    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mousedown', handleMouseDown);
    document.addEventListener('mouseup', handleMouseUp);
    document.addEventListener('wheel', handleWheel, { passive: false });
    // 鼠标侧键
    document.addEventListener('auxclick', (e) => {
    if (isPointerLocked) {
        e.preventDefault();
        e.stopPropagation();
    }
});

    // 4. 阻止右键菜单
    document.addEventListener('contextmenu', (e) => {
        if (isPointerLocked) {
            e.preventDefault();
            e.stopPropagation();
        }
    });

    console.log("Remote control initialized.");
}


function handlePointerLockChange() {
    const lockedElement = document.pointerLockElement ||
        document.mozPointerLockElement ||
        document.webkitPointerLockElement;

    if (lockedElement === remoteCanvas) {
        isPointerLocked = true;
    } else {
        isPointerLocked = false;
        // 退出指针锁定时，若仍有按键未抬起，强制补发 UP 防止远端卡键
        if (mouseButtonsMask !== 0) {
            for (const bit of [1, 2, 4, 8, 16]) {
                if (mouseButtonsMask & bit) {
                    sendControlPacket(MOUSE_ACTION_UP, 0, 0, bit, 0);
                }
            }
            mouseButtonsMask = 0;
        }
        pendingMovement.x = 0;
        pendingMovement.y = 0;
        pendingMovement.wheelY = 0;
        accumX = 0;
        accumY = 0;
    }
}


function handleMouseMove(e) {
    if (!isPointerLocked) return;

    const dx = e.movementX || e.mozMovementX || e.webkitMovementX || 0;
    const dy = e.movementY || e.mozMovementY || e.webkitMovementY || 0;

    pendingMovement.x += dx;
    pendingMovement.y += dy;

    scheduleSend(MOUSE_ACTION_MOVE);
}

function handleMouseDown(e) {
    if (!isPointerLocked) return;
    e.preventDefault();
    e.stopPropagation();

    flushPendingEvents(MOUSE_ACTION_MOVE);

    const pressingMask = getMouseButtonMask(e.button);
    if (pressingMask === 0) return;

    mouseButtonsMask |= pressingMask;
    sendControlPacket(MOUSE_ACTION_DOWN, 0, 0, pressingMask, 0);
}

function handleMouseUp(e) {
    if (!isPointerLocked) return;
    e.preventDefault();
    e.stopPropagation();

    flushPendingEvents(MOUSE_ACTION_MOVE);

    const releasingMask = getMouseButtonMask(e.button);
    if (releasingMask === 0) return;

    sendControlPacket(MOUSE_ACTION_UP, 0, 0, releasingMask, 0);
    mouseButtonsMask &= ~releasingMask;
}

function handleWheel(e) {
    if (!isPointerLocked) return;
    e.preventDefault();

    // 保持原始方向（向下为 +1，向上为 -1），由 Go 后端统一转换为 Windows 符号
    pendingMovement.wheelY += Math.sign(e.deltaY);
    scheduleSend(MOUSE_ACTION_MOVE);
}


function scheduleSend(actionType) {
    // 如果是 RAF 调度，我们需要保存最后一次的 actionType 吗？
    // 简化起见，移动和滚动统一走 RAF，点击直接发
    // 这里传入 actionType 主要是为了区分是否是移动
    // flushPendingEvents(MOUSE_ACTION_MOVE);
    if (rafScheduled) return;
    rafScheduled = true;

    requestAnimationFrame(() => {
        rafScheduled = false;
        flushPendingEvents(MOUSE_ACTION_MOVE);
    });
}

/**
 * 发送事件 (增加了 buttonsOverride 参数)
 * @param {number} actionType 
 * @param {number} buttonsOverride - 可选，强制指定发送的按键掩码
 */
function flushPendingEvents(actionType, buttonsOverride) {
    if (actionType === MOUSE_ACTION_MOVE &&
        pendingMovement.x === 0 &&
        pendingMovement.y === 0 &&
        pendingMovement.wheelY === 0) {
        return;
    }

    accumX += pendingMovement.x * MOUSE_SENSITIVITY;
    accumY += pendingMovement.y * MOUSE_SENSITIVITY;
    const dx = Math.trunc(accumX);
    const dy = Math.trunc(accumY);
    accumX -= dx;
    accumY -= dy;

    const wheel = pendingMovement.wheelY;
    const buttons = (buttonsOverride !== undefined) ? buttonsOverride : mouseButtonsMask;

    if (dx !== 0 || dy !== 0 || wheel !== 0 || actionType !== MOUSE_ACTION_MOVE) {
        sendControlPacket(actionType, dx, dy, buttons, wheel);
    }

    pendingMovement.x = 0;
    pendingMovement.y = 0;
    pendingMovement.wheelY = 0;
}
/**
 * 实际的网络发送逻辑
 */
function sendControlPacket(action, dx, dy, buttons, wheel) {
    const packet = createMousePacket(action, dx, dy, buttons, 0, wheel);
    sendReliableOrUnreliable(action, packet, wheel !== 0);
}


/**
 * 创建鼠标事件数据包 (18字节)
 */
function createMousePacket(action, deltaX, deltaY, buttons, wheelDeltaX = 0, wheelDeltaY = 0) {
    const buffer = new ArrayBuffer(18);
    const view = new DataView(buffer);

    view.setUint8(0, TYPE_MOUSE); // Type
    view.setUint8(1, action);     // Action

    // 直接写入相对位移，负数补码会被正确处理
    view.setInt32(2, deltaX, false); // BigEndian
    view.setInt32(6, deltaY, false); // BigEndian

    view.setUint32(10, buttons, false);

    // 滚轮也是有方向的，使用 Int16
    view.setInt16(14, wheelDeltaX, false);
    view.setInt16(16, wheelDeltaY, false);

    return buffer;
}

function sendReliableOrUnreliable(action, packet, hasWheel) {
    const isPureMove = (action === MOUSE_ACTION_MOVE && !hasWheel);
    if (isPureMove && window.dataChannelUnordered) {
        window.sendDataChannelMessage(window.dataChannelUnordered, packet);
    } else if (window.dataChannelOrdered) {
        window.sendDataChannelMessage(window.dataChannelOrdered, packet);
    } else if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(packet);
    }
}

// 如果在全屏模式下，尝试锁定键盘以拦截 Escape 和 F11，防止意外退出全屏或打断指针锁定
document.addEventListener('fullscreenchange', async () => {
    if (!navigator.keyboard || typeof navigator.keyboard.lock !== 'function') return;

    if (document.fullscreenElement) {
        try {
            // 全屏时接管 Escape 和 F11，防止单按 Esc 退出全屏或打断指针锁定
            await navigator.keyboard.lock(["Escape", "F11"]);
        } catch (err) {
            console.warn("Keyboard lock failed:", err);
        }
    } else {
        navigator.keyboard.unlock();
    }
});

// 启动
initRemoteControl();
