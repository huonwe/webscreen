import { addControlButton } from './control_ui.js';

const CAPTURE_INTERVAL_MS = 350; // 采样周期（Worker 内部会自动跳过未变化帧）
const WORKER_SCRIPT_URL = new URL('../paddleocr/worker.js', import.meta.url);

// 状态: 'idle' | 'selecting' | 'running'
let translateState = 'idle';
let ocrWorker = null;
let workerBusy = false;
let captureTimer = null;

// 用户框选的归一化区域 [0, 1]（相对于实际视频画面 ContentRect）
let roiNorm = null; // { nx, ny, nw, nh }

// DOM 引用
let selectorOverlay = null;
let subtitleOverlay = null;
let roiGuideBox = null;
let subtitleTextBox = null;

function getTargetCanvas() {
    if (typeof window !== 'undefined' && window.remoteCanvas instanceof HTMLCanvasElement) {
        return window.remoteCanvas;
    }
    if (typeof remoteCanvas !== 'undefined' && remoteCanvas instanceof HTMLCanvasElement) {
        return remoteCanvas;
    }
    return document.getElementById('remoteCanvas');
}

function getVideoContainer(canvas) {
    return (canvas && canvas.closest('.video-container')) ||
           document.querySelector('.video-container') ||
           document.body;
}

/**
 * 计算 <canvas> 在 object-fit: contain 下的实际有效画面区域（扣除黑边）
 */
function getRenderedContentRect(canvas) {
    const rect = canvas.getBoundingClientRect();
    const mediaW = canvas.width || 1920;
    const mediaH = canvas.height || 1080;

    if (rect.width <= 0 || rect.height <= 0) {
        return { left: 0, top: 0, width: 0, height: 0, rect };
    }

    const mediaRatio = mediaW / mediaH;
    const boxRatio = rect.width / rect.height;

    let contentW = rect.width;
    let contentH = rect.height;
    let offsetLeft = 0;
    let offsetTop = 0;

    if (boxRatio > mediaRatio) {
        // 左右有黑边
        contentW = rect.height * mediaRatio;
        offsetLeft = (rect.width - contentW) / 2;
    } else if (boxRatio < mediaRatio) {
        // 上下有黑边
        contentH = rect.width / mediaRatio;
        offsetTop = (rect.height - contentH) / 2;
    }

    return {
        left: offsetLeft,
        top: offsetTop,
        width: contentW,
        height: contentH,
        rect,
    };
}

function notifyToast(msg, duration = 1800) {
    if (typeof showToast === 'function') {
        showToast(msg, duration);
    }
}

function setButtonActive(active) {
    const btn = document.getElementById('gamestreamTranslateToggleBtn');
    if (btn) {
        btn.classList.toggle('active', active);
    }
}

// =========================================================================
// 1. Worker 生命周期管理
// =========================================================================

function ensureWorker() {
    if (ocrWorker) return ocrWorker;

    ocrWorker = new Worker(WORKER_SCRIPT_URL, { type: 'module' });
    ocrWorker.onmessage = (e) => {
        const msg = e.data || {};
        if (msg.type === 'RESULT') {
            workerBusy = false;
            renderTranslationResult(msg.translatedText, msg.rawText);
        } else if (msg.type === 'SKIPPED') {
            workerBusy = false;
        } else if (msg.type === 'ERROR') {
            workerBusy = false;
            console.warn('[OCR Worker]', msg.error);
            if (subtitleTextBox && !subtitleTextBox.dataset.hasText) {
                subtitleTextBox.textContent = `[OCR 初始化提示] ${msg.error}`;
                subtitleTextBox.style.display = 'block';
            }
        }
    };

    ocrWorker.onerror = (err) => {
        workerBusy = false;
        console.error('[OCR Worker Fatal]', err);
    };

    return ocrWorker;
}

// =========================================================================
// 2. 矩形区域框选交互（支持鼠标与触控）
// =========================================================================

function startRegionSelection() {
    const canvas = getTargetCanvas();
    if (!canvas) {
        notifyToast('未找到视频画布 (#remoteCanvas)');
        return;
    }

    // 若当前处于鼠标锁定状态，先解除锁定以便用户看到光标进行框选
    if (document.pointerLockElement) {
        document.exitPointerLock();
    }

    cleanupOverlays();
    translateState = 'selecting';
    setButtonActive(true);

    const container = getVideoContainer(canvas);
    if (getComputedStyle(container).position === 'static') {
        container.style.position = 'relative';
    }

    selectorOverlay = document.createElement('div');
    selectorOverlay.id = 'gs-ocr-selector';
    Object.assign(selectorOverlay.style, {
        position: 'absolute',
        inset: '0',
        zIndex: '40',
        cursor: 'crosshair',
        background: 'rgba(0, 0, 0, 0.35)',
        userSelect: 'none',
        touchAction: 'none',
    });

    const hint = document.createElement('div');
    hint.textContent = '请拖拽框选 Galgame 对话框区域（按 Esc 或右键取消）';
    Object.assign(hint.style, {
        position: 'absolute',
        top: '16px',
        left: '50%',
        transform: 'translateX(-50%)',
        padding: '6px 14px',
        borderRadius: '16px',
        background: 'rgba(20, 22, 26, 0.88)',
        border: '1px solid rgba(255, 255, 255, 0.25)',
        color: '#fff',
        fontSize: '13px',
        pointerEvents: 'none',
        whiteSpace: 'nowrap',
    });

    const selectionBox = document.createElement('div');
    Object.assign(selectionBox.style, {
        position: 'absolute',
        border: '2px solid #a8c7fa',
        background: 'rgba(168, 199, 250, 0.16)',
        boxShadow: '0 0 0 9999px rgba(0, 0, 0, 0.35)',
        display: 'none',
        pointerEvents: 'none',
    });

    selectorOverlay.appendChild(hint);
    selectorOverlay.appendChild(selectionBox);
    container.appendChild(selectorOverlay);

    let dragging = false;
    let startX = 0;
    let startY = 0;
    let curX = 0;
    let curY = 0;

    const getLocalPoint = (clientX, clientY) => {
        const box = selectorOverlay.getBoundingClientRect();
        return {
            x: Math.max(0, Math.min(box.width, clientX - box.left)),
            y: Math.max(0, Math.min(box.height, clientY - box.top)),
        };
    };

    const updateBox = () => {
        const left = Math.min(startX, curX);
        const top = Math.min(startY, curY);
        const width = Math.abs(curX - startX);
        const height = Math.abs(curY - startY);
        Object.assign(selectionBox.style, {
            display: 'block',
            left: `${left}px`,
            top: `${top}px`,
            width: `${width}px`,
            height: `${height}px`,
        });
    };

    const finishDrag = () => {
        if (!dragging) return;
        dragging = false;

        const left = Math.min(startX, curX);
        const top = Math.min(startY, curY);
        const width = Math.abs(curX - startX);
        const height = Math.abs(curY - startY);

        if (width < 20 || height < 16) {
            notifyToast('选区过小，请重新拖拽框选对话框');
            selectionBox.style.display = 'none';
            return;
        }

        // 将选区换算为相对于有效画面 ContentRect 的归一化坐标 [0, 1]
        const content = getRenderedContentRect(canvas);
        const containerRect = selectorOverlay.getBoundingClientRect();
        const absLeft = containerRect.left + left;
        const absTop = containerRect.top + top;

        const contentAbsLeft = content.rect.left + content.left;
        const contentAbsTop = content.rect.top + content.top;

        const x1 = Math.max(0, Math.min(content.width, absLeft - contentAbsLeft));
        const y1 = Math.max(0, Math.min(content.height, absTop - contentAbsTop));
        const x2 = Math.max(0, Math.min(content.width, absLeft + width - contentAbsLeft));
        const y2 = Math.max(0, Math.min(content.height, absTop + height - contentAbsTop));

        if (x2 - x1 < 16 || y2 - y1 < 12) {
            notifyToast('请框选画面有效区域内的对话框');
            selectionBox.style.display = 'none';
            return;
        }

        roiNorm = {
            nx: x1 / content.width,
            ny: y1 / content.height,
            nw: (x2 - x1) / content.width,
            nh: (y2 - y1) / content.height,
        };

        startAutoTranslate();
    };

    // 鼠标拖拽
    selectorOverlay.addEventListener('mousedown', (e) => {
        e.preventDefault();
        e.stopPropagation();
        if (e.button === 2) {
            stopTranslation(true);
            return;
        }
        if (e.button !== 0) return;
        const pt = getLocalPoint(e.clientX, e.clientY);
        dragging = true;
        startX = curX = pt.x;
        startY = curY = pt.y;
        updateBox();
    });

    selectorOverlay.addEventListener('mousemove', (e) => {
        if (!dragging) return;
        e.preventDefault();
        const pt = getLocalPoint(e.clientX, e.clientY);
        curX = pt.x;
        curY = pt.y;
        updateBox();
    });

    selectorOverlay.addEventListener('mouseup', (e) => {
        if (!dragging) return;
        e.preventDefault();
        e.stopPropagation();
        const pt = getLocalPoint(e.clientX, e.clientY);
        curX = pt.x;
        curY = pt.y;
        finishDrag();
    });

    selectorOverlay.addEventListener('contextmenu', (e) => {
        e.preventDefault();
        e.stopPropagation();
        stopTranslation(true);
    });

    // 触控拖拽
    selectorOverlay.addEventListener('touchstart', (e) => {
        e.preventDefault();
        e.stopPropagation();
        if (e.touches.length === 0) return;
        const pt = getLocalPoint(e.touches[0].clientX, e.touches[0].clientY);
        dragging = true;
        startX = curX = pt.x;
        startY = curY = pt.y;
        updateBox();
    }, { passive: false });

    selectorOverlay.addEventListener('touchmove', (e) => {
        if (!dragging || e.touches.length === 0) return;
        e.preventDefault();
        const pt = getLocalPoint(e.touches[0].clientX, e.touches[0].clientY);
        curX = pt.x;
        curY = pt.y;
        updateBox();
    }, { passive: false });

    selectorOverlay.addEventListener('touchend', (e) => {
        if (!dragging) return;
        e.preventDefault();
        e.stopPropagation();
        finishDrag();
    }, { passive: false });
}

// 框选阶段按 Esc 取消框选，不向远端游戏转发 Escape
document.addEventListener('keydown', (e) => {
    if (translateState === 'selecting' && e.code === 'Escape') {
        e.preventDefault();
        e.stopImmediatePropagation();
        stopTranslation(true);
    }
}, true);

// =========================================================================
// 3. 自动截取 Canvas ROI 与 Galgame 字幕浮层渲染
// =========================================================================

function createSubtitleOverlay(container) {
    subtitleOverlay = document.createElement('div');
    subtitleOverlay.id = 'gs-ocr-overlay';
    Object.assign(subtitleOverlay.style, {
        position: 'absolute',
        inset: '0',
        zIndex: '22',
        pointerEvents: 'none', // 完全穿透鼠标点击，不影响点击对话框推进剧情
        overflow: 'hidden',
    });

    // 虚线指示框，标明当前监控区域
    roiGuideBox = document.createElement('div');
    Object.assign(roiGuideBox.style, {
        position: 'absolute',
        border: '1px dashed rgba(168, 199, 250, 0.45)',
        borderRadius: '6px',
        boxSizing: 'border-box',
    });

    // 译文显示栏（贴合对话框上方或下方，保留原文可见）
    subtitleTextBox = document.createElement('div');
    Object.assign(subtitleTextBox.style, {
        position: 'absolute',
        padding: '10px 16px',
        borderRadius: '10px',
        background: 'rgba(12, 14, 18, 0.84)',
        border: '1px solid rgba(255, 255, 255, 0.16)',
        color: '#f8fafc',
        fontSize: 'clamp(14px, 1.5vw, 22px)',
        lineHeight: '1.55',
        letterSpacing: '0.02em',
        textShadow: '0 1px 2px rgba(0, 0, 0, 0.85)',
        backdropFilter: 'blur(6px)',
        boxSizing: 'border-box',
        whiteSpace: 'pre-wrap',
        wordBreak: 'break-word',
        display: 'none',
    });

    subtitleOverlay.appendChild(roiGuideBox);
    subtitleOverlay.appendChild(subtitleTextBox);
    container.appendChild(subtitleOverlay);
    syncOverlayPosition();
}

function syncOverlayPosition() {
    if (!subtitleOverlay || !roiNorm) return;
    const canvas = getTargetCanvas();
    if (!canvas) return;

    const content = getRenderedContentRect(canvas);
    const containerRect = subtitleOverlay.getBoundingClientRect();

    const baseLeft = (content.rect.left - containerRect.left) + content.left;
    const baseTop = (content.rect.top - containerRect.top) + content.top;

    const boxLeft = baseLeft + roiNorm.nx * content.width;
    const boxTop = baseTop + roiNorm.ny * content.height;
    const boxWidth = roiNorm.nw * content.width;
    const boxHeight = roiNorm.nh * content.height;

    Object.assign(roiGuideBox.style, {
        left: `${boxLeft}px`,
        top: `${boxTop}px`,
        width: `${boxWidth}px`,
        height: `${boxHeight}px`,
    });

    // 若选区位于画面下半部分（常规 Galgame 对话框），字幕显示在选区正上方；否则显示在选区正下方
    const placeAbove = (roiNorm.ny + roiNorm.nh / 2) > 0.45;
    Object.assign(subtitleTextBox.style, {
        left: `${boxLeft}px`,
        width: `${boxWidth}px`,
        top: placeAbove ? 'auto' : `${boxTop + boxHeight + 8}px`,
        bottom: placeAbove ? `${containerRect.height - boxTop + 8}px` : 'auto',
    });
}

function renderTranslationResult(translatedText, rawText) {
    if (!subtitleTextBox || translateState !== 'running') return;
    const text = (translatedText || '').trim();
    if (!text) {
        subtitleTextBox.style.display = 'none';
        delete subtitleTextBox.dataset.hasText;
        return;
    }
    subtitleTextBox.dataset.hasText = '1';
    subtitleTextBox.textContent = text;
    subtitleTextBox.style.display = 'block';
    syncOverlayPosition();
}

async function captureAndSendRoiFrame() {
    if (translateState !== 'running' || workerBusy || !roiNorm || !ocrWorker) {
        return;
    }

    const canvas = getTargetCanvas();
    if (!canvas || canvas.width <= 0 || canvas.height <= 0) {
        return;
    }

    const sx = Math.max(0, Math.floor(roiNorm.nx * canvas.width));
    const sy = Math.max(0, Math.floor(roiNorm.ny * canvas.height));
    const sw = Math.min(canvas.width - sx, Math.ceil(roiNorm.nw * canvas.width));
    const sh = Math.min(canvas.height - sy, Math.ceil(roiNorm.nh * canvas.height));

    if (sw < 8 || sh < 8) return;

    workerBusy = true;
    try {
        // 异步裁剪 Canvas 指定区域为 ImageBitmap，不阻塞主线程渲染
        const bitmap = await createImageBitmap(canvas, sx, sy, sw, sh);
        if (translateState !== 'running' || !ocrWorker) {
            bitmap.close();
            workerBusy = false;
            return;
        }
        ocrWorker.postMessage({ type: 'OCR_FRAME', bitmap }, [bitmap]);
    } catch (err) {
        workerBusy = false;
        console.warn('[video-translate] createImageBitmap failed:', err);
    }
}

function startAutoTranslate() {
    const canvas = getTargetCanvas();
    if (!canvas) return;

    cleanupOverlays();
    translateState = 'running';
    setButtonActive(true);

    const worker = ensureWorker();
    worker.postMessage({ type: 'RESET_DIFF' });

    createSubtitleOverlay(getVideoContainer(canvas));
    notifyToast('自动翻译已开启（点击侧边栏按钮可终止）');

    // 立即抓取首帧并开启定时循环
    captureAndSendRoiFrame();
    captureTimer = setInterval(captureAndSendRoiFrame, CAPTURE_INTERVAL_MS);
}

function cleanupOverlays() {
    if (selectorOverlay) {
        selectorOverlay.remove();
        selectorOverlay = null;
    }
    if (subtitleOverlay) {
        subtitleOverlay.remove();
        subtitleOverlay = null;
        roiGuideBox = null;
        subtitleTextBox = null;
    }
}

function stopTranslation(showTip = true) {
    if (captureTimer) {
        clearInterval(captureTimer);
        captureTimer = null;
    }
    cleanupOverlays();
    translateState = 'idle';
    workerBusy = false;
    roiNorm = null;
    setButtonActive(false);

    if (showTip) {
        notifyToast('自动翻译已终止');
    }
}

window.addEventListener('resize', syncOverlayPosition);
document.addEventListener('fullscreenchange', () => {
    requestAnimationFrame(syncOverlayPosition);
});

// =========================================================================
// 4. 注册侧边栏控制按钮
// =========================================================================

const translateBtn = addControlButton(
    'gamestreamTranslateToggleBtn',
    'Translate',
    'M12.87 15.07l-2.54-2.51.03-.03c1.74-1.94 2.98-4.17 3.71-6.53H17V4h-7V2H8v2H1v1.99h11.17C11.5 7.92 10.44 9.75 9 11.35 8.07 10.32 7.3 9.19 6.69 8h-2c.73 1.63 1.73 3.17 2.98 4.56l-5.09 5.02L4 19l5-5 3.11 3.11.76-2.04zM18.5 10h-2L12 22h2l1.12-3h4.75L21 22h2l-4.5-12zm-2.62 7l1.62-4.33L19.12 17h-3.24z'
);

if (translateBtn) {
    translateBtn.addEventListener('click', () => {
        if (translateState === 'idle') {
            startRegionSelection();
        } else {
            stopTranslation(true);
        }
    });
}