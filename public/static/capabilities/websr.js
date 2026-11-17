import WebSR from '@websr/websr';

let websr = null;
let outputCanvas = null;
let renderGeneration = 0;
let videoStyle = null;
let resizeObserver = null;

function getVideo() {
    return document.getElementById('remoteVideo');
}

function getVideoContainer() {
    return document.querySelector('.video-container');
}

function updateCanvasBounds() {
    const video = getVideo();
    const container = getVideoContainer();
    if (!video || !container || !outputCanvas) return;

    const videoRect = video.getBoundingClientRect();
    const containerRect = container.getBoundingClientRect();
    Object.assign(outputCanvas.style, {
        left: `${videoRect.left - containerRect.left}px`,
        top: `${videoRect.top - containerRect.top}px`,
        width: `${videoRect.width}px`,
        height: `${videoRect.height}px`
    });
}

function createOutputCanvas() {
    const video = getVideo();
    const container = getVideoContainer();
    if (!video || !container) {
        throw new Error('WebSR requires the remote video and its container');
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

    videoStyle = {
        opacity: video.style.opacity,
        position: video.style.position,
        zIndex: video.style.zIndex,
        transition: video.style.transition
    };
    video.style.position = 'relative';
    video.style.zIndex = '1';

    container.insertBefore(outputCanvas, video);
    updateCanvasBounds();
    resizeObserver = new ResizeObserver(updateCanvasBounds);
    resizeObserver.observe(video);
    window.addEventListener('resize', updateCanvasBounds);
}

function removeOutputCanvas() {
    const video = getVideo();
    if (resizeObserver) {
        resizeObserver.disconnect();
        resizeObserver = null;
    }
    window.removeEventListener('resize', updateCanvasBounds);

    if (video && videoStyle) {
        video.style.opacity = videoStyle.opacity;
        video.style.position = videoStyle.position;
        video.style.zIndex = videoStyle.zIndex;
        video.style.transition = videoStyle.transition;
    }
    if (outputCanvas) outputCanvas.remove();
    outputCanvas = null;
    videoStyle = null;
}

async function renderFrame(generation, onFirstFrame) {
    if (!websr || !outputCanvas || generation !== renderGeneration) return;

    try {
        await websr.render(getVideo());
        if (generation !== renderGeneration || !outputCanvas) return;
        if (onFirstFrame) {
            if (!outputCanvas.width || !outputCanvas.height) {
                throw new Error('WebSR output canvas is empty after rendering');
            }
            onFirstFrame();
        }
        getVideo().requestVideoFrameCallback(() => renderFrame(generation));
    } catch (error) {
        console.error('WebSR rendering failed', error);
        disableUpscaling();
    }
}

async function enableUpscaling(button) {
    const video = getVideo();
    if (!video || !video.videoWidth || !video.videoHeight) {
        throw new Error('WebSR cannot start before the video metadata is available');
    }

    (async () => {
        console.log("navigator.gpu:", navigator.gpu);
        const adapter = await navigator.gpu?.requestAdapter();
        console.log("adapter:", adapter);
        if (adapter) {
            console.log("features:", [...adapter.features]);
            const device = await adapter.requestDevice();
            console.log("device:", device);
        }
    })();

    createOutputCanvas();
    const gpu = await WebSR.initWebGPU();
    console.log('WebSR GPU support:', gpu);
    if (gpu === undefined) {
        throw new Error("This feature requires HTTPS");
    }
    if (!gpu) throw new Error("Browser/device doesn't support WebGPU. This feature also requires HTTPS.");

    // const weight_url = "../static/websr/cnn-xs.bin"
    // const buffer = await (await fetch(weight_url)).arrayBuffer();
    // if (!buffer) {
    //     throw new Error(`Failed to load WebSR weights from ${weight_url}`);
    // }
    const weight_url = "../static/websr/cnn-2x-s-an.json"
    const response = await fetch(weight_url);
    if (!response.ok) {
        throw new Error(`Failed to load WebSR weights (${response.status})`);
    }

    websr = new WebSR({
        network_name: 'anime4k/cnn-2x-s',
        weights: await response.json(),
        // network_name: 'websr/cnn-xs',
        // weights: WebSR.parseBinaryWeights(buffer),
        gpu,
        canvas: outputCanvas
    });

    const generation = ++renderGeneration;
    button.classList.add('active');
    button.setAttribute('aria-pressed', 'true');
    renderFrame(generation, () => {
        const renderedVideo = getVideo();
        renderedVideo.style.transition = 'opacity 240ms ease';
        requestAnimationFrame(() => {
            if (generation === renderGeneration && outputCanvas) {
                renderedVideo.style.opacity = '0';
            }
        });
    });
}

function disableUpscaling() {
    renderGeneration++;
    if (websr && typeof websr.destroy === 'function') websr.destroy();
    websr = null;
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
        if (websr) {
            disableUpscaling();
            return;
        }

        button.disabled = true;
        try {
            await enableUpscaling(button);
        } catch (error) {
            console.error('Failed to enable WebSR', error);
            disableUpscaling();
            if (typeof showToast === 'function') {
                showToast(`无法启用超采样: ${error.message}`, 3000);
            }
        } finally {
            button.disabled = false;
        }
    });
    controls.appendChild(button);
}

createWebSRButton();
