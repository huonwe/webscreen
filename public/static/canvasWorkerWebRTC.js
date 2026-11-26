let upscalingEnabled = false;
let isUpscalingBusy = false; // 防止主线程超分来不及处理时，VideoFrame 堆积撑爆解码器
let srPort = null;

self.onmessage = (event) => {
    const { type } = event.data;
    if (type === 'upscaling') {
        upscalingEnabled = Boolean(event.data.enabled);
        isUpscalingBusy = false;
        return;
    }

    // 主线程渲染完超分帧后回传通知，解除背压锁定
    if (type === 'frame-done') {
        isUpscalingBusy = false;
        return;
    }

    if (type === 'init') {
        if (event.data.srPort) {
            if (srPort) srPort.close();
            srPort = event.data.srPort;
        }
        upscalingEnabled = Boolean(event.data.upscalingEnabled);
        renderStream(event.data.readable, event.data.canvas);
    }

    if (type === 'set-sr-port') {
        if (srPort) srPort.close();
        srPort = event.data.port;
        return;
    }
};

async function renderStream(readable, canvas) {
    // 开启 desynchronized 绕过浏览器垂直同步等待，降低一帧延迟
    const ctx = canvas.getContext('2d', {
        alpha: false,
        desynchronized: true
    });
    const reader = readable.getReader();

    while (true) {
        const { done, value: frame } = await reader.read();
        if (done) return;

        if (upscalingEnabled) {
            if (srPort) {
                try {
                    srPort.postMessage({ type: 'frame', frame }, [frame]);
                } catch (error) {
                    frame.close();
                    self.postMessage({ type: 'error', message: error.message || String(error) });
                }
            } else {
                // 端口尚未就绪时直接释放，防止阻塞解码器
                frame.close();
            }
            continue; // 在 WS Worker 的 output 回调中则是 return;
        }

        try {
            if (canvas.width !== frame.displayWidth || canvas.height !== frame.displayHeight) {
                canvas.width = frame.displayWidth;
                canvas.height = frame.displayHeight;
            }
            ctx.drawImage(frame, 0, 0);
        } finally {
            frame.close();
        }
    }
}