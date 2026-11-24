var CONFIG = (function () {
    const stored = sessionStorage.getItem('webscreen_device_configs_now');
    if (stored) {
        try {
            return JSON.parse(stored);
        } catch (error) {
            console.warn('Failed to parse stored config:', error);
        }
    }
    return {
        type: 'android',
        av_sync: false,
        driver_config: {
            video_codec: 'h265',
            audio_codec: 'opus',
            audio: 'true',
            video_bit_rate: '8000000',
            control: 'true',
        }
    };
})();

// const remoteVideo = document.getElementById('remoteVideo');

let videoDecoder = null;
let audioDecoder = null;
let audioContext = null;
let nextAudioTime = 0;
let audioQueue = [];
let audioContextRequested = false;
let videoConfig = null;
let videoNeedsKeyFrame = true;
let lastVideoTimestamp = -1;
let videoFrameCount = 0;

function splitAnnexB(payload) {
    const nalus = [];
    let offset = 0;

    for (let i = 0; i + 3 < payload.length;) {
        let start = -1;
        let startSize = 0;

        if (payload[i] === 0 && payload[i + 1] === 0 &&
            payload[i + 2] === 0 && payload[i + 3] === 1) {
            start = i;
            startSize = 4;
        } else if (payload[i] === 0 && payload[i + 1] === 0 &&
            payload[i + 2] === 1) {
            start = i;
            startSize = 3;
        }

        if (start !== -1) {
            if (offset >= 0 && offset < start) {
                nalus.push(payload.subarray(offset, start));
            }
            offset = start + startSize;
            i = offset;
        } else {
            i++;
        }
    }

    if (offset < payload.length) {
        nalus.push(payload.subarray(offset));
    }
    return nalus.filter(nalu => nalu.length > 0);
}

function h264ToAvcc(payload) {
    let nalus = splitAnnexB(payload);
    if (nalus.length === 1 && nalus[0] === payload) {
        const avccNalus = [];
        let offset = 0;
        let looksLikeAvcc = true;
        while (offset + 4 <= payload.length) {
            const naluSize = new DataView(payload.buffer, payload.byteOffset + offset, 4).getUint32(0, false);
            offset += 4;
            if (naluSize === 0 || offset + naluSize > payload.length) {
                looksLikeAvcc = false;
                break;
            }
            avccNalus.push(payload.subarray(offset, offset + naluSize));
            offset += naluSize;
        }
        if (looksLikeAvcc && offset === payload.length) {
            nalus = avccNalus;
        }
    }
    if (nalus.length === 0) nalus = [payload];

    const avcc = [];
    for (const nalu of nalus) {
        const length = new Uint8Array(4);
        new DataView(length.buffer).setUint32(0, nalu.length, false);
        avcc.push(length, nalu);
    }

    let size = 0;
    for (const part of avcc) size += part.length;
    const result = new Uint8Array(size);
    let offset = 0;
    for (const part of avcc) {
        result.set(part, offset);
        offset += part.length;
    }
    return result;
}

function updateAvcConfiguration(payload) {
    let nalus = splitAnnexB(payload);
    if (nalus.length === 0) nalus = [payload];

    const sps = nalus.find(nalu => (nalu[0] & 0x1f) === 7);
    const pps = nalus.find(nalu => (nalu[0] & 0x1f) === 8);
    if (!sps || sps.length < 4 || !pps) return;

    const description = new Uint8Array(6 + 2 + sps.length + 1 + 2 + pps.length);
    description.set([
        1,
        sps[1],
        sps[2],
        sps[3],
        0xff,
        0xe1
    ]);
    let offset = 6;
    new DataView(description.buffer).setUint16(offset, sps.length, false);
    offset += 2;
    description.set(sps, offset);
    offset += sps.length;
    description[offset++] = 1;
    new DataView(description.buffer).setUint16(offset, pps.length, false);
    offset += 2;
    description.set(pps, offset);

    videoConfig = {
        codec: `avc1.${sps[1].toString(16).padStart(2, '0')}${sps[2].toString(16).padStart(2, '0')}${sps[3].toString(16).padStart(2, '0')}`,
        codedWidth: videoConfig?.codedWidth || 1280,
        codedHeight: videoConfig?.codedHeight || 720,
        description
    };
}

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

async function initWebCodecs(mediaMeta) {
    if (!window.VideoDecoder || !window.EncodedVideoChunk) {
        throw new Error('This browser does not support WebCodecs VideoDecoder');
    }
    if (!window.AudioDecoder || !window.EncodedAudioChunk) {
        throw new Error('This browser does not support WebCodecs AudioDecoder');
    }

    const canvas = document.getElementById('remoteVideo');
    const ctx = canvas.getContext('2d');
    const videoCodec = CONFIG.driver_config?.video_codec || 'h264';
    const width = mediaMeta?.width || 1280;
    const height = mediaMeta?.height || 720;

    videoConfig = null;
    switch (videoCodec.toLowerCase()) {
        case 'h264':
            videoConfig = { codec: 'avc1.640028', codedWidth: width, codedHeight: height };
            break;
        case 'h265':
        case 'hevc':
            videoConfig = { codec: 'hvc1.1.6.L93.B0', codedWidth: width, codedHeight: height };
            break;
        case 'av1':
            videoConfig = { codec: 'av01.0.08M.08', codedWidth: width, codedHeight: height };
            break;
        default:
            throw new Error(`Unsupported test codec: ${videoCodec}`);
    }

    const videoSupport = await VideoDecoder.isConfigSupported(videoConfig);
    if (!videoSupport.supported) {
        throw new Error(`Browser cannot decode ${videoConfig.codec}`);
    }

    videoDecoder = new VideoDecoder({
        output(frame) {
            if (canvas.width !== frame.displayWidth) canvas.width = frame.displayWidth;
            if (canvas.height !== frame.displayHeight) canvas.height = frame.displayHeight;
            ctx.drawImage(frame, 0, 0, canvas.width, canvas.height);
            frame.close();
        },
        error(error) {
            console.error('VideoDecoder error:', error);
            status(`Video decoder error: ${error}`);
            videoNeedsKeyFrame = true;
            try {
                videoDecoder.reset();
                videoDecoder.configure(videoConfig);
            } catch (resetError) {
                console.error('Failed to reset video decoder:', resetError);
            }
        }
    });
    videoDecoder.configure(videoSupport.config);

    const audioConfig = { codec: 'opus', sampleRate: 48000, numberOfChannels: 2 };
    const audioSupport = await AudioDecoder.isConfigSupported(audioConfig);
    if (!audioSupport.supported) {
        console.warn('Browser cannot decode Opus with WebCodecs');
        return;
    }

    // Do not create the AudioContext during WebSocket/metadata handling.
    // It is created and resumed by a user gesture below.
    audioDecoder = new AudioDecoder({
        output(audioData) {
            playAudioData(audioData);
            audioData.close();
        },
        error(error) {
            console.error('AudioDecoder error:', error);
            status(`Audio decoder error: ${error}`);
        }
    });
    audioDecoder.configure(audioSupport.config);
}

async function ensureAudioStarted() {
    if (!window.AudioContext && !window.webkitAudioContext) {
        status('This browser does not support Web Audio');
        return false;
    }

    if (!audioContext) {
        const AudioContextClass = window.AudioContext || window.webkitAudioContext;
        audioContext = new AudioContextClass({ sampleRate: 48000 });
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
    if (audioQueue.length >= 100) {
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
    if (nextAudioTime < now + 0.01) nextAudioTime = now + 0.01;
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

document.addEventListener('pointerdown', requestAudioStart);
document.addEventListener('keydown', requestAudioStart);

function handleMediaFrame(buffer) {
    if (buffer.byteLength < 10) return;

    const bytes = new Uint8Array(buffer);
    const kind = bytes[0];
    const keyFrame = bytes[1] === 1;
    const pts = Number(new DataView(buffer).getBigUint64(2, false));
    let payload = bytes.subarray(10);
    // console.log(`Received media frame: kind=${kind}, keyFrame=${keyFrame}, pts=${pts}, payloadSize=${payload.length}`);
    if (kind === 0) {
        if (!videoDecoder || videoDecoder.state !== 'configured') return;
        if (videoDecoder.decodeQueueSize > 8) {
            videoNeedsKeyFrame = true;
            status('Dropping stale video frames');
            return;
        }
        if (CONFIG.driver_config?.video_codec === 'h264') {
            updateAvcConfiguration(payload);

            if (videoNeedsKeyFrame && keyFrame && videoConfig?.description) {
                videoDecoder.reset();
                videoDecoder.configure(videoConfig);
                videoNeedsKeyFrame = false;
            }

            if (videoNeedsKeyFrame) {
                if (!keyFrame) return;
            }

            payload = h264ToAvcc(payload);
        }

        let timestamp = Number.isFinite(pts) ? pts : 0;
        if (timestamp <= lastVideoTimestamp) {
            timestamp = lastVideoTimestamp + 16667;
        }
        lastVideoTimestamp = timestamp;
        videoFrameCount++;

        videoDecoder.decode(new EncodedVideoChunk({
            type: keyFrame ? 'key' : 'delta',
            timestamp,
            data: payload
        }));
    } else if (kind === 1) {
        if (!audioDecoder || audioDecoder.state !== 'configured') return;
        audioDecoder.decode(new EncodedAudioChunk({
            type: 'key',
            timestamp: pts,
            data: payload
        }));
    }
}

window.sendDataChannelMessage = function (channel, message) {
    // console.log('Sending message to channel', channel, ':', message);
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(message);
    }
};

async function start() {
    // if (!CONFIG.remote_uuid) {
    //     status('Missing remote_uuid');
    //     console.error('No device configuration found');
    //     return;
    // }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    window.ws = new WebSocket(`${protocol}//${window.location.host}/screen-ws/wsn`);
    window.ws.binaryType = 'arraybuffer';

    window.ws.onopen = () => {
        window.ws.send(JSON.stringify({
            protocol: 'websocket',
            video_codec: 'h265',
            audio_codec: CONFIG.driver_config?.audio_codec || 'opus',
            remote_uuid: '68bf97a0-011c-40b5-8710-cb49a5bad369',
            extra: { av_sync: String(CONFIG.av_sync === true) }
        }));
        status('WebSocket connected');
    };

    let remoteConfigSent = false;
    window.ws.onmessage = async (event) => {
        if (event.data instanceof ArrayBuffer) {
            handleMediaFrame(event.data);
            return;
        }

        let message;
        try {
            message = JSON.parse(event.data);
        } catch (error) {
            console.warn('Unknown text WebSocket message:', event.data);
            return;
        }

        if (message.status === 'error') {
            status(message.message || 'Server error');
            return;
        }

        if (message.stage === 'metainfo_remote') {
            if (message.media_meta) {
                try {
                    await initWebCodecs(message.media_meta);
                    status('WebCodecs initialized');
                } catch (error) {
                    console.error(error);
                    status(error.message);
                }
            }
            if (message.capabilities) {
                console.log('Received capabilities from remote:', message.capabilities);
                await updateUIBasedOnCapabilities(message.capabilities);
            }
            return;
        }

        // if (message.useWSTransport) status('WebSocket media transport');

        if (!remoteConfigSent) {
            remoteConfigSent = true;
            const remoteConfig = { ...(CONFIG.driver_config || {}) };
            if (CONFIG.type === 'android') {
                remoteConfig.deviceID = CONFIG.device_ip || CONFIG.device_id || '';
            }
            window.ws.send(JSON.stringify(remoteConfig));
        }
    };

    window.ws.onerror = (event) => {
        console.error('WebSocket error:', event);
        status('WebSocket error');
    };
    window.ws.onclose = () => status('WebSocket closed');
}

function loadScript(src) {
    return new Promise((resolve, reject) => {
        // Check if script is already loaded
        if (document.querySelector(`script[src="${src}"]`)) {
            resolve();
            return;
        }
        const script = document.createElement('script');
        script.src = src;
        script.type = 'module';
        script.onload = resolve;
        script.onerror = reject;
        document.head.appendChild(script);
    });
}

async function updateUIBasedOnCapabilities(caps) {
    if (!caps) return;

    await loadScript('/static/capabilities/websr.js');

    // Handle Control
    if (caps.can_control) {
        // Load control scripts
        try {
            if (caps.is_linux) {
                await loadScript('/static/capabilities/linux_mouse.js');
                await loadScript('/static/capabilities/keyboard.js');
                await loadScript('/static/capabilities/touch.js');
            }
            if (caps.is_android) {
                await loadScript('/static/capabilities/keyboard.js');
                await loadScript('/static/capabilities/touch.js');
                await loadScript('/static/capabilities/scroll.js');
                await loadScript('/static/capabilities/buttons.js');

                await loadScript('/static/capabilities/keep_screen_on.js');
                await loadScript('/static/capabilities/backlight.js');

            }

            console.log("Control scripts loaded");
        } catch (e) {
            console.error("Failed to load control scripts", e);
        }

        // Handle Clipboard
        if (caps.can_clipboard) {
            try {
                // Add timestamp to force cache busting
                await loadScript('/static/capabilities/clipboard.js');
            } catch (e) {
                console.error("Failed to load clipboard script", e);
            }
        }
        // Handle UHID
        if (caps.can_uhid) {
            try {
                await loadScript('/static/capabilities/uhid_mouse.js');
                await loadScript('/static/capabilities/uhid_keyboard.js');
                await loadScript('/static/capabilities/uhid_gamepad.js');
                console.log("UHID scripts loaded");
            } catch (e) {
                console.error("Failed to load UHID scripts", e);
            }
        }

    }

}