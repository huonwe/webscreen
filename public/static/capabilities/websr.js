let worker = null;
let outputCanvas = null;
let renderGeneration = 0;
let sourceStyle = null;
let resizeObserver = null;
let isProcessing = false;
let isReady = false;
let isShowingOutput = false;
let pendingSourceFrame = null;

function getSourceCanvas() {
    return document.getElementById('remoteCanvas');
}

function getVideoContainer() {
    return document.querySelector('.canvas-container');
}

function updateCanvasBounds() {
    const source = getSourceCanvas();
    const container = getVideoContainer();
    if (!source || !container || !outputCanvas) return;

    const sourceRect = source.getBoundingClientRect();
    const containerRect = container.getBoundingClientRect();
    Object.assign(outputCanvas.style, {
        left: `${sourceRect.left - containerRect.left}px`,
        top: `${sourceRect.top - containerRect.top}px`,
        width: `${sourceRect.width}px`,
        height: `${sourceRect.height}px`
    });
}

function createOutputCanvas() {
    const source = getSourceCanvas();
    const container = getVideoContainer();
    if (!source || !container) {
        throw new Error('WebSR requires the remote canvas and its container');
    }

    if (getComputedStyle(container).position === 'static') {
        container.style.position = 'relative';
    }

    outputCanvas = document.createElement('canvas');
    outputCanvas.id = 'websr-output';
    outputCanvas.setAttribute('aria-hidden', 'true');
    Object.assign(outputCanvas.style, {
        position: 'absolute',
        zIndex: '0',
        pointerEvents: 'none',
        display: 'block',
        objectFit: 'contain',
        objectPosition: 'center'
    });

    sourceStyle = {
        opacity: source.style.opacity,
        position: source.style.position,
        zIndex: source.style.zIndex,
        transition: source.style.transition
    };
    source.style.position = 'relative';
    source.style.zIndex = '1';

    container.insertBefore(outputCanvas, source);
    updateCanvasBounds();
    resizeObserver = new ResizeObserver(updateCanvasBounds);
    resizeObserver.observe(source);
    window.addEventListener('resize', updateCanvasBounds);
}

function removeOutputCanvas() {
    const source = getSourceCanvas();
    if (resizeObserver) {
        resizeObserver.disconnect();
        resizeObserver = null;
    }
    window.removeEventListener('resize', updateCanvasBounds);

    if (source && sourceStyle) {
        source.style.opacity = sourceStyle.opacity;
        source.style.position = sourceStyle.position;
        source.style.zIndex = sourceStyle.zIndex;
        source.style.transition = sourceStyle.transition;
    }
    if (outputCanvas) outputCanvas.remove();
    outputCanvas = null;
    sourceStyle = null;
}

function failUpscaling(error) {
    console.error('WebSR rendering failed', error);
    disableUpscaling();
    if (typeof showToast === 'function') {
        showToast(`超分辨率失败: ${error.message || error}`, 3000);
    }
}

function sendFrame(frame, generation) {
    if (!worker || !outputCanvas || generation !== renderGeneration) {
        frame.close();
        return;
    }
    if (!isReady || isProcessing) {
        if (pendingSourceFrame) pendingSourceFrame.close();
        pendingSourceFrame = frame;
        return;
    }

    isProcessing = true;
    try {
        worker.postMessage({ type: 'frame', frame }, [frame]);
    } catch (error) {
        frame.close();
        isProcessing = false;
        if (generation === renderGeneration) failUpscaling(error);
    }
}

window.notifySourceFrameUpdated = function (frame) {
    if (!frame || typeof frame.close !== 'function') return;
    if (!worker || !outputCanvas || renderGeneration === 0) {
        frame.close();
        return;
    }
    sendFrame(frame, renderGeneration);
};

function enableUpscaling(button) {
    const source = getSourceCanvas();
    if (!(source instanceof HTMLCanvasElement)) {
        throw new Error('WebSR requires remoteCanvas to be a canvas');
    }
    if (!source.width || !source.height) {
        throw new Error('WebSR cannot start before the canvas has a frame');
    }
    if (!('transferControlToOffscreen' in HTMLCanvasElement.prototype)) {
        throw new Error('This browser does not support OffscreenCanvas');
    }
    if (typeof window.setWebSRSourceEnabled !== 'function') {
        throw new Error('The active video pipeline cannot forward source frames');
    }

    createOutputCanvas();
    const generation = ++renderGeneration;
    isProcessing = false;
    isReady = false;
    isShowingOutput = false;
    pendingSourceFrame = null;
    window.setWebSRSourceEnabled(true);

    const ready = new Promise((resolve, reject) => {
        worker = new Worker(new URL('../websr/worker.js', import.meta.url), {
            type: 'module'
        });

        worker.onmessage = (event) => {
            const { type, message, width, height } = event.data;
            if (generation !== renderGeneration) return;

            if (type === 'ready') {
                isReady = true;
                button.classList.add('active');
                button.setAttribute('aria-pressed', 'true');
                resolve();
            } else if (type === 'done') {
                isProcessing = false;
                if (!isShowingOutput) {
                    if (!width || !height) {
                        failUpscaling(new Error('WebSR output canvas is empty'));
                        return;
                    }
                    isShowingOutput = true;
                    const renderedSource = getSourceCanvas();
                    renderedSource.style.transition = 'opacity 240ms ease';
                    renderedSource.style.opacity = '0';
                }
                if (pendingSourceFrame) {
                    const nextFrame = pendingSourceFrame;
                    pendingSourceFrame = null;
                    sendFrame(nextFrame, generation);
                }
            } else if (type === 'error') {
                const error = new Error(message || 'Unknown WebSR worker error');
                if (!isReady) reject(error);
                else failUpscaling(error);
            }
        };
        worker.onerror = (event) => {
            const error = new Error(event.message || 'WebSR worker failed');
            if (!isReady) reject(error);
            else failUpscaling(error);
        };

        try {
            const offscreen = outputCanvas.transferControlToOffscreen();
            worker.postMessage({ type: 'init', canvas: offscreen }, [offscreen]);
        } catch (error) {
            reject(error);
        }
    });

    return ready.then(() => {
        if (pendingSourceFrame) {
            const firstFrame = pendingSourceFrame;
            pendingSourceFrame = null;
            sendFrame(firstFrame, generation);
        }
    }).catch((error) => {
        if (generation === renderGeneration) disableUpscaling();
        throw error;
    });
}

function disableUpscaling() {
    renderGeneration++;
    window.setWebSRSourceEnabled(false);
    if (pendingSourceFrame) pendingSourceFrame.close();
    pendingSourceFrame = null;
    if (worker) worker.terminate();
    worker = null;
    isProcessing = false;
    isReady = false;
    isShowingOutput = false;
    removeOutputCanvas();

    const button = document.getElementById('websrToggleButton');
    if (button) {
        button.classList.remove('active');
        button.setAttribute('aria-pressed', 'false');
    }
}

function createWebSRButton() {
    const controls = document.querySelector('.controls-container');
    if (!controls || document.getElementById('websrToggleButton')) return;

    const button = document.createElement('button');
    button.id = 'websrToggleButton';
    button.className = 'control-btn';
    button.type = 'button';
    button.title = 'Toggle super resolution';
    button.setAttribute('aria-label', button.title);
    button.setAttribute('aria-pressed', 'false');
    button.innerHTML = `
        <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor">
            <path d="m12 2 1.8 6.2L20 10l-6.2 1.8L12 18l-1.8-6.2L4 10l6.2-1.8L12 2zm7 13 .8 2.2L22 18l-2.2.8L19 21l-.8-2.2L16 18l2.2-.8L19 15z"/>
        </svg>`;
    button.addEventListener('click', async () => {
        if (worker) {
            disableUpscaling();
            return;
        }

        button.disabled = true;
        try {
            await enableUpscaling(button);
        } catch (error) {
            console.error('Failed to enable WebSR', error);
            if (typeof showToast === 'function') {
                showToast(`无法启用超分辨率: ${error.message}`, 3000);
            }
        } finally {
            button.disabled = false;
        }
    });
    controls.appendChild(button);
}

createWebSRButton();
