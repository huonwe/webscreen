const jitterBufferTargetMs = 35; // 目标缓冲区延迟 (毫秒)

// Load CONFIG from sessionStorage if available, otherwise use URL params or defaults
var CONFIG = (function() {
    // Try to load from sessionStorage first (set by console.js)
    const stored = sessionStorage.getItem('webcpy_stream_config');
    if (stored) {
        try {
            const parsed = JSON.parse(stored);
            // Clear it after reading to avoid stale data
            sessionStorage.removeItem('webcpy_stream_config');
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
            driver_config: {
                video_codec: "h264",
                audio_codec: "opus",
                video_bit_rate: "20000000",
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
            video_bit_rate: "20000000"
        }
    };
})();

async function start() {
    console.log("Starting WebRTC connection...");
    const pc = new RTCPeerConnection();

    // 1. 监听远端流
    pc.ontrack = function (event) {
        if (event.track.kind === 'video') {
            const el = document.getElementById('remoteVideo');
            el.srcObject = event.streams[0];

            // 监听视频尺寸变化
            el.addEventListener('loadedmetadata', triggerCheck);
            el.addEventListener('resize', triggerCheck);
        } else if (event.track.kind === 'audio') {
            // 音频单独用一个 audio 标签播放，彻底解耦同步
            const audioEl = document.createElement('audio');
            audioEl.srcObject = event.streams[0];
            audioEl.autoplay = true;
            document.body.appendChild(audioEl);
        }
    };

    // 2. 添加一个仅接收的 Transceiver (重要)
    // 告诉浏览器："我想要接收视频，但我不需要发视频给你"
    pc.addTransceiver('video', { direction: 'recvonly' });
    pc.addTransceiver('audio', { direction: 'recvonly' });

    // 3. 创建 Offer
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);

    // 4. 等待 ICE
    await new Promise(resolve => {
        if (pc.iceGatheringState === 'complete') resolve();
        else pc.onicecandidate = e => { if (!e.candidate) resolve(); }
    });

    // 5. 建立 WebSocket 连接
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // Construct URL matching the hardcoded config to satisfy backend check
    const wsUrl = `${protocol}//${window.location.host}/screen/${CONFIG.device_type}/${CONFIG.device_id}/${CONFIG.device_ip}/${CONFIG.device_port}/ws`;

    window.ws = new WebSocket(wsUrl);
    window.ws.binaryType = "arraybuffer";

    window.ws.onopen = () => {
        console.log('WebSocket connected');
        // 发送 Config 和 SDP
        const config = {
            ...CONFIG,
            sdp: pc.localDescription.sdp
        };
        console.log(CONFIG)
        window.ws.send(JSON.stringify(config));
    };

    let isFirstMessage = true;
    window.ws.onmessage = async (event) => {
        if (isFirstMessage) {
            isFirstMessage = false;
            // 第一条消息是 SDP Answer + capabilities
            const message = JSON.parse(event.data);
            if (message.status !== 'ok') {
                console.error("Failed to start streaming:", message.message);
                return;
            }
            const answerSdp = message.sdp;
            const capabilities = message.capabilities;
            console.log("Received SDP Answer");
            console.log("Driver Capabilities:", capabilities);

            // Update UI based on capabilities
            await updateUIBasedOnCapabilities(capabilities);

            // 6. 设置 Answer
            await pc.setRemoteDescription(new RTCSessionDescription({
                type: 'answer',
                sdp: answerSdp
            }));

            pc.getReceivers().forEach(receiver => {
                if (receiver.track.kind === 'video') {
                    if (receiver.jitterBufferTarget !== undefined) {
                        receiver.jitterBufferTarget = jitterBufferTargetMs;
                    }
                    console.log('✓ 已启用 WebRTC 低延迟模式 (playoutDelayHint=', receiver.playoutDelayHint, ', jitterBufferTarget=', receiver.jitterBufferTarget, ')');
                }
            });
            setInterval(() => force_sync(pc), 1000);
            return;
        }

        if (event.data instanceof ArrayBuffer) {
            const view = new Uint8Array(event.data);
            if (view[0] === 17) { // WS_TYPE_CLIPBOARD_DATA
                const decoder = new TextDecoder();
                const text = decoder.decode(view.slice(1));
                // console.log("Clipboard from device:", text);
                // Copy to browser clipboard
                try {
                    navigator.clipboard.writeText(text).catch(err => {
                        console.error('Failed to write to clipboard:', err);
                    });
                } catch (e) {
                    console.error('Clipboard API not available:', e);
                    console.log("HTTPS is required for clipboard access.");
                }

            }
        } else {
            console.log('Received message from server:', event.data);
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

            let delay_ms = (currentDelay * 5000).toFixed(2);
            // if (delay_ms > 50) {
            //     console.log(`WebRTC 内部延迟: ${delay_ms} ms`);
            // }

            const videoEl = document.getElementById('remoteVideo');
            if (!videoEl) return;

            // --- 策略 A: 延迟较低时 (100ms - 500ms)，通过 1.25倍速 偷偷追帧 ---
            if (currentDelay > 0.1 && currentDelay < 0.5) {
                if (videoEl.playbackRate !== 1.1) {
                    console.log("轻微延迟，启用 1.1x 倍速追赶");
                    videoEl.playbackRate = 1.1;
                }
            }
            // 延迟恢复正常后，切回 1.0
            else if (currentDelay <= 0.1 && videoEl.playbackRate !== 1.0) {
                console.log("延迟恢复正常，切回 1.0x");
                videoEl.playbackRate = 1.0;
            }
        }
    });
};

function setClipboard(text) {
    if (!window.ws || window.ws.readyState !== WebSocket.OPEN) return;
    const encoder = new TextEncoder();
    const data = encoder.encode(text);
    const packet = new Uint8Array(1 + data.length);
    packet[0] = 15; // WS_TYPE_SET_CLIPBOARD
    packet.set(data, 1);
    window.ws.send(packet);
}

function getClipboard() {
    if (!window.ws || window.ws.readyState !== WebSocket.OPEN) return;
    const packet = new Uint8Array(1);
    packet[0] = 16; // WS_TYPE_GET_CLIPBOARD
    window.ws.send(packet);
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
        show('.feature-control');
        // Load control scripts
        try {
            await loadScript('/static/controlMessages.js');
            await loadScript('/static/buttons.js');
            await loadScript('/static/keyboard.js');
            await loadScript('/static/touch.js');
            await loadScript('/static/scroll.js');
            console.log("Control scripts loaded");
        } catch (e) {
            console.error("Failed to load control scripts", e);
        }
        // Handle Clipboard
        if (caps.can_clipboard) {
            show('.feature-clipboard');
        }
        // Handle UHID
        if (caps.can_uhid) {
            show('.feature-uhid');
            try {
                await loadScript('/static/uhid_mouse.js');
                await loadScript('/static/uhid_keyboard.js');
                await loadScript('/static/uhid_gamepad.js');
                console.log("UHID scripts loaded");
            } catch (e) {
                console.error("Failed to load UHID scripts", e);
            }
        }

    }

}
