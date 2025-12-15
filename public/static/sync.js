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

            console.log(`WebRTC 内部延迟: ${(currentDelay * 1000).toFixed(2)} ms`);

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
            if (currentDelay > 1.0) {
                console.warn("延迟严重，执行重置...");
                videoEl.pause();
                // 必须通过 setTimeout 给浏览器喘息时间来丢弃 Buffer
                setTimeout(() => {
                    videoEl.play().catch(e => console.error(e));
                }, 0);

                // 重置后强制 playoutDelayHint 再次生效
                const receivers = pc.getReceivers();
                const videoReceiver = receivers.find(r => r.track && r.track.kind === 'video');
                if (videoReceiver) {
                    if (videoReceiver.playoutDelayHint !== undefined) {
                        videoReceiver.playoutDelayHint = 0;
                    }
                    if (videoReceiver.jitterBufferTarget !== undefined) {
                        videoReceiver.jitterBufferTarget = 0;
                    }
                }
            }
        }
    });
};