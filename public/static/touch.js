const TYPE_TOUCH = 0x01; // touch event
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

const videoElement = document.getElementById('remoteVideo');

// 缓存视频元素的位置和尺寸，避免频繁调用 getBoundingClientRect
let cachedVideoRect = null;
let cachedScaleX = 1;
let cachedScaleY = 1;

// 更新缓存的视频尺寸和位置
function updateVideoCache() {
    if (videoElement.videoWidth && videoElement.videoHeight) {
        cachedVideoRect = videoElement.getBoundingClientRect();
        cachedScaleX = videoElement.videoWidth / videoElement.clientWidth;
        cachedScaleY = videoElement.videoHeight / videoElement.clientHeight;
        return true;
    }
    return false;
}

// 监听视频尺寸变化
videoElement.addEventListener('loadedmetadata', updateVideoCache);
window.addEventListener('resize', updateVideoCache);

// 使用 requestAnimationFrame 批量处理移动事件，减少延迟
let pendingMoveEvents = new Map(); // pointerId -> {x, y}
let rafScheduled = false;

function scheduleMoveSend() {
    if (!rafScheduled && pendingMoveEvents.size > 0) {
        rafScheduled = true;
        requestAnimationFrame(() => {
            rafScheduled = false;
            // 批量发送所有待发送的移动事件
            pendingMoveEvents.forEach((data, pointerId) => {
                sendTouchEvent(TOUCH_ACTION_MOVE, pointerId, data.x, data.y, data.pressure);
            });
            pendingMoveEvents.clear();
        });
    }
}

function getScreenCoordinates(clientX, clientY) {
    // 使用缓存的矩形和缩放比例
    if (!cachedVideoRect) {
        if (!updateVideoCache()) {
            return null;
        }
    }

    const offsetX = clientX - cachedVideoRect.left;
    const offsetY = clientY - cachedVideoRect.top;

    const x = Math.round(offsetX * cachedScaleX);
    const y = Math.round(offsetY * cachedScaleY);

    // Clamp coordinates to be within video bounds
    const clampedX = Math.max(0, Math.min(x, videoElement.videoWidth));
    const clampedY = Math.max(0, Math.min(y, videoElement.videoHeight));

    return { x: clampedX, y: clampedY };
}

// 获取触摸压力值
function getTouchPressure(touch) {
    // touch.force: 0.0 (无压力) 到 1.0 (最大压力)
    // 转换为 uint16: 0 到 65535
    // 某些设备不支持 force 属性，默认返回 1.0 (完全按下)
    const force = typeof touch.force !== 'undefined' ? touch.force : 1.0;
    
    // 映射到 0-65535 范围
    // 注意：很多设备即使支持 force，也只会返回 0 或 1，而不是连续值
    // iOS 设备(支持 3D Touch/Haptic Touch) 会返回 0.0 - 1.0 之间的浮点数
    const pressure = Math.round(force * 65535);
    
    return Math.max(0, Math.min(65535, pressure));
}

// ========== 鼠标事件处理 (单点) ==========
let activeMousePointer = null;

videoElement.addEventListener('mousedown', (event) => {
    if (event.button !== 0) return; // Only Left Click
    activeMousePointer = 0; // 使用 pointerId 0 表示鼠标
    const coords = getScreenCoordinates(event.clientX, event.clientY);
    if (coords) {
        sendTouchEvent(TOUCH_ACTION_DOWN, 0, coords.x, coords.y);
    }
});

videoElement.addEventListener('mouseup', (event) => {
    if (activeMousePointer !== null) {
        const coords = getScreenCoordinates(event.clientX, event.clientY);
        if (coords) {
            sendTouchEvent(TOUCH_ACTION_UP, 0, coords.x, coords.y);
        }
        activeMousePointer = null;
    }
});

videoElement.addEventListener('mousemove', (event) => {
    if (activeMousePointer !== null && event.buttons === 1) {
        const coords = getScreenCoordinates(event.clientX, event.clientY);
        if (coords) {
            pendingMoveEvents.set(0, { ...coords, pressure: 65535 });
            scheduleMoveSend();
        }
    }
});

// 处理鼠标移出视频区域后释放的情况
videoElement.addEventListener('mouseleave', (event) => {
    if (activeMousePointer !== null && event.buttons !== 1) {
        const coords = getScreenCoordinates(event.clientX, event.clientY);
        if (coords) {
            sendTouchEvent(TOUCH_ACTION_UP, 0, coords.x, coords.y);
        }
        activeMousePointer = null;
    }
});

// ========== 触摸事件处理 (多点触控) ==========
const activeTouches = new Map(); // touchIdentifier -> pointerId

videoElement.addEventListener('touchstart', (event) => {
    event.preventDefault();
    updateVideoCache(); // 触摸开始时更新缓存
    
    for (let i = 0; i < event.changedTouches.length; i++) {
        const touch = event.changedTouches[i];
        const pointerId = touch.identifier % 10; // 限制在 0-9 范围内
        activeTouches.set(touch.identifier, pointerId);
        
        const coords = getScreenCoordinates(touch.clientX, touch.clientY);
        if (coords) {
            const pressure = getTouchPressure(touch);
            sendTouchEvent(TOUCH_ACTION_DOWN, pointerId, coords.x, coords.y, pressure);
        }
    }
}, { passive: false });

videoElement.addEventListener('touchend', (event) => {
    event.preventDefault();
    
    for (let i = 0; i < event.changedTouches.length; i++) {
        const touch = event.changedTouches[i];
        const pointerId = activeTouches.get(touch.identifier);
        
        if (pointerId !== undefined) {
            const coords = getScreenCoordinates(touch.clientX, touch.clientY);
            if (coords) {
                sendTouchEvent(TOUCH_ACTION_UP, pointerId, coords.x, coords.y);
            }
            activeTouches.delete(touch.identifier);
        }
    }
}, { passive: false });

videoElement.addEventListener('touchmove', (event) => {
    event.preventDefault();
    
    for (let i = 0; i < event.changedTouches.length; i++) {
        const touch = event.changedTouches[i];
        const pointerId = activeTouches.get(touch.identifier);
        
        if (pointerId !== undefined) {
            const coords = getScreenCoordinates(touch.clientX, touch.clientY);
            if (coords) {
                const pressure = getTouchPressure(touch);
                pendingMoveEvents.set(pointerId, { ...coords, pressure });
            }
        }
    }
    scheduleMoveSend();
}, { passive: false });

videoElement.addEventListener('touchcancel', (event) => {
    event.preventDefault();
    
    for (let i = 0; i < event.changedTouches.length; i++) {
        const touch = event.changedTouches[i];
        const pointerId = activeTouches.get(touch.identifier);
        
        if (pointerId !== undefined) {
            const coords = getScreenCoordinates(touch.clientX, touch.clientY);
            if (coords) {
                sendTouchEvent(TOUCH_ACTION_UP, pointerId, coords.x, coords.y);
            }
            activeTouches.delete(touch.identifier);
        }
    }
}, { passive: false });

function sendTouchEvent(action, ptrId, x, y, pressure=65535, buttons=1) {
    if (!window.ws || window.ws.readyState !== WebSocket.OPEN) {
        console.warn("WebSocket is not open. Cannot send message.");
        return;
    }
    // console.log(`Sending touch event: action=${action}, ptrId=${ptrId}, x=${x}, y=${y}`);
    const p = createTouchPacket(action, ptrId, x, y, pressure, buttons);
    window.ws.send(p);
}


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