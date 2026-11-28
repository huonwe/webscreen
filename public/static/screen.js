let isUHIDMouseEnabled = false;
// 缓存视频元素的位置和尺寸，避免频繁调用 getBoundingClientRect
let cachedRect = {
    VideoRect: null,
    ContentRect: { left: 0, top: 0, width: 0, height: 0 },
    MediaWidth: 0,
    MediaHeight: 0
}

const remoteCanvas = document.getElementById('remoteCanvas');
const canvasContainer = document.getElementById('canvasContainer');
let mannualRotate = false;
remoteCanvas.style.touchAction = 'none';
remoteCanvas.style.overscrollBehavior = 'none';
let checkTimeout;

function triggerCheck() {
    clearTimeout(checkTimeout);
    checkTimeout = setTimeout(checkOrientation, 1000);
}

window.addEventListener('resize', triggerCheck);

function checkOrientation() {
    if (mannualRotate) return;
    const video = document.getElementById('remoteCanvas');
    if (!video.width || !video.height) return;

    const isPagePortrait = window.innerHeight > window.innerWidth;
    const isVideoLandscape = video.width > video.height;
    const isPageLandscape = window.innerWidth > window.innerHeight;
    const isVideoPortrait = video.height > video.width;

    if ((isPagePortrait && isVideoLandscape) ||
        (isPageLandscape && isVideoPortrait)) {
        const packet = createRotatePacket();
        sendButtonEvent(packet);
    }
}

export function SetMannualRotate(value) {
    mannualRotate = value;
}

export function MannualRotated() {
    return mannualRotate;
}
export function getRemoteCanvas() {
    return remoteCanvas;
}

export function getCanvasContainer() {
    return canvasContainer;
}

export function getCachedRect() {
    return cachedRect;
}

export function isUHIDMouseEnabled() {
    return isUHIDMouseEnabled;
}

export function setUHIDMouseEnabled(enabled) {
    isUHIDMouseEnabled = enabled;
}


// 1. VideoRect（DOM 容器矩形）
// 来源：remoteCanvas.getBoundingClientRect()
// 含义：<canvas> 元素本身在浏览器视口（Viewport）中的位置和大小（CSS 像素）。
// 包含属性：left、top、right、bottom、width、height、x、y。
// 作用：确定整个播放器框在屏幕上的绝对位置，以及框的总宽高（elWidth、elHeight）。
// 2. ContentRect（实际视频内容显示区）
// 含义：视频画面在 <canvas> 容器内部等比例缩放居中（Contain 模式）后，真正有画面的区域（不含黑边），单位为 CSS 像素。
// 包含属性：
// left / top：有效画面相对于 remoteCanvas 左上角的偏移量（即上下黑边或左右黑边的厚度）。
// width / height：有效画面实际渲染出来的宽度和高度。
// 计算逻辑：
// 当容器比视频更宽（elRatio > vidRatio）：画面高度撑满容器（top = 0），左右留黑边，水平居中（left = (elWidth - drawWidth) / 2）。
// 当容器比视频更高（elRatio <= vidRatio）：画面宽度撑满容器（left = 0），上下留黑边，垂直居中（top = (elHeight - drawHeight) / 2）。
// 3. MediaWidth 与 MediaHeight（原始媒体分辨率）
// 来源：remoteCanvas.width 和 remoteCanvas.height
// 含义：视频源本身的固有像素分辨率（Buffer 尺寸，例如 1920 × 1080），与 CSS 样式拉伸无关。
// 作用：作为坐标换算的目标基准。
// 更新缓存的视频尺寸和位置
export function updateVideoCache() {
    const vidWidth = remoteCanvas.width;
    const vidHeight = remoteCanvas.height;

    if (vidWidth && vidHeight) {
        cachedRect.VideoRect =
            remoteCanvas.getBoundingClientRect();

        const elWidth = cachedRect.VideoRect.width;
        const elHeight = cachedRect.VideoRect.height;

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

        cachedRect.ContentRect = {
            left: startX,
            top: startY,
            width: drawWidth,
            height: drawHeight
        };
        cachedRect.MediaWidth = vidWidth;
        cachedRect.MediaHeight = vidHeight;
        return true;
    }

    return false;
}

/**
 * 自动监听视频元素尺寸变化
 * 代替 window.resize，能捕捉 CSS 旋转、缩放等引起的尺寸改变
 */
let updateCacheTimer;
function initVideoObserver() {
    const remoteCanvas = getRemoteCanvas();
    if (!remoteCanvas) return;

    const observer = new ResizeObserver(entries => {
        // for (let entry of entries) {
        // 使用防抖，防止短时间内频繁触发
        if (updateCacheTimer) clearTimeout(updateCacheTimer);

        updateCacheTimer = setTimeout(() => {
            console.log("Detected video size change, updating cache...");
            updateVideoCache();
        }, 100);
        // }
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
export function RequestPointerLock() {
    // if (!uhidMouseEnabled) return;

    const remoteCanvas = getRemoteCanvas();
    const requestMethod = remoteCanvas.requestPointerLock ||
        remoteCanvas.mozRequestPointerLock ||
        remoteCanvas.webkitRequestPointerLock;

    if (requestMethod) {
        requestMethod.call(remoteCanvas);
    }
}

export function exitPointerLock() {
    const exitMethod = document.exitPointerLock ||
        document.mozExitPointerLock ||
        document.webkitExitPointerLock;

    if (exitMethod) {
        exitMethod.call(document);
    }
}

export function PointerLocked() {
    return document.pointerLockElement === getRemoteCanvas() ||
        document.mozPointerLockElement === getRemoteCanvas() ||
        document.webkitPointerLockElement === getRemoteCanvas();
}

// 这里WebRTC会自动通过RTCP请求关键帧，但我们也可以手动请求
const TYPE_RKF = 0x63; // request key frame

function createRequestKeyFramePacket() {
    const buffer = new ArrayBuffer(2);
    const view = new DataView(buffer);
    view.setUint8(0, TYPE_RKF);
    view.setUint8(1, 0);

    return buffer;
}

/**
 * 显示顶部 Toast 提示消息
 * @param {string} message 消息文本
 * @param {number} duration 持续时间(ms)
 */
export function ShowToast(message, duration = 1000) {
    console.log(message)
    let toast = document.getElementById('scrmux-toast');

    if (!toast) {
        toast = document.createElement('div');
        toast.id = 'scrmux-toast';

        // 注入样式
        Object.assign(toast.style, {
            position: 'fixed',
            top: '-60px', // 初始位置在屏幕外
            left: '50%',
            transform: 'translateX(-50%)',
            backgroundColor: 'rgba(33, 33, 33, 0.95)',
            color: '#ffffff',
            padding: '12px 24px',
            borderRadius: '8px',
            boxShadow: '0 4px 12px rgba(0,0,0,0.2)',
            zIndex: '10000',
            fontSize: '14px',
            fontWeight: '500',
            pointerEvents: 'none',
            transition: 'top 0.4s cubic-bezier(0.22, 1, 0.36, 1), opacity 0.4s ease',
            opacity: '0',
            whiteSpace: 'nowrap',
            maxWidth: '90%',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            display: 'flex',
            alignItems: 'center',
            gap: '8px'
        });

        document.body.appendChild(toast);
    }

    toast.textContent = message;

    // 显示动画
    requestAnimationFrame(() => {
        toast.style.top = '24px'; // 下滑出现
        toast.style.opacity = '1';
    });

    // 清除旧定时器
    if (toast.timeoutId) {
        clearTimeout(toast.timeoutId);
    }

    // 自动隐藏
    toast.timeoutId = setTimeout(() => {
        toast.style.top = '-60px'; // 上滑消失
        toast.style.opacity = '0';
    }, duration);
}

function getFullscreenElement() {
    return document.fullscreenElement ||
        document.webkitFullscreenElement ||
        document.mozFullScreenElement ||
        document.msFullscreenElement ||
        null;
}

export function RequestFullScreen() {
    console.log("Attempting to toggle fullscreen");
    const remoteCanvas = getRemoteCanvas();
    if (!remoteCanvas) return;
    const target = canvasContainer || remoteCanvas;

    // if (!getFullscreenElement()) {
        const requestMethod = target.requestFullscreen ||
            target.webkitRequestFullscreen ||
            target.mozRequestFullScreen ||
            target.msRequestFullscreen;

        if (requestMethod) {
            const ret = requestMethod.call(target);
            // 兼容旧版 WebKit 返回 undefined 而非 Promise 的情况
            if (ret && typeof ret.catch === 'function') {
                ret.catch(err => {
                    console.error(`Error attempting to enable full-screen mode: ${err.message} (${err.name})`);
                });
            }
        } else if (remoteCanvas && typeof remoteCanvas.webkitEnterFullscreen === 'function') {
            // iPhone Safari 兜底（注意：原生播放器全屏会遮挡自定义 HTML 覆盖层）
            remoteCanvas.webkitEnterFullscreen();
        }
    // } 
    // else {
    //     const exitMethod = document.exitFullscreen ||
    //         document.webkitExitFullscreen ||
    //         document.mozCancelFullScreen ||
    //         document.msExitFullscreen;

    //     if (exitMethod) {
    //         const ret = exitMethod.call(document);
    //         if (ret && typeof ret.catch === 'function') {
    //             ret.catch(() => { });
    //         }
    //     }
    // }
}

async function syncKeyboardLock(isFullscreen) {
    if (!navigator.keyboard || typeof navigator.keyboard.lock !== 'function') return;
    try {
        if (isFullscreen) {
            await navigator.keyboard.lock(['Escape', 'F11']);
        } else {
            navigator.keyboard.unlock();
        }
    } catch (err) {
        // 非 HTTPS 或部分浏览器策略拒绝时静默忽略
    }
}

function handleFullscreenChange() {
    const remoteCanvas = getRemoteCanvas();
    if (!remoteCanvas) return;
    const isFullscreen = !!getFullscreenElement();

    if (isFullscreen) {
        remoteCanvas.style.width = '100%';
        remoteCanvas.style.height = '100%';
        remoteCanvas.style.objectFit = 'contain';
    } else {
        remoteCanvas.style.width = '';
        remoteCanvas.style.height = '';
        remoteCanvas.style.objectFit = '';
    }

    syncKeyboardLock(isFullscreen);

    // 等浏览器完成全屏布局重排后再刷新视频区域与触控坐标缓存
    requestAnimationFrame(() => {
        updateVideoCache();
        window.dispatchEvent(new Event('resize'));
    });
}

export function addControlButton(id, title, path, className = '') {
    const existing = document.getElementById(id);
    if (existing) return existing;

    const controls = document.querySelector('.controls-container');
    if (!controls) throw new Error('Controls container is not available');

    const button = document.createElement('button');
    button.id = id;
    button.type = 'button';
    button.className = `control-btn ${className}`.trim();
    button.title = title;
    button.setAttribute('aria-label', title);
    button.innerHTML = `
        <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
            <path d="${path}"></path>
        </svg>`;
    controls.appendChild(button);
    return button;
}

export function addControlSeparator() {
    const controls = document.querySelector('.controls-container');
    if (!controls) throw new Error('Controls container is not available');

    const separator = document.createElement('div');
    separator.className = 'separator';
    controls.appendChild(separator);
    return separator;
}

addControlButton('fullscreen-btn', '全屏', 'M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z');

document.addEventListener('fullscreenchange', handleFullscreenChange);
document.addEventListener('webkitfullscreenchange', handleFullscreenChange);
document.addEventListener('mozfullscreenchange', handleFullscreenChange);
document.addEventListener('MSFullscreenChange', handleFullscreenChange);



let wakeLock = null;

// 申请常亮
async function requestWakeLock() {
    try {
        wakeLock = await navigator.wakeLock.request('screen');
        console.log('Screen Wake Lock is active');

        // 监听释放事件
        wakeLock.addEventListener('release', () => {
            console.log('Screen Wake Lock is released');
        });
    } catch (err) {
        console.error(`${err.name}, ${err.message}`);
    }
}

// 释放常亮
function releaseWakeLock() {
    if (wakeLock !== null) {
        wakeLock.release();
        wakeLock = null;
    }
}

// 当页面不再需要常亮或页面切换到后台时释放
document.addEventListener('visibilitychange', () => {
    if (wakeLock !== null && document.visibilityState === 'visible') {
        requestWakeLock();
    }
    else if (wakeLock !== null && document.visibilityState === 'hidden') {
        releaseWakeLock();
    }
});

