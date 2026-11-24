const jitterBufferTargetMs = 0; // 0 is a cake

// Load CONFIG from sessionStorage if available, otherwise use URL params or defaults
var CONFIG = (function () {

    // // if url has ?debug, use hardcoded debug config
    const urlParams = new URLSearchParams(window.location.search);
    if (urlParams.has('debug')) {
        console.log("Using debug config from URL parameter");
        return {
            device_type: "sunshine",
            device_id: "127.0.0.1:5555",
            device_ip: "127.0.0.1",
            device_port: "5555",
            av_sync: false,
            driver_config: {
                max_fps: "60",
                video_codec: "h265",
                audio_codec: "opus",
                video_bit_rate: "4000000",
                audio: "true",
                file_path: "test/test.h265",
                deviceID: "emulator-5554",
                // new_display: "1920x1080/60",
            }
        };
    }

    // Try to load from sessionStorage first (set by console.js)
    const stored = sessionStorage.getItem('webscreen_device_configs_now');
    console.log("Stored config:", stored);
    if (stored) {
        try {
            const parsed = JSON.parse(stored);
            console.log("Using stored config:", parsed);
            // Clear it after reading to avoid stale data
            // sessionStorage.removeItem('webscreen_device_configs_now');
            return parsed;
        } catch (e) {
            console.warn('Failed to parse stored config:', e);
        }
    }

    // Fallback to hardcoded defaults for testing
    return {
        device_type: "none",
        device_id: "emulator-5554",
        device_ip: "0",
        device_port: "0",
        av_sync: false,
        driver_config: {
            video_codec: "h264",
            audio: "true",
            audio_codec: "opus",
            video_bit_rate: "8000000"
        }
    };
})();

if (window.location.pathname.startsWith('/screen-ws')) {
    CONFIG.protocol = 'websocket';
    CONFIG.remote_uuid = CONFIG.remote_uuid || '68bf97a0-011c-40b5-8710-cb49a5bad369';
    CONFIG.type = CONFIG.type || 'android';
}

let activeTransportProtocol = 'webrtc';
let webRtcFrameWorker = null;
let webSRSourceEnabled = false;

window.isWebSRSourceEnabled = () => webSRSourceEnabled;
window.setWebSRSourceEnabled = (enabled) => {
    webSRSourceEnabled = Boolean(enabled);
    if (webRtcFrameWorker) {
        webRtcFrameWorker.postMessage({ type: 'upscaling', enabled: webSRSourceEnabled });
    }
    if (window.WebSocketMedia) {
        window.WebSocketMedia.setUpscaling(webSRSourceEnabled);
    }
};
window.handleWebSRSourceFrame = (frame) => {
    if (webSRSourceEnabled && typeof window.notifySourceFrameUpdated === 'function') {
        window.notifySourceFrameUpdated(frame);
    } else {
        frame.close();
    }
};

async function start() {
    console.log("CONFIG:", CONFIG);
    if (!CONFIG.remote_uuid) {
        console.error("No device configuration found. Please set up the device configuration in the console.");
        showToast(i18n.t('error_no_device_config'), 3000);
        return;
    }

    activeTransportProtocol = CONFIG.protocol || 'websocket';
    const usesWebSocket = activeTransportProtocol === 'websocket';
    if (usesWebSocket) window.WebSocketMedia.enableAudioStart();
    if (!usesWebSocket && activeTransportProtocol !== 'webrtc') {
        const error = new Error(`Unsupported transport protocol: ${activeTransportProtocol}`);
        console.error(error);
        showToast(error.message, 3000);
        return;
    }

    console.log(`Starting ${activeTransportProtocol} connection...`);
    let pc = null;
    window.dataChannelOrdered = null;
    window.dataChannelUnordered = null;

    if (!usesWebSocket) {
        pc = new RTCPeerConnection();
        pc.ontrack = function (event) {
            if (event.track.kind === 'video') {
                console.log("Received remote video track:", event.track);
                const canvas = document.getElementById('remoteVideo');
                canvas.addEventListener('loadedmetadata', triggerCheck);
                canvas.addEventListener('resize', triggerCheck);
                const offscreen = canvas.transferControlToOffscreen();
                const processor = new MediaStreamTrackProcessor({ track: event.track });
                const readable = processor.readable;

                const worker = new Worker('/static/canvasWorkerWebRTC.js');
                webRtcFrameWorker = worker;
                worker.onmessage = (messageEvent) => {
                    if (messageEvent.data.type === 'source-frame') {
                        window.handleWebSRSourceFrame(messageEvent.data.frame);
                    }
                };
                worker.postMessage(
                    {
                        type: 'init',
                        readable,
                        canvas: offscreen,
                        upscalingEnabled: webSRSourceEnabled
                    },
                    [readable, offscreen]
                );
            } else if (event.track.kind === 'audio') {
                const audioEl = document.createElement('audio');
                audioEl.srcObject = event.streams[0];
                audioEl.autoplay = true;
                document.body.appendChild(audioEl);
            }
            pc.getReceivers().forEach(receiver => {
                if (receiver.track.kind === 'video' || receiver.track.kind === 'audio') {
                    if ('contentHint' in receiver.track) {
                        receiver.contentHint = 'motion';
                    }
                    if ('playoutDelayHint' in receiver) {
                        receiver.playoutDelayHint = 0;
                    }
                    if ('jitterBufferTarget' in receiver) {
                        receiver.jitterBufferTarget = 0;
                    }
                }
            });
        };

        pc.addTransceiver('video', { direction: 'recvonly' });
        pc.addTransceiver('audio', { direction: 'recvonly' });

        const dataChannelOrdered = pc.createDataChannel("control-ordered", { ordered: true });
        const dataChannelUnordered = pc.createDataChannel("control-unordered", { ordered: false });
        window.dataChannelOrdered = dataChannelOrdered;
        window.dataChannelUnordered = dataChannelUnordered;
        dataChannelUnordered.addEventListener('message', (event) => {
            console.log("DataChannel Unordered Message:", event.data);
            const view = new Uint8Array(event.data);
            const decoder = new TextDecoder();
            switch (view[0]) {
                case 0x17: {
                    const text = decoder.decode(view.slice(1));
                    console.log("Clipboard from device:", text);
                    try {
                        navigator.clipboard.writeText(text).catch(err => {
                            console.error('Failed to write to clipboard:', err);
                        });
                    } catch (error) {
                        console.error('Clipboard API not available:', error);
                    }
                    break;
                }
                case 0x64: {
                    const textMsg = decoder.decode(view.slice(1));
                    console.log("Text message from agent:", textMsg);
                    showToast(textMsg, 3000);
                    break;
                }
                default:
                    console.warn("Unknown binary message type:", view[0]);
            }
        });

        const offer = await pc.createOffer();
        await pc.setLocalDescription(offer);
        await new Promise(resolve => {
            if (pc.iceGatheringState === 'complete') resolve();
            else pc.onicecandidate = event => {
                if (!event.candidate) resolve();
            };
        });
    }

    // This socket carries signaling; in WebSocket mode, it also carries media frames.
    const wsPath = window.location.pathname.startsWith('/screen-ws')
        ? '/screen-ws/wsn'
        : '/screen/ws';
    const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}${wsPath}`;
    window.ws = new WebSocket(wsUrl);
    window.ws.binaryType = 'arraybuffer';

    let websocketMediaReady = false;
    let pendingWebSocketFrames = [];

    window.ws.onopen = () => {
        console.log('WebSocket connected');
        const extra = { av_sync: String(CONFIG.av_sync === true) };
        if (pc) extra.sdp = pc.localDescription.sdp;
        const config = {
            protocol: activeTransportProtocol,
            video_codec: CONFIG.driver_config?.video_codec || 'h264',
            audio_codec: CONFIG.driver_config?.audio_codec || 'opus',
            remote_uuid: CONFIG.remote_uuid,
            extra
        };
        console.log("connection config:", config);
        window.ws.send(JSON.stringify(config));

        const remoteConfig = { ...(CONFIG.driver_config || {}) };
        if (CONFIG.type === 'android') {
            remoteConfig.deviceID = CONFIG.device_ip || CONFIG.device_id || '';
        }
        window.ws.send(JSON.stringify(remoteConfig));
    };

    window.ws.onmessage = async (event) => {
        if (event.data instanceof ArrayBuffer) {
            if (usesWebSocket) {
                if (websocketMediaReady) {
                    window.WebSocketMedia.handleFrame(event.data);
                } else {
                    pendingWebSocketFrames.push(event.data);
                    if (pendingWebSocketFrames.length > 120) {
                        let latestKeyFrame = -1;
                        for (let index = pendingWebSocketFrames.length - 1; index >= 0; index--) {
                            const frame = new Uint8Array(pendingWebSocketFrames[index]);
                            if (frame[0] === 0 && frame[1] === 1) {
                                latestKeyFrame = index;
                                break;
                            }
                        }
                        pendingWebSocketFrames = latestKeyFrame >= 0
                            ? pendingWebSocketFrames.slice(latestKeyFrame, latestKeyFrame + 8)
                            : pendingWebSocketFrames.slice(-8);
                    }
                }
            } else {
                console.warn('Unexpected binary WebSocket message while using WebRTC');
            }
            return;
        }

        let message;
        try {
            message = JSON.parse(event.data);
        } catch (error) {
            console.warn('Ignoring unknown WebSocket message:', error);
            return;
        }
        console.log("Received message:", message);
        if (message.status === 'error') {
            console.error("Error from server:", message);
            showToast(i18n.t('error_from_server', { msg: message.message }), 2000);
            return;
        }

        if (message.stage === 'metainfo_remote') {
            if (usesWebSocket && message.media_meta) {
                try {
                    const audioAvailable = await window.WebSocketMedia.initialize(
                        message.media_meta,
                        CONFIG.driver_config?.video_codec || 'h264'
                    );
                    websocketMediaReady = true;
                    window.WebSocketMedia.status(
                        audioAvailable
                            ? 'WebSocket media initialized'
                            : 'Video initialized; Opus audio is not supported'
                    );
                    let latestKeyFrame = -1;
                    for (let index = pendingWebSocketFrames.length - 1; index >= 0; index--) {
                        const frame = new Uint8Array(pendingWebSocketFrames[index]);
                        if (frame[0] === 0 && frame[1] === 1) {
                            latestKeyFrame = index;
                            break;
                        }
                    }
                    if (latestKeyFrame >= 0) {
                        pendingWebSocketFrames = pendingWebSocketFrames.slice(latestKeyFrame, latestKeyFrame + 8);
                    } else {
                        pendingWebSocketFrames = pendingWebSocketFrames.slice(-8);
                    }
                    for (const frame of pendingWebSocketFrames) {
                        window.WebSocketMedia.handleFrame(frame);
                    }
                    pendingWebSocketFrames = [];
                } catch (error) {
                    console.error('Failed to initialize WebSocket media', error);
                    window.WebSocketMedia.status(error.message);
                    showToast(error.message, 3000);
                    return;
                }
            }
            if (message.capabilities) {
                console.log("Received capabilities from Nexus:", message.capabilities);
                await updateUIBasedOnCapabilities(message.capabilities);
            }
            return;
        }

        if (message.status !== 'ok' || message.stage !== 'connection_view_session') {
            return;
        }

        if (pc) {
            if (!message.sdp) {
                showToast(i18n.t('error_empty_sdp_answer'), 2000);
                return;
            }
            const formattedSdp = message.sdp
                .split(/\r\n|\r|\n/)
                .map(line => line.trim())
                .filter(line => line.length > 0)
                .join('\r\n') + '\r\n';
            await pc.setRemoteDescription({ type: 'answer', sdp: formattedSdp });
            console.log("WebRTC connection established with Nexus");
        } else if (!message.useWSTransport) {
            console.error('Server did not enable WebSocket media transport');
            showToast('WebSocket media transport was not enabled', 3000);
            return;
        } else {
            console.log("Using WebSocket for audio/video transport");
        }

    };
    window.ws.onclose = () => {
        if (usesWebSocket) {
            window.WebSocketMedia.stop();
            websocketMediaReady = false;
        }
    };
}

function sendDataChannelMessage(channel, msg) {
    // console.log("sendDataChannelMessage:", channel, msg);
    if (activeTransportProtocol === 'websocket') {
        if (window.ws && window.ws.readyState === WebSocket.OPEN) {
            window.ws.send(msg);
        } else {
            console.warn('Cannot send control message: WebSocket is not open');
        }
        return;
    }
    if (channel && channel.readyState === 'open') {
        // console.log("Sending message via DataChannel:", channel.label);
        channel.send(msg);
    }
}

let lastJitterDelay = 0;
let lastEmittedCount = 0;

async function force_sync(pc) {
    if (!pc) {
        console.warn("RTCPeerConnection is not defined. Cannot force sync.");
        return;
    };

    // 获取 WebRTC 统计信息
    const stats = await pc.getStats();

    stats.forEach(report => {
        // 找到视频接收通道的统计
        if (report.type === 'inbound-rtp' && report.kind === 'video') {

            // jitterBufferDelay 是累积的总延迟时间（秒）
            // jitterBufferEmittedCount 是累积的总帧数
            // 我们需要计算“当前”的平均延迟，所以要减去上一次的值取差值

            const deltaDelay = report.jitterBufferDelay - lastJitterDelay;
            const deltaCount = report.jitterBufferEmittedCount - lastEmittedCount;

            // 更新历史值
            lastJitterDelay = report.jitterBufferDelay;
            lastEmittedCount = report.jitterBufferEmittedCount;

            // 计算最近一秒内的平均帧延迟
            let currentDelay = 0;
            if (deltaCount > 0) {
                currentDelay = deltaDelay / deltaCount;
            }

            // let delay_ms = (currentDelay * 5000).toFixed(2);
            // if (delay_ms > 50) {
            //     console.log(`WebRTC Internal Delay: ${delay_ms} ms`);
            // }

            const videoEl = document.getElementById('remoteVideo');
            if (!videoEl) return;

            if (currentDelay > 0.1) {
                if (videoEl.playbackRate !== 1.1) {
                    // console.log("轻微延迟，启用 1.1x 倍速追赶");
                    videoEl.playbackRate = 1.1;
                }
            }
            // 延迟恢复正常后，切回 1.0
            else if (currentDelay <= 0.1 && videoEl.playbackRate !== 1.0) {
                // console.log("延迟恢复正常，切回 1.0x");
                videoEl.playbackRate = 1.0;
            }
        }
    });
};


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
            if (caps.is_windows) {
                await loadScript('/static/capabilities/linux_mouse.js');
                await loadScript('/static/capabilities/keyboard.js');
                await loadScript('/static/capabilities/gamestream.js');
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

async function renderStreamToCanvas(videoTrack, canvas) {
    const ctx = canvas.getContext('2d');

    // 1. 创建处理器，将轨道转换为可读流
    const processor = new MediaStreamTrackProcessor({ track: videoTrack });
    const reader = processor.readable.getReader();

    // 2. 循环读取每一帧 VideoFrame
    while (true) {
        const { done, value: frame } = await reader.read();
        if (done) break;

        // 动态同步画布宽高
        if (canvas.width !== frame.displayWidth || canvas.height !== frame.displayHeight) {
            canvas.width = frame.displayWidth;
            canvas.height = frame.displayHeight;
        }

        // 3. 直接将 VideoFrame 绘制到 Canvas
        ctx.drawImage(frame, 0, 0);

        // ⚠️ 极其重要：必须手动释放帧内存，否则会瞬间内存泄漏导致页面崩溃
        frame.close();
    }
}