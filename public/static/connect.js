const jitterBufferTargetMs = 35; // 目标缓冲区延迟 (毫秒)
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

    // 4. 发送 Offer 给 Go 后端 (Wait for ICE gathering inside browser logic for simplicity if not using vanilla ice gathering trick)
    // 注意：上面的 Go 代码用了 GatheringCompletePromise，所以我们直接发 LocalDescription 即可
    // 但浏览器侧通常也需要等 ICE，或者为了简单，我们直接把当前的 SDP 发过去（可能不含 candidate，但局域网通常没事）

    // 为了最稳妥，我们等一下 ICE
    await new Promise(resolve => {
        if (pc.iceGatheringState === 'complete') resolve();
        else pc.onicecandidate = e => { if (!e.candidate) resolve(); }
    });

    // 5. POST 请求交换 SDP
    const response = await fetch('/sdp', {
        method: 'POST',
        body: pc.localDescription.sdp
    });

    const answerSdp = await response.text();

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

    window.ws = new WebSocket('ws://' + window.location.host + '/ws');
        window.ws.onopen = () => console.log('WebSocket connected');
        window.ws.onmessage = (event) => {
            console.log('Received message from server:', event.data);
    };
    
    // let p = createRequestKeyFramePacket();
    // sendWSMessage(p);

    // 重置按钮状态
    // p = createTouchPacket(TOUCH_ACTION_UP, 0, 0, 0);
    // sendWSMessage(p);
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

            let delay_ms = (currentDelay * 1000).toFixed(2);
            if (delay_ms > 50) {
            console.log(`WebRTC 内部延迟: ${delay_ms} ms`);
            }

            const videoEl = document.getElementById('remoteVideo');
            if (!videoEl) return;

            // --- 策略 A: 延迟较低时 (100ms - 500ms)，通过 1.25倍速 偷偷追帧 ---
            if (currentDelay > 0.1 && currentDelay < 0.5) {
                if (videoEl.playbackRate !== 1.25) {
                    console.log("轻微延迟，启用 1.25x 倍速追赶");
                    videoEl.playbackRate = 1.25;
                }
            }
            // 延迟恢复正常后，切回 1.0
            else if (currentDelay <= 0.1 && videoEl.playbackRate !== 1.0) {
                console.log("延迟恢复正常，切回 1.0x");
                videoEl.playbackRate = 1.0;
            }

            // --- 策略 B: 延迟爆炸时 (> 1000ms)，暴力重置 ---
            // 既然 buffered 读不到，我们不能用 currentTime 跳转
            // 最有效的“清空 Buffer”方法是：暂停一瞬间再播放
            // if (currentDelay > 1.0) {
            //     console.warn("延迟严重，执行重置...");
            //     videoEl.pause();
            //     // 必须通过 setTimeout 给浏览器喘息时间来丢弃 Buffer
            //     setTimeout(() => {
            //         videoEl.play().catch(e => console.error(e));
            //     }, 0);

            //     // 重置后强制 playoutDelayHint 再次生效
            //     const receivers = pc.getReceivers();
            //     const videoReceiver = receivers.find(r => r.track && r.track.kind === 'video');
            //     if (videoReceiver) {
            //         if (videoReceiver.jitterBufferTarget !== undefined) {
            //             videoReceiver.jitterBufferTarget = jitterBufferTargetMs;
            //         }
            //     }
            // }
        }
    });
};


// function sendWSMessage(message) {
//     if (window.ws && window.ws.readyState === WebSocket.OPEN) {
//         window.ws.send(message);
//     } else {
//         console.warn("WebSocket is not open. Cannot send message.");
//     }
// }
