const canvasElementFs = document.getElementById('remoteCanvas');
const canvasContainer = document.querySelector('.video-container');

if (canvasElementFs) {
    // 常驻禁用默认触摸手势，避免非全屏模式下触控板/直触触发页面滚动或缩放
    canvasElementFs.style.touchAction = 'none';
    canvasElementFs.style.overscrollBehavior = 'none';

    // 阻止点击/双击视频时的默认行为（如暂停/播放）
    // canvasElementFs.addEventListener('click', (event) => {
    //     event.preventDefault();
    //     event.stopPropagation();
    // });

    // canvasElementFs.addEventListener('dblclick', (event) => {
    //     event.preventDefault();
    //     event.stopPropagation();
    // });
}

function getFullscreenElement() {
    return document.fullscreenElement ||
           document.webkitFullscreenElement ||
           document.mozFullScreenElement ||
           document.msFullscreenElement ||
           null;
}

function fullscreenPlayer() {
    if (!canvasElementFs && !canvasContainer) return;
    const target = canvasContainer || canvasElementFs;

    if (!getFullscreenElement()) {
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
        } else if (canvasElementFs && typeof canvasElementFs.webkitEnterFullscreen === 'function') {
            // iPhone Safari 兜底（注意：原生播放器全屏会遮挡自定义 HTML 覆盖层）
            canvasElementFs.webkitEnterFullscreen();
        }
    } else {
        const exitMethod = document.exitFullscreen ||
                           document.webkitExitFullscreen ||
                           document.mozCancelFullScreen ||
                           document.msExitFullscreen;

        if (exitMethod) {
            const ret = exitMethod.call(document);
            if (ret && typeof ret.catch === 'function') {
                ret.catch(() => {});
            }
        }
    }
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
    if (!canvasElementFs) return;
    const isFullscreen = !!getFullscreenElement();

    if (isFullscreen) {
        canvasElementFs.style.width = '100%';
        canvasElementFs.style.height = '100%';
        canvasElementFs.style.objectFit = 'contain';
    } else {
        canvasElementFs.style.width = '';
        canvasElementFs.style.height = '';
        canvasElementFs.style.objectFit = '';
    }

    syncKeyboardLock(isFullscreen);

    // 等浏览器完成全屏布局重排后再刷新视频区域与触控坐标缓存
    requestAnimationFrame(() => {
        if (typeof window.updateVideoCache === 'function') {
            window.updateVideoCache();
        }
        window.dispatchEvent(new Event('resize'));
    });
}

document.addEventListener('fullscreenchange', handleFullscreenChange);
document.addEventListener('webkitfullscreenchange', handleFullscreenChange);
document.addEventListener('mozfullscreenchange', handleFullscreenChange);
document.addEventListener('MSFullscreenChange', handleFullscreenChange);