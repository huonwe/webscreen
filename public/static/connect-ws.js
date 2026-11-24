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
            video_codec: 'h264',
            audio_codec: 'opus',
            audio: 'true',
            video_bit_rate: '8000000'
        }
    };
})();

// const remoteVideo = document.getElementById('remoteVideo');

let videoDecoder = null;
let audioDecoder = null;
let audioContext = null;
let nextAudioTime = 0;

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

    let videoConfig;
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
        }
    });
    videoDecoder.configure(videoSupport.config);

    const audioConfig = { codec: 'opus', sampleRate: 48000, numberOfChannels: 2 };
    const audioSupport = await AudioDecoder.isConfigSupported(audioConfig);
    if (!audioSupport.supported) {
        console.warn('Browser cannot decode Opus with WebCodecs');
        return;
    }

    audioContext = new AudioContext({ sampleRate: 48000 });
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

function playAudioData(audioData) {
    if (!audioContext) return;

    const channels = audioData.numberOfChannels;
    const frameCount = audioData.numberOfFrames;
    const buffer = audioContext.createBuffer(channels, frameCount, audioData.sampleRate);

    for (let channel = 0; channel < channels; channel++) {
        const samples = new Float32Array(frameCount);
        audioData.copyTo(samples, { planeIndex: channel, format: 'f32-planar' });
        buffer.copyToChannel(samples, channel);
    }

    if (audioContext.state === 'suspended') audioContext.resume();

    const source = audioContext.createBufferSource();
    source.buffer = buffer;
    source.connect(audioContext.destination);

    const now = audioContext.currentTime;
    if (nextAudioTime < now + 0.01) nextAudioTime = now + 0.01;
    source.start(nextAudioTime);
    nextAudioTime += frameCount / audioData.sampleRate;
}

function handleMediaFrame(buffer) {
    if (buffer.byteLength < 10) return;

    const bytes = new Uint8Array(buffer);
    const kind = bytes[0];
    const keyFrame = bytes[1] === 1;
    const pts = Number(new DataView(buffer).getBigUint64(2, false));
    const payload = bytes.subarray(10);
    // console.log(`Received media frame: kind=${kind}, keyFrame=${keyFrame}, pts=${pts}, payloadSize=${payload.length}`);
    if (kind === 0) {
        if (!videoDecoder || videoDecoder.state !== 'configured') return;
        videoDecoder.decode(new EncodedVideoChunk({
            type: keyFrame ? 'key' : 'delta',
            timestamp: pts,
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
    console.log('Sending message to channel', channel, ':', message);
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
            video_codec: 'h264',
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