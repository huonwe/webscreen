// 【重要】动作常量定义 (必须与 Go 后端 InputController 保持一致)
// Go端: 0=Move, 1=Down, 2=Up
const TYPE_MOUSE = 0x01;
const MOUSE_ACTION_MOVE = 0;
const MOUSE_ACTION_DOWN = 1;
const MOUSE_ACTION_UP   = 2;

// var mouse_x = 0;
// var mouse_y = 0;
let virtualMouse = { 
    x: 0, 
    y: 0, 
    wheelY: 0 
};
/**
 * Remote Control Mouse Handler
 */

// 配置项
const MOUSE_SENSITIVITY = 1.0; 

// 状态变量
let isPointerLocked = false;
let mouseButtonsMask = 0; // Bitmask: 1=Left, 2=Right, 4=Middle
let pendingMovement = { x: 0, y: 0, wheelY: 0 };
// let rafScheduled = false;

// 获取视频元素
const videoElementMouse = document.getElementById('remoteVideo');

function initRemoteControl() {
    if (!videoElementMouse) {
        console.error(`Video element not found.`);
        return;
    }

    // 防止重复初始化
    if (window.mouseControlInitialized) return;
    window.mouseControlInitialized = true;

    // 1. 点击视频区域请求锁定鼠标
    videoElementMouse.addEventListener('mousedown', (e) => {
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
    
    // 4. 阻止右键菜单
    document.addEventListener('contextmenu', (e) => {
        if (isPointerLocked) {
            e.preventDefault();
            e.stopPropagation();
        }
    });

    console.log("Remote control initialized.");
}

function requestPointerLock() {
    const requestMethod = videoElementMouse.requestPointerLock ||
                          videoElementMouse.mozRequestPointerLock ||
                          videoElementMouse.webkitRequestPointerLock;
    if (requestMethod) {
        requestMethod.call(videoElementMouse);
    }
}

function handlePointerLockChange() {
    const lockedElement = document.pointerLockElement ||
                          document.mozPointerLockElement ||
                          document.webkitPointerLockElement;

    if (lockedElement === videoElementMouse) {
        isPointerLocked = true;
        console.log(">> Mouse LOCKED <<");
    } else {
        isPointerLocked = false;
        console.log(">> Mouse UNLOCKED <<");
        resetMouseState();
    }
}

function resetMouseState() {
    mouseButtonsMask = 0;
    pendingMovement = { x: 0, y: 0, wheelY: 0 };
    // 发送 Move 事件归零
    sendControlPacket(MOUSE_ACTION_MOVE, 0, 0, 0, 0); 
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
    e.preventDefault(); e.stopPropagation();

    switch(e.button) {
        case 0: mouseButtonsMask |= 1; break; // Left
        case 2: mouseButtonsMask |= 2; break; // Right (Go端需映射为3)
        case 1: mouseButtonsMask |= 4; break; // Middle
    }

    // 点击立即发送，不走 RAF 延迟
    flushPendingEvents(MOUSE_ACTION_DOWN); 
}

/**
 * 处理鼠标抬起
 */
function handleMouseUp(e) {
    if (!isPointerLocked) return;
    e.preventDefault(); e.stopPropagation();

    // 1. 先计算出当前正在被抬起的按键掩码
    let releasingMask = 0;
    switch(e.button) {
        case 0: releasingMask = 1; break; // Left
        case 2: releasingMask = 2; break; // Right
        case 1: releasingMask = 4; break; // Middle
    }

    // 2. 发送 UP 事件
    // 【关键】这里我们传入 releasingMask，告诉后端是"这个键"抬起了
    // 如果传入全局的 mouseButtonsMask，那时它已经被清除了，后端就不知道谁抬起了
    flushPendingEvents(MOUSE_ACTION_UP, releasingMask);

    // 3. 事件发送后再更新全局状态 (清除对应的位)
    mouseButtonsMask &= ~releasingMask;
}

function handleWheel(e) {
    if (!isPointerLocked) return;
    e.preventDefault();

    // 归一化滚轮
    const delta = -Math.sign(e.deltaY); 
    
    pendingMovement.wheelY += delta;
    scheduleSend(MOUSE_ACTION_MOVE); // 滚轮视为带 Wheel 数据的 Move
}

function scheduleSend(actionType) {
    // 如果是 RAF 调度，我们需要保存最后一次的 actionType 吗？
    // 简化起见，移动和滚动统一走 RAF，点击直接发
    // 这里传入 actionType 主要是为了区分是否是移动
    flushPendingEvents(MOUSE_ACTION_MOVE);
    // if (rafScheduled) return;
    // rafScheduled = true;

    // requestAnimationFrame(() => {
    //     rafScheduled = false;
    //     flushPendingEvents(MOUSE_ACTION_MOVE);
    // });
}

/**
 * 发送事件 (增加了 buttonsOverride 参数)
 * @param {number} actionType 
 * @param {number} buttonsOverride - 可选，强制指定发送的按键掩码
 */
function flushPendingEvents(actionType, buttonsOverride) {
    // 只有当没有任何数据变化时才跳过
    if (actionType === MOUSE_ACTION_MOVE && 
        pendingMovement.x === 0 && 
        pendingMovement.y === 0 && 
        pendingMovement.wheelY === 0) {
        return;
    }

    const dx = Math.round(pendingMovement.x * MOUSE_SENSITIVITY);
    const dy = Math.round(pendingMovement.y * MOUSE_SENSITIVITY);
    const wheel = pendingMovement.wheelY;
    
    // 【关键】如果有传入 override (比如 MouseUp 时)，就用传入的，否则用全局状态
    // 注意检查 undefined，因为 0 也是有效值
    const buttons = (buttonsOverride !== undefined) ? buttonsOverride : mouseButtonsMask;

    // 发送
    sendControlPacket(actionType, dx, dy, buttons, wheel);
    // 重置累计
    pendingMovement.x = 0;
    pendingMovement.y = 0;
    pendingMovement.wheelY = 0;
}

/**
 * 实际的网络发送逻辑
 */
function sendControlPacket(action, dx, dy, buttons, wheel) {
    if (!window.ws || window.ws.readyState !== WebSocket.OPEN) return;

    // 调试日志：查看发送的具体动作和坐标
    
    virtualMouse.x += dx;
    virtualMouse.y += dy;
    // console.log(`Send: Act=${action}, x=${virtualMouse.x}, y=${virtualMouse.y}, Btn=${buttons}`);
    const packet = createMousePacket(action, virtualMouse.x, virtualMouse.y, buttons, 0, wheel);
    window.ws.send(packet);
}

/**
 * 创建鼠标事件数据包 (18字节)
 */
function createMousePacket(action, x, y, buttons, wheelDeltaX = 0, wheelDeltaY = 0) {
    const buffer = new ArrayBuffer(18);
    const view = new DataView(buffer);

    view.setUint8(0, TYPE_MOUSE); // Type
    view.setUint8(1, action);     // Action

    // 【重要修复】使用 setInt32 处理相对位移
    // 这样负数 (如 -5) 会被正确写入为补码
    // Go 端读取 Uint32 后强转 int32 即可还原为 -5
    view.setInt32(2, x, false); // BigEndian
    view.setInt32(6, y, false); // BigEndian

    view.setUint32(10, buttons, false);

    // 滚轮也是有方向的，使用 Int16
    view.setInt16(14, wheelDeltaX, false);
    view.setInt16(16, wheelDeltaY, false);

    return buffer;
}

// 启动
initRemoteControl();