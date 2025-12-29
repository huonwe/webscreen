const remoteVideo = document.getElementById('remoteVideo');

// 缓存视频元素的位置和尺寸，避免频繁调用 getBoundingClientRect
window.cachedRect = {
    VideoRect: null,
    ContentRect: { left: 0, top: 0, width: 0, height: 0 }
}
// 更新缓存的视频尺寸和位置
function updateVideoCache() {
    if (remoteVideo.videoWidth && remoteVideo.videoHeight) {
        window.cachedRect.VideoRect = remoteVideo.getBoundingClientRect();
        
        const elWidth = window.cachedRect.VideoRect.width;
        const elHeight = window.cachedRect.VideoRect.height;
        const vidWidth = remoteVideo.videoWidth;
        const vidHeight = remoteVideo.videoHeight;
        
        if (elWidth === 0 || elHeight === 0) return false;

        const vidRatio = vidWidth / vidHeight;
        const elRatio = elWidth / elHeight;

        let drawWidth, drawHeight, startX, startY;

        if (elRatio > vidRatio) {
            // 元素比视频宽 (Pillarbox: 左右黑边)
            drawHeight = elHeight;
            drawWidth = drawHeight * vidRatio;
            startY = 0;
            startX = (elWidth - drawWidth) / 2;
        } else {
            // 元素比视频高 (Letterbox: 上下黑边)
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
        return true;
    }
    return false;
}

// 监听视频尺寸变化
remoteVideo.addEventListener('loadedmetadata', updateVideoCache);
window.addEventListener('resize', updateVideoCache);

// 指针锁定 API (Pointer Lock) - 用于更好的鼠标控制
function requestPointerLock() {
    // if (!uhidMouseEnabled) return;

    const requestMethod = remoteVideo.requestPointerLock ||
        remoteVideo.mozRequestPointerLock ||
        remoteVideo.webkitRequestPointerLock;

    if (requestMethod) {
        requestMethod.call(remoteVideo);
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