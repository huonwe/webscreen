const jitterBufferTargetMs = 35; // 0 is a cake

// Load CONFIG from sessionStorage if available, otherwise use URL params or defaults
var CONFIG = (function () {
    // Try to load from sessionStorage first (set by console.js)
    const stored = sessionStorage.getItem('webscreen_device_configs');
    console.log("Stored config:", stored);
    if (stored) {
        try {
            const parsed = JSON.parse(stored);
            console.log("Using stored config:", parsed);
            // Clear it after reading to avoid stale data
            sessionStorage.removeItem('webscreen_device_configs');
            return parsed;
        } catch (e) {
            console.warn('Failed to parse stored config:', e);
        }
    }

    // Try to extract from URL path: /screen/:device_type/:device_id/:device_ip/:device_port
    const pathMatch = window.location.pathname.match(/^\/screen\/([^\/]+)\/([^\/]+)\/([^\/]+)\/([^\/]+)/);
    if (pathMatch) {
        return {
            device_type: decodeURIComponent(pathMatch[1]),
            device_id: decodeURIComponent(pathMatch[2]),
            device_ip: decodeURIComponent(pathMatch[3]),
            device_port: decodeURIComponent(pathMatch[4]),
            av_sync: false,
            driver_config: {
                max_fps: "60",
                video_codec: "h264",
                audio_codec: "opus",
                video_bit_rate: "8000000",

                // new_display: "1920x1080/60",
            }
        };
    }

    // Fallback to hardcoded defaults for testing
    return {
        device_type: "android",
        device_id: "emulator-5554",
        device_ip: "0",
        device_port: "0",
        driver_config: {
            video_codec: "h264",
            audio_codec: "opus",
            video_bit_rate: "8000000"
        }
    };
})();

async function start() {
    console.log("Starting WebRTC connection...");
    const pc = new RTCPeerConnection();

    // 1. Listen for remote tracks
    pc.ontrack = function (event) {
        if (event.track.kind === 'video') {
            const el = document.getElementById('remoteVideo');
            el.srcObject = event.streams[0];

            el.addEventListener('loadedmetadata', triggerCheck);
            el.addEventListener('resize', triggerCheck);
        } else if (event.track.kind === 'audio') {
            const audioEl = document.createElement('audio');
            audioEl.srcObject = event.streams[0];
            audioEl.autoplay = true;
            document.body.appendChild(audioEl);
        }
    };

    // 2. Add a recvonly Transceiver
    pc.addTransceiver('video', { direction: 'recvonly' });
    pc.addTransceiver('audio', { direction: 'recvonly' });

    // 3. Create Offer
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);

    // 4. Wait ICE
    await new Promise(resolve => {
        if (pc.iceGatheringState === 'complete') resolve();
        else pc.onicecandidate = e => { if (!e.candidate) resolve(); }
    });

    // 5. Establish WebSocket connection
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // Construct URL matching the hardcoded config to satisfy backend check
    const wsUrl = `${protocol}//${window.location.host}/screen/ws`;

    window.ws = new WebSocket(wsUrl);
    window.ws.binaryType = "arraybuffer";

    window.ws.onopen = () => {
        console.log('WebSocket connected');
        // 发送 Config 和 SDP
        console.log("config:", CONFIG);
        const config = {
            ...CONFIG,
            sdp: pc.localDescription.sdp
        };
        console.log(CONFIG.driver_config)
        window.ws.send(JSON.stringify(config));
    };
    window.ws.onmessage = async (event) => {
        if (typeof event.data === 'string') {
            const message = JSON.parse(event.data);
            console.log("Received message status:", message.status);
            switch (message.status) {
                case 'ok':
                    switch (message.stage) {
                        case 'webrtc_init':
                            const answerSdp = message.sdp;
                            const capabilities = message.capabilities;
                            console.log("Received SDP Answer");
                            console.log("Driver Capabilities:", capabilities);

                            // Update UI based on capabilities
                            await updateUIBasedOnCapabilities(capabilities);

                            // 设置 Answer
                            await pc.setRemoteDescription(new RTCSessionDescription({
                                type: 'answer',
                                sdp: answerSdp
                            }));

                            pc.getReceivers().forEach(receiver => {
                                if (receiver.track.kind === 'video') {
                                    if (receiver.jitterBufferTarget !== undefined) {
                                        receiver.jitterBufferTarget = jitterBufferTargetMs;
                                    }
                                    console.log('✓ (playoutDelayHint=', receiver.playoutDelayHint, ', jitterBufferTarget=', receiver.jitterBufferTarget, ')');
                                }
                            });
                            setInterval(() => force_sync(pc), 1000);
                            break;
                        default:
                            break;
                    }
                    break;
                case 'error':
                    console.error("Error from server:", message);
                    showToast(i18n.t('error_from_server', { msg: message.message }), 2000);
                    break;
                default:
                    console.warn("Unknown message status:", message.status);
            }
        } else {
            const view = new Uint8Array(event.data);
            switch (view[0]) {
                case 0x17: // TYPE_CLIPBOARD_DATA
                    const decoder = new TextDecoder();
                    const text = decoder.decode(view.slice(1));
                    console.log("Clipboard from device:", text);
                    // Copy to browser clipboard
                    try {
                        navigator.clipboard.writeText(text).catch(err => {
                            console.error('Failed to write to clipboard:', err);
                        });
                    } catch (e) {
                        console.error('Clipboard API not available:', e);
                        console.log("HTTPS is required for clipboard access.");
                    }
                    break;
                default:
                    console.warn("Unknown binary message type:", view[0]);
            }
        }
    };
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

            if (currentDelay > 0.1 && currentDelay < 0.5) {
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
        script.onload = resolve;
        script.onerror = reject;
        document.head.appendChild(script);
    });
}

async function updateUIBasedOnCapabilities(caps) {
    if (!caps) return;

    // Helper to show elements
    const show = (selector) => {
        document.querySelectorAll(selector).forEach(el => el.style.display = ''); // Remove inline display:none to revert to CSS
    };

    // Handle Control
    if (caps.can_control) {
        // Load control scripts
        try {
            if (caps.is_linux) {
                await loadScript('/static/capabilities/linux_mouse.js');
                await loadScript('/static/capabilities/keyboard.js');
                // show('.feature-linux-mouse');
            }
            if (caps.is_android) {
                await loadScript('/static/capabilities/controlMessages.js');
                await loadScript('/static/capabilities/keyboard.js');
                await loadScript('/static/capabilities/touch.js');
                await loadScript('/static/capabilities/scroll.js');
                await loadScript('/static/capabilities/buttons.js');
                show('.feature-android-buttons');
                show('.feature-control');
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
                show('.feature-clipboard');
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
                show('.feature-uhid');
                console.log("UHID scripts loaded");
            } catch (e) {
                console.error("Failed to load UHID scripts", e);
            }
        }

    }

}
