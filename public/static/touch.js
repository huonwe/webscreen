const videoElement = document.getElementById('remoteVideo');
const TOUCH_SAMPLING_RATE = 16; // 采样间隔(ms), 16ms ≈ 60fps

function throttle(func, limit) {
    let inThrottle;
    return function() {
        const args = arguments;
        const context = this;
        if (!inThrottle) {
            func.apply(context, args);
            inThrottle = true;
            setTimeout(() => inThrottle = false, limit);
        }
    }
}

function getScreenCoordinates(event) {
    // Ensure video metadata is loaded
    if (!videoElement.videoWidth || !videoElement.videoHeight) {
        return null;
    }

    // Calculate scale factors
    // Since we use max-width/max-height and display:block, the video element
    // size exactly matches the rendered video size.
    const scaleX = videoElement.videoWidth / videoElement.clientWidth;
    const scaleY = videoElement.videoHeight / videoElement.clientHeight;

    // Calculate coordinates
    let clientX, clientY;

    if (event.touches && event.touches.length > 0) {
        clientX = event.touches[0].clientX;
        clientY = event.touches[0].clientY;
    } else if (event.changedTouches && event.changedTouches.length > 0) {
        clientX = event.changedTouches[0].clientX;
        clientY = event.changedTouches[0].clientY;
    } else {
        clientX = event.clientX;
        clientY = event.clientY;
    }

    const rect = videoElement.getBoundingClientRect();
    const offsetX = clientX - rect.left;
    const offsetY = clientY - rect.top;

    // 检查点击是否在视频区域内
    // 如果是在 document 上触发的 mouseup，且鼠标已经移出了 video 区域，
    // offsetX/offsetY 可能会超出范围，或者变成负数。
    // 但更重要的是，如果鼠标完全移出了浏览器窗口，clientX/clientY 可能是有效的，
    // 但相对于 videoElement 的计算可能需要更鲁棒的处理。

    const x = Math.round(offsetX * scaleX);
    const y = Math.round(offsetY * scaleY);

    // Clamp coordinates to be within video bounds (just in case)
    const clampedX = Math.max(0, Math.min(x, videoElement.videoWidth));
    const clampedY = Math.max(0, Math.min(y, videoElement.videoHeight));

    return { x: clampedX, y: clampedY };
}

let isMouseDown = false;
let lastX = 0;
let lastY = 0;

videoElement.addEventListener('mousedown', (event) => {
    if (event.button !== 0) return; // Only Left Click
    isMouseDown = true;
    const coords = getScreenCoordinates(event);
    if (coords) {
        lastX = coords.x;
        lastY = coords.y;
        // console.log(`MouseDown at: ${coords.x}, ${coords.y}`);
        sendTouchEvent(TOUCH_ACTION_DOWN, 0, coords.x, coords.y);
    }
});

document.addEventListener('mouseup', (event) => {
    if (isMouseDown) {
        isMouseDown = false;
        let coords = getScreenCoordinates(event);
        
        // 如果在 document 上释放鼠标，且 getScreenCoordinates 返回 null (例如 video 尺寸未加载)
        // 或者计算出的坐标异常，我们可以使用最后一次已知的有效坐标 (lastX, lastY)
        // 这对于“拖拽出屏幕外释放”的情况特别有用，因为我们希望在最后的位置抬起手指。
        if (!coords) {
             coords = { x: lastX, y: lastY };
        }

        if (coords) {
            // console.log(`MouseUp at: ${coords.x}, ${coords.y}`);
            sendTouchEvent(TOUCH_ACTION_UP, 0, coords.x, coords.y);
        }
    }
});

const handleMouseMove = throttle((event) => {
    if (event.buttons !== 1) return; // Only when left button is pressed
    const coords = getScreenCoordinates(event);
    if (coords) {
        lastX = coords.x;
        lastY = coords.y;
        sendTouchEvent(TOUCH_ACTION_MOVE, 0, coords.x, coords.y);
    }
}, TOUCH_SAMPLING_RATE);

videoElement.addEventListener('mousemove', handleMouseMove);

videoElement.addEventListener('touchstart', (event) => {
    event.preventDefault(); // Prevent scrolling/mouse emulation
    const coords = getScreenCoordinates(event);
    if (coords) {
        // console.log(`TouchStart at: ${coords.x}, ${coords.y}`);
        sendTouchEvent(TOUCH_ACTION_DOWN, 0, coords.x, coords.y);
    }
}, { passive: false });

videoElement.addEventListener('touchend', (event) => {
    event.preventDefault();
    const coords = getScreenCoordinates(event);
    if (coords) {
        // console.log(`TouchEnd at: ${coords.x}, ${coords.y}`);
        sendTouchEvent(TOUCH_ACTION_UP, 0, coords.x, coords.y);
    }
}, { passive: false });

const handleTouchMove = throttle((event) => {
    event.preventDefault();
    const coords = getScreenCoordinates(event);
    if (coords) {
        sendTouchEvent(TOUCH_ACTION_MOVE, 0, coords.x, coords.y);
    }
}, TOUCH_SAMPLING_RATE);

videoElement.addEventListener('touchmove', handleTouchMove, { passive: false });

videoElement.addEventListener('touchcancel', (event) => {
    event.preventDefault();
    const coords = getScreenCoordinates(event);
    if (coords) {
        // console.log(`TouchCancel at: ${coords.x}, ${coords.y}`);
        sendTouchEvent(TOUCH_ACTION_UP, 0, coords.x, coords.y);
    }
}, { passive: false });

function sendTouchEvent(action, ptrId, x, y) {
    if (!window.ws || window.ws.readyState !== WebSocket.OPEN) {
        console.warn("WebSocket is not open. Cannot send message.");
        return;
    }
    // console.log(`Sending touch event: action=${action}, ptrId=${ptrId}, x=${x}, y=${y}`);
    p = createTouchPacket(action, ptrId, x, y);
    window.ws.send(p);
}