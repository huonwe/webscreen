const remoteCanvas = document.getElementById('remoteCanvas');

window.isUHIDMouseEnabled = false;
// 缓存视频元素的位置和尺寸，避免频繁调用 getBoundingClientRect
window.cachedRect = {
    VideoRect: null,
    ContentRect: { left: 0, top: 0, width: 0, height: 0 },
    MediaWidth: 0,
    MediaHeight: 0
}
function getMediaDimensions(element) {
    if (element instanceof HTMLCanvasElement) {
        return {
            width: element.width,
            height: element.height
        };
    }

    if (element instanceof HTMLVideoElement) {
        return {
            width: element.videoWidth,
            height: element.videoHeight
        };
    }

    return { width: 0, height: 0 };
}
// 更新缓存的视频尺寸和位置
function updateVideoCache() {
    const dimensions = getMediaDimensions(remoteCanvas);
    const vidWidth = dimensions.width;
    const vidHeight = dimensions.height;

    if (vidWidth && vidHeight) {
        window.cachedRect.VideoRect =
            remoteCanvas.getBoundingClientRect();

        const elWidth = window.cachedRect.VideoRect.width;
        const elHeight = window.cachedRect.VideoRect.height;

        if (elWidth === 0 || elHeight === 0) return false;

        const vidRatio = vidWidth / vidHeight;
        const elRatio = elWidth / elHeight;

        let drawWidth, drawHeight, startX, startY;

        if (elRatio > vidRatio) {
            drawHeight = elHeight;
            drawWidth = drawHeight * vidRatio;
            startY = 0;
            startX = (elWidth - drawWidth) / 2;
        } else {
            drawWidth = elWidth;
            drawHeight = drawWidth / vidRatio;
            startX = 0;
            startY = (elHeight - drawHeight) / 2;
        }

        window.cachedRect.ContentRect = {
            left: startX,
            top: startY,
            width: drawWidth,
            height: drawHeight
        };
        window.cachedRect.MediaWidth = vidWidth;
        window.cachedRect.MediaHeight = vidHeight;
        return true;
    }

    return false;
}

/**
 * 自动监听视频元素尺寸变化
 * 代替 window.resize，能捕捉 CSS 旋转、缩放等引起的尺寸改变
 */
function initVideoObserver() {
    const remoteCanvas = document.getElementById('remoteCanvas');
    if (!remoteCanvas) return;

    const observer = new ResizeObserver(entries => {
        for (let entry of entries) {
            // 使用防抖，防止短时间内频繁触发
            if (window.updateCacheTimer) clearTimeout(window.updateCacheTimer);
            
            window.updateCacheTimer = setTimeout(() => {
                console.log("Detected video size change, updating cache...");
                if (typeof updateVideoCache === 'function') {
                    updateVideoCache();
                }
            }, 100);
        }
    });

    // 开始观察视频元素
    observer.observe(remoteCanvas);
}
initVideoObserver();

// 监听视频尺寸变化
// remoteCanvas.addEventListener('loadedmetadata', updateVideoCache);
// window.addEventListener('resize', ()=> {
//     setTimeout(() => {
//         updateVideoCache();
//     }, 500);
// });

// 指针锁定 API (Pointer Lock) - 用于更好的鼠标控制
function requestPointerLock() {
    // if (!uhidMouseEnabled) return;

    const requestMethod = remoteCanvas.requestPointerLock ||
        remoteCanvas.mozRequestPointerLock ||
        remoteCanvas.webkitRequestPointerLock;

    if (requestMethod) {
        requestMethod.call(remoteCanvas);
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

// 这里WebRTC会自动通过RTCP请求关键帧，但我们也可以手动请求
const TYPE_RKF   = 0x63; // request key frame

function createRequestKeyFramePacket() {
    const buffer = new ArrayBuffer(2);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_RKF);
    view.setUint8(1, 0);

    return buffer;
}

