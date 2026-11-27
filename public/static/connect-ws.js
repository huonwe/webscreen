let mediaWorker = null;
let mediaWorkerReady = false;
let upscaleEnabled = false;
let audioContext = null;
let nextAudioTime = 0;
let audioQueue = [];
let audioContextRequested = false;
let audioStartListenersAdded = false;

function status(text) {
    let el = document.getElementById('wsStatus');
    if (!el) {
        el = document.createElement('div');
        el.id = 'wsStatus';
        el.style.cssText = 'position:fixed;left:12px;bottom:12px;padding:6px 8px;background:rgba(0,0,0,.75);color:#fff;z-index:100';
        document.body.appendChild(el);
    }
    el.textContent = text;
}

async function initWebCodecs(mediaMeta, videoCodec) {
    if (!window.Worker || !HTMLCanvasElement.prototype.transferControlToOffscreen) {
        throw new Error('This browser does not support OffscreenCanvas workers');
    }
    const canvas = document.getElementById('remoteCanvas');
    if (!(canvas instanceof HTMLCanvasElement)) {
        throw new Error('WebSocket media output canvas was not found');
    }
    if (mediaWorker) mediaWorker.terminate();
    mediaWorker = null;
    mediaWorkerReady = false;

    const worker = new Worker('/static/canvasWorkerWS.js');
    window.streamWorker = worker;
    let offscreen;
    try {
        offscreen = canvas.transferControlToOffscreen();
    } catch (error) {
        worker.terminate();
        throw error;
    }
    mediaWorker = worker;
    const width = mediaMeta?.width || 1280;
    const height = mediaMeta?.height || 720;
    enableAudioStart();
    return new Promise((resolve, reject) => {
        worker.onmessage = (event) => {
            if (event.data.type === 'ready') {
                mediaWorkerReady = true;
                resolve(event.data.audioAvailable);
            } else if (event.data.type === 'audio') {
                try {
                    playAudioData(event.data.audioData);
                } catch (error) {
                    console.error('Failed to play decoded WebSocket audio:', error);
                    status(error.message);
                } finally {
                    event.data.audioData.close();
                }
            } else if (event.data.type === 'source-frame') {
                if (typeof window.handleWebSRSourceFrame === 'function') {
                    window.handleWebSRSourceFrame(event.data.frame);
                } else {
                    event.data.frame.close();
                }
            } else if (event.data.type === 'warning') {
                console.warn('WebSocket media worker:', event.data.message);
                status(event.data.message);
            } else if (event.data.type === 'error') {
                console.error('WebSocket media worker error:', event.data.message);
                status(event.data.message);
                if (!mediaWorkerReady) {
                    worker.terminate();
                    mediaWorker = null;
                    reject(new Error(event.data.message));
                }
            }
        };
        worker.onerror = (event) => {
            const error = new Error(event.message || 'WebSocket media worker failed');
            const wasReady = mediaWorkerReady;
            status(error.message);
            mediaWorkerReady = false;
            if (mediaWorker === worker) mediaWorker = null;
            worker.terminate();
            if (!wasReady) reject(error);
        };

        try {
            worker.postMessage({
                type: 'init',
                canvas: offscreen,
                codec: videoCodec || 'h264',
                width,
                height,
                upscalingEnabled: upscaleEnabled || window.isWebSRSourceEnabled?.() || false
            }, [offscreen]);
        } catch (error) {
            worker.terminate();
            mediaWorker = null;
            reject(error);
        }
    });
}

async function ensureAudioStarted() {
    const AudioContextClass = window.AudioContext || window.webkitAudioContext;
    if (!AudioContextClass) {
        status('This browser does not support Web Audio');
        return false;
    }

    if (!audioContext) {
        // 移除强制 sampleRate: 48000，避免部分手机硬件采样率不匹配导致初始化异常
        // createBufferSource 会自动重采样到设备原生采样率
        audioContext = new AudioContextClass({
            latencyHint: 'interactive'
        });
        nextAudioTime = 0;
    }

    // 1. iOS 静音解锁补丁：同步播放一个 1 帧的静音 Buffer 激活底层音频通道
    try {
        const silentBuffer = audioContext.createBuffer(1, 1, 22050);
        const silentSource = audioContext.createBufferSource();
        silentSource.buffer = silentBuffer;
        silentSource.connect(audioContext.destination);
        silentSource.start(0);
    } catch (e) {
        console.warn('Silent buffer unlock warning:', e);
    }

    // 2. 兼容 iOS 切后台后产生的 'interrupted' 状态
    if (audioContext.state === 'suspended' || audioContext.state === 'interrupted') {
        await audioContext.resume();
    }

    if (audioContext.state !== 'running') {
        status('Click or tap again to enable audio');
        return false;
    }

    // 解锁成功后移除监听，避免后续每次点击都重复触发
    removeAudioStartListeners();
    status('Audio enabled');
    drainAudioQueue();
    return true;
}



function queueAudioData(audioData) {
    if (audioQueue.length >= 12) {
        audioQueue.shift();
    }

    const channels = [];
    for (let channel = 0; channel < audioData.numberOfChannels; channel++) {
        const samples = new Float32Array(audioData.numberOfFrames);
        audioData.copyTo(samples, { planeIndex: channel, format: 'f32-planar' });
        channels.push(samples);
    }

    audioQueue.push({
        channels,
        frameCount: audioData.numberOfFrames,
        sampleRate: audioData.sampleRate
    });
}

function drainAudioQueue() {
    if (!audioContext || audioContext.state !== 'running') return;
    while (audioQueue.length > 0) {
        playQueuedAudio(audioQueue.shift());
    }
}

function playAudioData(audioData) {
    if (!audioContext || audioContext.state !== 'running') {
        queueAudioData(audioData);
        // status('Click or tap to enable audio');
        return;
    }

    const channelCount = audioData.numberOfChannels;
    const frameCount = audioData.numberOfFrames;
    const channels = [];
    for (let channel = 0; channel < channelCount; channel++) {
        const samples = new Float32Array(frameCount);
        audioData.copyTo(samples, { planeIndex: channel, format: 'f32-planar' });
        channels.push(samples);
    }
    playChannels(channels, frameCount, audioData.sampleRate);
}

function playQueuedAudio(chunk) {
    playChannels(chunk.channels, chunk.frameCount, chunk.sampleRate);
}

function playChannels(channels, frameCount, sampleRate) {
    if (!audioContext || audioContext.state !== 'running') return;

    const buffer = audioContext.createBuffer(channels.length, frameCount, sampleRate);
    channels.forEach((samples, channel) => {
        buffer.copyToChannel(samples, channel);
    });
    const source = audioContext.createBufferSource();
    source.buffer = buffer;
    source.connect(audioContext.destination);

    const now = audioContext.currentTime;
    if (nextAudioTime < now) {
        nextAudioTime = now;
    } else if (nextAudioTime - now > 0.05) {
        nextAudioTime = now;
    }

    source.start(nextAudioTime);
    nextAudioTime += frameCount / sampleRate;
}

function requestAudioStart() {
    if (audioContextRequested) return;
    audioContextRequested = true;
    ensureAudioStarted().finally(() => {
        audioContextRequested = false;
    });
}

function enableAudioStart() {
    if (audioStartListenersAdded) return;
    // 3. 移动端必须监听 touchend / click，Safari 不会在 pointerdown 时放行 AudioContext
    document.addEventListener('touchend', requestAudioStart, { passive: true });
    document.addEventListener('click', requestAudioStart, { passive: true });
    document.addEventListener('keydown', requestAudioStart);
    audioStartListenersAdded = true;
}

function removeAudioStartListeners() {
    if (!audioStartListenersAdded) return;
    document.removeEventListener('touchend', requestAudioStart);
    document.removeEventListener('click', requestAudioStart);
    document.removeEventListener('keydown', requestAudioStart);
    audioStartListenersAdded = false;
}

function handleMediaFrame(buffer) {
    if (!mediaWorker || !mediaWorkerReady) {
        status('WebSocket media worker is not initialized');
        return;
    }
    try {
        mediaWorker.postMessage({ type: 'frame', buffer }, [buffer]);
    } catch (error) {
        console.error('Failed to send WebSocket media frame to worker:', error);
        status(error.message);
    }
}

function stopMediaWorker() {
    if (mediaWorker) mediaWorker.terminate();
    mediaWorker = null;
    mediaWorkerReady = false;
}

function setUpscaling(enabled) {
    upscaleEnabled = Boolean(enabled);
    if (mediaWorker) {
        mediaWorker.postMessage({ type: 'upscaling', enabled: upscaleEnabled });
    }
}

window.WebSocketMedia = {
    initialize: initWebCodecs,
    handleFrame: handleMediaFrame,
    enableAudioStart,
    status,
    setUpscaling,
    stop: stopMediaWorker
};