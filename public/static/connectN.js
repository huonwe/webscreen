const jitterBufferTargetMs = 0; // 0 is a cake

// Load CONFIG from sessionStorage if available, otherwise use URL params or defaults
var CONFIG = (function () {

    // // if url has ?debug, use hardcoded debug config
    const urlParams = new URLSearchParams(window.location.search);
    if (urlParams.has('debug')) {
        console.log("Using debug config from URL parameter");
        return {
            type: "sunshine",
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
        type: "none",
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

async function start() {
    console.log("CONFIG:", CONFIG);
    if (!CONFIG.remote_uuid) {
        console.error("No device configuration found. Please set up the device configuration in the console.");
        showToast(i18n.t('error_no_device_config'), 3000);
        return;
    }
    console.log("Starting WebRTC connection...");
    const pc = new RTCPeerConnection();

    // 1. Listen for remote tracks
    pc.ontrack = function (event) {
        if (event.track.kind === 'video') {
            console.log("Received remote video track:", event.track);
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

                console.log('✓ (playoutDelayHint=', receiver.playoutDelayHint, ', jitterBufferTarget=', receiver.jitterBufferTarget, ')');
            }
        });
    };

    // 2. Add a recvonly Transceiver
    pc.addTransceiver('video', { direction: 'recvonly' });
    pc.addTransceiver('audio', { direction: 'recvonly' });

    // Add data channel for control messages if needed
    const dataChannelOrdered = pc.createDataChannel("control-ordered", {
        ordered: true,
    });
    const dataChannelUnordered = pc.createDataChannel("control-unordered", {
        ordered: false,
        // maxRetransmits: 0,
    });
    window.dataChannelOrdered = dataChannelOrdered;
    window.dataChannelUnordered = dataChannelUnordered;
    dataChannelUnordered.addEventListener('message', (event) => {
        console.log("DataChannel Unordered Message:", event.data);
        const view = new Uint8Array(event.data);
        const decoder = new TextDecoder();
        // console.log("Received binary message, type:", view[0]);
        switch (view[0]) {
            case 0x17: // TYPE_CLIPBOARD_DATA
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
            case 0x64: // TYPE_TEXT_MSG
                const textMsg = decoder.decode(view.slice(1));
                console.log("Text message from agent:", textMsg);
                showToast(textMsg, 3000);
                break;
            default:
                console.warn("Unknown binary message type:", view[0]);
        }
    }
    )



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
    const wsUrl = `${protocol}//${window.location.host}/screen/wsn`;

    window.ws = new WebSocket(wsUrl);
    window.ws.binaryType = "arraybuffer";

    window.ws.onopen = () => {
        console.log('WebSocket connected');
        const config = {
            protocol: "webrtc",
            video_codec: CONFIG.driver_config?.video_codec || "h264",
            audio_codec: CONFIG.driver_config?.audio_codec || "opus",
            remote_uuid: CONFIG.remote_uuid,
            extra: {
                sdp: pc.localDescription.sdp,
                av_sync: String(CONFIG.av_sync === true)
            }
        };
        console.log("connection config:", config);
        window.ws.send(JSON.stringify(config));
    };
    let remoteConfigSent = false;
    window.ws.onmessage = async (event) => {
        const message = JSON.parse(event.data);
        console.log("Received message:", message);
        if (message.status === 'error') {
            console.error("Error from server:", message);
            showToast(i18n.t('error_from_server', { msg: message.message }), 2000);
            return;
        }
        
        if (message.stage === 'metainfo_remote' && message.capabilities) {
            console.log("Received capabilities from Nexus:", message.capabilities);
            await updateUIBasedOnCapabilities(message.capabilities);
            return;
        }
        if (message.stage === 'metainfo_remote' && message.media_meta) {
            console.log("Received media meta from Nexus:", message.media_meta);
            return;
        }
        if (message.status !== 'ok' || message.stage !== 'connection_view_session' || !message.sdp) {

            return;
        }

        const answerSdp = message.sdp;
        console.log("Received SDP answer from Nexus:", answerSdp);
        if (!answerSdp.length) {
            showToast(i18n.t('error_empty_sdp_answer'), 2000);
            return;
        }
        const formattedSdp = answerSdp
            .split(/\r\n|\r|\n/)
            .map(line => line.trim())
            .filter(line => line.length > 0)
            .join('\r\n') + '\r\n';
        await pc.setRemoteDescription({ type: 'answer', sdp: formattedSdp });
        console.log("WebRTC connection established with Nexus");
        // await loadScript('/static/capabilities/keyboard.js');
        // await loadScript('/static/capabilities/touch.js');
        // await loadScript('/static/capabilities/scroll.js');
        // await loadScript('/static/capabilities/buttons.js');
        if (!remoteConfigSent) {
            remoteConfigSent = true;
            const remoteConfig = { ...(CONFIG.driver_config || {}) };
            if (CONFIG.type === 'android') {
                remoteConfig.deviceID = CONFIG.device_ip || CONFIG.device_id || '';
            }
            window.ws.send(JSON.stringify(remoteConfig));
        }
    };
}

function sendDataChannelMessage(channel, msg) {
    // console.log("sendDataChannelMessage:", channel, msg);
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
