// websrWorker.js
import WebSR from 'https://esm.sh/@websr/websr';

let websr = null;
let outputCanvas = null;
let sourcePort = null;

// 下沉到 Worker 内部的最新帧覆盖状态
let isProcessing = false;
let pendingFrame = null;
let lastOutWidth = 0;
let lastOutHeight = 0;

function bindSourcePort(port) {
    if (sourcePort) {
        sourcePort.onmessage = null;
        sourcePort.close();
    }
    if (pendingFrame) {
        pendingFrame.close();
        pendingFrame = null;
    }

    sourcePort = port;
    sourcePort.onmessage = (event) => {
        if (event.data.type === 'frame') {
            handleIncomingFrame(event.data.frame);
        }
    };
}

function handleIncomingFrame(frame) {
    if (!websr) {
        frame.close();
        return;
    }
    // 如果上一帧还在 GPU 渲染中，丢弃排队中的旧帧，只保留最新帧
    if (isProcessing) {
        if (pendingFrame) pendingFrame.close();
        pendingFrame = frame;
        return;
    }
    renderLoop(frame);
}

async function renderLoop(initialFrame) {
    let currentFrame = initialFrame;
    isProcessing = true;

    try {
        while (currentFrame) {
            try {
                await websr.render(currentFrame);

                // 仅在首次渲染或分辨率变化时通知主线程调整 Canvas 布局，减少主线程打扰
                if (outputCanvas.width !== lastOutWidth || outputCanvas.height !== lastOutHeight) {
                    lastOutWidth = outputCanvas.width;
                    lastOutHeight = outputCanvas.height;
                    self.postMessage({
                        type: 'done',
                        width: lastOutWidth,
                        height: lastOutHeight
                    });
                }
            } catch (error) {
                self.postMessage({ type: 'error', message: error.message || String(error) });
            } finally {
                currentFrame.close();
            }

            // 取出渲染期间到达的最新一帧继续画，没有则退出循环
            currentFrame = pendingFrame;
            pendingFrame = null;
        }
    } finally {
        isProcessing = false;
    }
}

self.onmessage = async (event) => {
    const { type } = event.data;

    // 接收来自主线程分配的直连端口（支持随时切换 WebRTC / WS 源）
    if (type === 'set-source-port') {
        bindSourcePort(event.data.port);
        return;
    }

    if (type === 'init') {
        outputCanvas = event.data.canvas;
        if (event.data.port) {
            bindSourcePort(event.data.port);
        }
        try {
            const gpu = await WebSR.initWebGPU();
            if (!gpu) {
                throw new Error('This browser or device does not support WebGPU');
            }

            let resolution_width = event.data.resolution_width;
            let resolution_height = event.data.resolution_height;
            if (resolution_width < resolution_height) {
                [resolution_width, resolution_height] = [resolution_height, resolution_width];
            }

            let network_name, weights;
            if (isMobile()) {
                network_name = "anime4k/cnn-2x-s";
                weights = await (await fetch('./cnn-2x-s-an.json')).json();
            } else if (isLowerThan1080P(resolution_width, resolution_height)) {
                network_name = "websr/cnn-s";
                weights = WebSR.parseBinaryWeights(await (await fetch('./cnn-s.bin')).arrayBuffer());
            } else if (is1080P(resolution_width, resolution_height)) {
                network_name = "anime4k/cnn-2x-m";
                weights = await (await fetch('./cnn-2x-m-an.json')).json();
            } else if (isEqualHigherThan4K(resolution_width, resolution_height)) {
                network_name = "anime4k/cnn-restore-m";
                weights = await (await fetch('./cnn-restore-m-an.json')).json();
            } else {
                network_name = "anime4k/cnn-2x-s";
                weights = await (await fetch('./cnn-2x-s-an.json')).json();
            }

            websr = new WebSR({
                network_name,
                weights,
                gpu,
                canvas: outputCanvas
            });

            self.postMessage({ type: 'ready' });
        } catch (error) {
            self.postMessage({ type: 'error', message: error.message || String(error) });
        }
        return;
    }

    // 保留兼容性：如果仍有直接发来的 frame 也能处理
    if (type === 'frame') {
        // handleIncomingFrame(event.data.frame);
        console.warn('Received frame directly, but handling is not implemented. You should use a MessageChannel to connect the stream worker to the WebSR worker.');
    }
};

function isMobile() { return /Mobi|Android/i.test(navigator.userAgent); }
function isLowerThan1080P(w, h) { return w < 1920 || h < 1080; }
function isEqualHigherThan4K(w, h) { return w >= 3840 || h >= 2160; }
function is1080P(w, h) { return w === 1920 && h === 1080; }