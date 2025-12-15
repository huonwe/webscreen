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
            if (receiver.playoutDelayHint !== undefined) {
                receiver.playoutDelayHint = 0;
            }
            if (receiver.jitterBufferTarget !== undefined) {
                receiver.jitterBufferTarget = 0;
            }
            console.log('✓ 已启用 WebRTC 低延迟模式 (playoutDelayHint=', receiver.playoutDelayHint, ', jitterBufferTarget=', receiver.jitterBufferTarget, ')');
        }
    });
    setInterval(() => force_sync(pc), 1000);
    
    // let p = createRequestKeyFramePacket();
    // sendWSMessage(p);

    // 重置按钮状态
    // p = createTouchPacket(TOUCH_ACTION_UP, 0, 0, 0);
    // sendWSMessage(p);
}

function sendWSMessage(message) {
    if (window.ws && window.ws.readyState === WebSocket.OPEN) {
        window.ws.send(message);
    } else {
        console.warn("WebSocket is not open. Cannot send message.");
    }
}
