let upscalingEnabled = false;

self.onmessage = (event) => {
    const { type } = event.data;
    if (type === 'upscaling') {
        upscalingEnabled = Boolean(event.data.enabled);
        return;
    }

    if (type === 'init') {
        upscalingEnabled = Boolean(event.data.upscalingEnabled);
        renderStream(event.data.readable, event.data.canvas);
    }
};

async function renderStream(readable, canvas) {
    const ctx = canvas.getContext('2d');
    const reader = readable.getReader();

    while (true) {
        const { done, value: frame } = await reader.read();
        if (done) return;

        if (upscalingEnabled) {
            try {
                if (canvas.width !== frame.displayWidth || canvas.height !== frame.displayHeight) {
                    canvas.width = frame.displayWidth;
                    canvas.height = frame.displayHeight;
                }
                self.postMessage({ type: 'source-frame', frame }, [frame]);
            } catch (error) {
                frame.close();
                self.postMessage({ type: 'error', message: error.message || String(error) });
            }
            continue;
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
