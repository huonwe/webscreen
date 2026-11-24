let mediaWorker = null;
let mediaWorkerReady = false;
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
    const canvas = document.getElementById('remoteVideo');
    if (!(canvas instanceof HTMLCanvasElement)) {
        throw new Error('WebSocket media output canvas was not found');
    }
    if (mediaWorker) mediaWorker.terminate();
    mediaWorker = null;
    mediaWorkerReady = false;

    const worker = new Worker('/static/canvasWorkerWS.js');
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
                height
            }, [offscreen]);
        } catch (error) {
            worker.terminate();
            mediaWorker = null;
            reject(error);
        }
    });
}

async function ensureAudioStarted() {
    if (!window.AudioContext && !window.webkitAudioContext) {
        status('This browser does not support Web Audio');
        return false;
    }

    if (!audioContext) {
        const AudioContextClass = window.AudioContext || window.webkitAudioContext;
        audioContext = new AudioContextClass({
            sampleRate: 48000,
            latencyHint: 'interactive'
        });
        nextAudioTime = 0;
    }

    if (audioContext.state === 'suspended') {
        await audioContext.resume();
    }

    if (audioContext.state !== 'running') {
        status('Click or tap again to enable audio');
        return false;
    }

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
        status('Click or tap to enable audio');
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
    document.addEventListener('pointerdown', requestAudioStart);
    document.addEventListener('keydown', requestAudioStart);
    audioStartListenersAdded = true;
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

window.WebSocketMedia = {
    initialize: initWebCodecs,
    handleFrame: handleMediaFrame,
    enableAudioStart,
    status,
    stop: stopMediaWorker
};