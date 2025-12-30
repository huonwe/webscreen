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

/**
 * 自动监听视频元素尺寸变化
 * 代替 window.resize，能捕捉 CSS 旋转、缩放等引起的尺寸改变
 */
function initVideoObserver() {
    const remoteVideo = document.getElementById('remoteVideo');
    if (!remoteVideo) return;

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
    observer.observe(remoteVideo);
}
initVideoObserver();

// 监听视频尺寸变化
// remoteVideo.addEventListener('loadedmetadata', updateVideoCache);
// window.addEventListener('resize', ()=> {
//     setTimeout(() => {
//         updateVideoCache();
//     }, 500);
// });

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